package grit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

// AgentAction represents a CRUD task extracted by the AI
type AgentAction struct {
	Action string                 `json:"action"` // create, read_all, read_by_id, update, delete
	Model  string                 `json:"model"`
	ID     interface{}            `json:"id,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// AICrudHandler returns a handler that parses natural language and executes CRUD
func AICrudHandler(llmModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respond(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
			return
		}

		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respond(w, http.StatusBadRequest, false, "Invalid JSON", nil)
			return
		}

		if req.Prompt == "" {
			respond(w, http.StatusBadRequest, false, "prompt is required", nil)
			return
		}

		// 1. Build Model Context
		context := buildModelContext()

		// 2. Prepare System Prompt
		systemPrompt := fmt.Sprintf(`You are a database agent for the Grit framework. 
Available Models and their fields:
%s

Your task is to convert the user's natural language request into a JSON action.
For 'read_all', you can include key-value pairs in 'data' to filter the results (e.g., { "author": "sayana" }).

Return ONLY a valid JSON object with the following structure:
{
  "action": "create" | "read_all" | "read_by_id" | "update" | "delete",
  "model": "model_name",
  "id": "optional_id",
  "data": { "field": "value" }
}

User Request: "%s"
JSON:`, context, req.Prompt)

		// 3. Get LLM Response
		response, err := Prompt(llmModel, systemPrompt)
		if err != nil {
			respond(w, http.StatusBadGateway, false, fmt.Sprintf("LLM error: %v", err), nil)
			return
		}

		// 4. Parse Action
		var action AgentAction
		// Clean the response in case LLM adds markdown or fluff
		cleanJSON := strings.TrimSpace(response)
		if strings.HasPrefix(cleanJSON, "```json") {
			cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
			cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		}
		cleanJSON = strings.TrimSpace(cleanJSON)

		if err := json.Unmarshal([]byte(cleanJSON), &action); err != nil {
			respond(w, http.StatusBadRequest, false, "Failed to parse AI action", map[string]interface{}{
				"ai_raw": response,
			})
			return
		}

		// Deterministic intent fallback from user prompt (prevents LLM misclassification,
		// e.g., edit/delete accidentally returned as create).
		overrideAction, ok := inferActionFromPrompt(req.Prompt)
		if ok {
			action.Action = overrideAction
		}

		if action.Model == "" {
			action.Model = inferModelFromPrompt(req.Prompt)
		}

		action.Action = normalizeActionName(action.Action)
		action.Model = normalizeModelName(action.Model)

		// Fallback: if user asked for filtered read but model didn't emit data,
		// infer simple field filters from the original prompt.
		if action.Action == "read_all" && len(action.Data) == 0 {
			action.Data = inferReadAllFilters(req.Prompt, action.Model)
		}

		actorEmail, _ := r.Context().Value(FirebaseEmailKey).(string)
		actorUID, _ := r.Context().Value(FirebaseUIDKey).(string)

		// 5. Execute Action
		result, err := executeAgentAction(action, actorEmail, actorUID, req.Prompt)
		if err != nil {
			respond(w, http.StatusBadRequest, false, err.Error(), map[string]interface{}{"action": action})
			return
		}

		respond(w, http.StatusOK, true, "Action executed successfully", map[string]interface{}{
			"action": action,
			"result": result,
		})
	}
}

func buildModelContext() string {
	var sb strings.Builder
	for name, m := range models {
		sb.WriteString(fmt.Sprintf("- %s: ", name))
		t := reflect.TypeOf(m).Elem()
		var fields []string
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			jsonTag := f.Tag.Get("json")
			if jsonTag == "" {
				jsonTag = f.Name
			}
			fields = append(fields, fmt.Sprintf("%s (%s)", jsonTag, f.Type.Name()))
		}
		sb.WriteString(strings.Join(fields, ", "))
		sb.WriteString("\n")
	}
	return sb.String()
}

func executeAgentAction(a AgentAction, actorEmail, actorUID, userPrompt string) (interface{}, error) {
	if a.Action == "" {
		return nil, fmt.Errorf("action is required")
	}
	if a.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	s := NewStore(a.Model)
	if s == nil {
		return nil, fmt.Errorf("model %s not registered", a.Model)
	}

	actorName := actorDisplayName(actorEmail, actorUID)

	switch a.Action {
	case "create":
		if strings.EqualFold(a.Model, "posts") {
			if strings.TrimSpace(actorEmail) == "" && strings.TrimSpace(actorUID) == "" {
				return nil, fmt.Errorf("unauthorized: authenticated user required to create posts via AI")
			}
			if a.Data == nil {
				a.Data = map[string]interface{}{}
			}
			a.Data["author"] = actorName
		}

		obj := clone(s.GetModel())
		dataJSON, _ := json.Marshal(a.Data)
		if err := json.Unmarshal(dataJSON, obj); err != nil {
			return nil, fmt.Errorf("failed to map data to model: %v", err)
		}
		if err := s.Create(obj); err != nil {
			return nil, err
		}
		return obj, nil

	case "read_all":
		slice := makeSlice(s.GetModel())
		if err := s.ReadAll(slice); err != nil {
			return nil, err
		}

		// Apply filters if present in 'data'
		if len(a.Data) > 0 {
			filtered := applyAgentFilters(slice, a.Data)
			return filtered, nil
		}

		return slice, nil

	case "read_by_id":
		if a.ID == nil {
			return nil, fmt.Errorf("id is required for read_by_id")
		}
		obj := clone(s.GetModel())
		if err := s.GetByID(a.ID, obj); err != nil {
			return nil, err
		}
		return obj, nil

	case "update":
		if strings.EqualFold(a.Model, "posts") {
			if strings.TrimSpace(actorEmail) == "" && strings.TrimSpace(actorUID) == "" {
				return nil, fmt.Errorf("unauthorized: authenticated user required to update posts via AI")
			}

			if a.Data == nil {
				a.Data = map[string]interface{}{}
			}
			delete(a.Data, "author") // prevent changing ownership from AI prompt

			if a.ID == nil {
				resolvedID, err := resolveOwnPostID(s, actorName, a.Data, userPrompt, a.Action)
				if err != nil {
					return nil, err
				}
				a.ID = resolvedID
			}

			if err := verifyPostOwnership(s, a.ID, actorName); err != nil {
				return nil, err
			}
		}

		if a.ID == nil {
			return nil, fmt.Errorf("id is required for update")
		}
		if err := s.Update(a.ID, a.Data); err != nil {
			return nil, err
		}
		return "updated successfully", nil

	case "delete":
		if strings.EqualFold(a.Model, "posts") {
			if strings.TrimSpace(actorEmail) == "" && strings.TrimSpace(actorUID) == "" {
				return nil, fmt.Errorf("unauthorized: authenticated user required to delete posts via AI")
			}

			if a.ID == nil {
				resolvedID, err := resolveOwnPostID(s, actorName, a.Data, userPrompt, a.Action)
				if err != nil {
					return nil, err
				}
				a.ID = resolvedID
			}

			if err := verifyPostOwnership(s, a.ID, actorName); err != nil {
				return nil, err
			}
		}

		if a.ID == nil {
			return nil, fmt.Errorf("id is required for delete")
		}
		if err := s.Delete(a.ID); err != nil {
			return nil, err
		}
		return "deleted successfully", nil

	default:
		return nil, fmt.Errorf("unknown action: %s", a.Action)
	}
}

func actorDisplayName(email, uid string) string {
	email = strings.TrimSpace(email)
	if email != "" {
		if at := strings.Index(email, "@"); at > 0 {
			return email[:at]
		}
		return email
	}
	uid = strings.TrimSpace(uid)
	if uid != "" {
		return uid
	}
	return "anonymous"
}

// applyAgentFilters performs in-memory filtering of a slice of models
func applyAgentFilters(data interface{}, filters map[string]interface{}) interface{} {
	val := reflect.ValueOf(data)
	originalWasPtr := false
	if val.Kind() == reflect.Ptr {
		originalWasPtr = true
		if val.IsNil() {
			return data
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Slice {
		return data
	}

	result := reflect.MakeSlice(val.Type(), 0, 0)

	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		match := true

		// Check each filter
		for k, v := range filters {
			fieldValue := getFieldValue(item.Interface(), k)
			if !agentValuesEqual(fieldValue, v) {
				match = false
				break
			}
		}

		if match {
			result = reflect.Append(result, item)
		}
	}

	if originalWasPtr {
		out := reflect.New(result.Type())
		out.Elem().Set(result)
		return out.Interface()
	}

	return result.Interface()
}

func getFieldValue(obj interface{}, fieldName string) interface{} {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		f := typ.Field(i)
		jsonTag := f.Tag.Get("json")
		tagName := strings.Split(jsonTag, ",")[0]
		if tagName == "" {
			tagName = f.Name
		}
		// Match by JSON tag (ignoring omitempty) or field name
		if strings.EqualFold(tagName, fieldName) || strings.EqualFold(f.Name, fieldName) {
			return val.Field(i).Interface()
		}
	}

	return nil
}

func agentValuesEqual(a, b interface{}) bool {
	as, aIsString := a.(string)
	bs, bIsString := b.(string)
	if aIsString && bIsString {
		return strings.EqualFold(strings.TrimSpace(as), strings.TrimSpace(bs))
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func inferReadAllFilters(prompt, model string) map[string]interface{} {
	fields := modelFieldNames(model)
	if len(fields) == 0 {
		return nil
	}

	filters := map[string]interface{}{}

	for _, field := range fields {
		if strings.EqualFold(field, "id") {
			continue
		}

		if value := extractFilterValue(prompt, field); value != "" {
			filters[field] = value
		}
	}

	// Common natural phrasing fallback: "posts by Milan"
	if len(filters) == 0 && containsField(fields, "author") {
		if value := extractByValue(prompt); value != "" {
			filters["author"] = value
		}
	}

	if len(filters) == 0 {
		return nil
	}

	return filters
}

func modelFieldNames(model string) []string {
	m := models[model]
	if m == nil {
		return nil
	}

	t := reflect.TypeOf(m)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			tag = f.Name
		}
		fields = append(fields, tag)
	}

	return fields
}

func containsField(fields []string, target string) bool {
	for _, f := range fields {
		if strings.EqualFold(f, target) {
			return true
		}
	}
	return false
}

func extractFilterValue(prompt, field string) string {
	if strings.TrimSpace(prompt) == "" {
		return ""
	}

	qf := regexp.QuoteMeta(field)

	// Examples matched:
	// author = Milan
	// author is Milan
	// author: Milan
	reKV := regexp.MustCompile(`(?i)\b` + qf + `\b\s*(?:=|is|:)\s*["']?([\w@.\- ]+?)["']?(?:\s*(?:,|\.|$))`)
	if m := reKV.FindStringSubmatch(strings.TrimSpace(prompt)); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// Example matched:
	// by author Milan
	reByField := regexp.MustCompile(`(?i)\bby\s+` + qf + `\s+["']?([\w@.\- ]+?)["']?(?:\s*(?:,|\.|$))`)
	if m := reByField.FindStringSubmatch(strings.TrimSpace(prompt)); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return ""
}

func extractByValue(prompt string) string {
	re := regexp.MustCompile(`(?i)\bby\s+["']?([\w@.\- ]+?)["']?(?:\s*(?:,|\.|$))`)
	if m := re.FindStringSubmatch(strings.TrimSpace(prompt)); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func resolveOwnPostID(s Store, actorName string, data map[string]interface{}, prompt, action string) (interface{}, error) {
	slice := makeSlice(s.GetModel())
	if err := s.ReadAll(slice); err != nil {
		return nil, err
	}

	targetTitle := extractMutationTargetTitle(prompt, data, action)

	val := reflect.ValueOf(slice)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, fmt.Errorf("no posts found")
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("failed to resolve posts")
	}

	candidates := make([]reflect.Value, 0)
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		author := fmt.Sprintf("%v", getFieldValue(item.Interface(), "author"))
		if !strings.EqualFold(strings.TrimSpace(author), strings.TrimSpace(actorName)) {
			continue
		}

		if targetTitle != "" {
			title := fmt.Sprintf("%v", getFieldValue(item.Interface(), "title"))
			if !strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(targetTitle)) {
				continue
			}
		}

		candidates = append(candidates, item)
	}

	if len(candidates) == 0 {
		if targetTitle != "" {
			return nil, fmt.Errorf("no matching post found for your profile with title: %s", targetTitle)
		}
		return nil, fmt.Errorf("could not identify which post to modify. Include id or title in your command")
	}

	if len(candidates) > 1 {
		if targetTitle != "" {
			return nil, fmt.Errorf("multiple posts found with title '%s'. Please provide post id", targetTitle)
		}
		return nil, fmt.Errorf("multiple posts found. Please provide title or post id")
	}

	id := getFieldValue(candidates[0].Interface(), "id")
	if id == nil || strings.TrimSpace(fmt.Sprintf("%v", id)) == "" {
		return nil, fmt.Errorf("resolved post has no id")
	}

	return id, nil
}

func verifyPostOwnership(s Store, id interface{}, actorName string) error {
	obj := clone(s.GetModel())
	if err := s.GetByID(id, obj); err != nil {
		return err
	}

	author := fmt.Sprintf("%v", getFieldValue(obj, "author"))
	if !strings.EqualFold(strings.TrimSpace(author), strings.TrimSpace(actorName)) {
		return fmt.Errorf("forbidden: you can only modify your own posts")
	}

	return nil
}

func extractTitleFromPrompt(prompt string) string {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return ""
	}

	// title "..." / titled "..." / post "..."
	reQuoted := regexp.MustCompile(`(?i)\b(?:title|titled|post)\s+["']([^"']+)["']`)
	if m := reQuoted.FindStringSubmatch(p); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// named ...
	reNamed := regexp.MustCompile(`(?i)\bnamed\s+([\w@.\- ]+?)(?:\s*(?:,|\.|$))`)
	if m := reNamed.FindStringSubmatch(p); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return ""
}

func extractMutationTargetTitle(prompt string, data map[string]interface{}, action string) string {
	p := strings.TrimSpace(prompt)

	// Highest priority: explicit target fields from AI JSON.
	if data != nil {
		if mt, ok := data["match_title"].(string); ok && strings.TrimSpace(mt) != "" {
			return strings.TrimSpace(mt)
		}
		if ot, ok := data["old_title"].(string); ok && strings.TrimSpace(ot) != "" {
			return strings.TrimSpace(ot)
		}
		if ot, ok := data["target_title"].(string); ok && strings.TrimSpace(ot) != "" {
			return strings.TrimSpace(ot)
		}

		// For delete/update, title in data commonly identifies the existing post.
		if t, ok := data["title"].(string); ok && strings.TrimSpace(t) != "" &&
			(strings.EqualFold(action, "delete") || strings.EqualFold(action, "update")) {
			return strings.TrimSpace(t)
		}
	}

	// Strongest signal for updates: from "old" to "new"
	if strings.EqualFold(action, "update") {
		reFromTo := regexp.MustCompile(`(?i)\bfrom\s+["']([^"']+)["']\s+to\s+["'][^"']+["']`)
		if m := reFromTo.FindStringSubmatch(p); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}

	// delete/update post titled "..."
	reTitled := regexp.MustCompile(`(?i)\b(?:delete|remove|edit|update)\b.*\b(?:titled|title|post)\s+["']([^"']+)["']`)
	if m := reTitled.FindStringSubmatch(p); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// fallback generic extraction from prompt
	if t := extractTitleFromPrompt(p); t != "" {
		return t
	}

	return ""
}

func normalizeActionName(action string) string {
	a := strings.ToLower(strings.TrimSpace(action))
	switch a {
	case "edit", "modify", "patch":
		return "update"
	case "remove":
		return "delete"
	case "read", "list", "get_all":
		return "read_all"
	case "get", "find":
		return "read_by_id"
	default:
		return a
	}
}

func normalizeModelName(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return m
	}

	if _, ok := models[m]; ok {
		return m
	}
	if !strings.HasSuffix(m, "s") {
		if _, ok := models[m+"s"]; ok {
			return m + "s"
		}
	}
	return m
}

func inferActionFromPrompt(prompt string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return "", false
	}

	if strings.Contains(p, "delete") || strings.Contains(p, "remove") {
		return "delete", true
	}
	if strings.Contains(p, "update") || strings.Contains(p, "edit") || strings.Contains(p, "change") || strings.Contains(p, "modify") {
		return "update", true
	}
	if strings.Contains(p, "create") || strings.Contains(p, "new post") || strings.Contains(p, "write") || strings.Contains(p, "generate") {
		return "create", true
	}
	if strings.Contains(p, "by id") || strings.Contains(p, "id ") {
		return "read_by_id", true
	}
	if strings.Contains(p, "list") || strings.Contains(p, "show") || strings.Contains(p, "fetch") || strings.Contains(p, "get") {
		return "read_all", true
	}

	return "", false
}

func inferModelFromPrompt(prompt string) string {
	p := strings.ToLower(strings.TrimSpace(prompt))
	switch {
	case strings.Contains(p, "post"):
		return "posts"
	case strings.Contains(p, "comment"):
		return "comments"
	case strings.Contains(p, "question"):
		return "questions"
	default:
		return ""
	}
}
