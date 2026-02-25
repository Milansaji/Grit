package grit

import (
	"encoding/json"
	"net/http"
)

func OpenAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paths := map[string]interface{}{}
		for _, d := range getDocs() {
			if _, ok := paths[d.Path]; !ok {
				paths[d.Path] = map[string]interface{}{}
			}
			methodObj := map[string]interface{}{
				"summary": d.Summary,
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Success",
					},
				},
			}
			// 🔐 JWT support
			if d.Protected {
				methodObj["security"] = []map[string]interface{}{
					{"BearerAuth": []string{}},
				}
			}

			// ✅ PARAMETERS (GET, DELETE, PUT) - Swagger UI will show input fields
			if (d.Method == http.MethodGet || d.Method == http.MethodDelete || d.Method == http.MethodPut) &&
				len(d.Params) > 0 {
				params := []map[string]interface{}{}
				for _, p := range d.Params {
					in := p.In
					if in == "" {
						in = "query" // Can be "query", "path", "header"
					}
					t := p.Type
					if t == "" {
						t = "string"
					}

					paramObj := map[string]interface{}{
						"name":        p.Name,
						"in":          in,
						"required":    p.Required,
						"description": p.Description, // Add description if available
						"schema": map[string]interface{}{
							"type": t,
						},
					}

					// Add example value if provided
					if p.Example != nil && p.Example != "" {
						paramObj["schema"].(map[string]interface{})["example"] = p.Example
					}

					params = append(params, paramObj)
				}
				methodObj["parameters"] = params
			}

			// 🔥 REQUEST BODY (POST, PUT) - Swagger UI will show JSON editor
			if d.Method == http.MethodPost || d.Method == http.MethodPut {
				schema := d.Body
				if schema == nil {
					schema = AuthRequestSchema() // Your default schema
				}
				methodObj["requestBody"] = map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": schema,
						},
					},
				}
			}

			paths[d.Path].(map[string]interface{})[lower(d.Method)] = methodObj
		}

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		spec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":       "Grit API",
				"description": "Auto-generated API documentation",
				"version":     "1.0.0",
			},
			"servers": []map[string]interface{}{
				{
					"url": scheme + "://" + r.Host,
				},
			},
			"paths": paths,
			"components": map[string]interface{}{
				"securitySchemes": map[string]interface{}{
					"BearerAuth": map[string]interface{}{
						"type":         "http",
						"scheme":       "bearer",
						"bearerFormat": "JWT",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}
}

func lower(method string) string {
	switch method {
	case "GET":
		return "get"
	case "POST":
		return "post"
	case "PUT":
		return "put"
	case "DELETE":
		return "delete"
	case "PATCH":
		return "patch"
	default:
		return "get"
	}
}
