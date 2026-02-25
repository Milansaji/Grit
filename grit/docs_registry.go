package grit

type DocParam struct {
	Name        string
	In          string // "query", "path", or "header"
	Required    bool
	Type        string // "string", "integer", "boolean", "number", "array"
	Example     interface{}
	Description string // Description for Swagger UI
}

type DocEntry struct {
	Method      string
	Path        string
	Summary     string
	Description string // Endpoint description
	Protected   bool
	Body        map[string]interface{}
	Params      []DocParam
}

var docsRegistry = map[string]DocEntry{}

func docKey(method, path string) string {
	return method + ":" + path
}

// Auto registration (router uses this)
func registerDocs(method, path string) {
	key := docKey(method, path)
	if _, ok := docsRegistry[key]; ok {
		return
	}
	docsRegistry[key] = DocEntry{
		Method:      method,
		Path:        path,
		Summary:     method + " " + path,
		Description: "Auto-generated endpoint for " + path,
	}
}

// Manual override
func RegisterDoc(d DocEntry) {
	key := docKey(d.Method, d.Path)
	docsRegistry[key] = d
}

func getDocs() []DocEntry {
	out := make([]DocEntry, 0, len(docsRegistry))
	for _, d := range docsRegistry {
		out = append(out, d)
	}
	return out
}
