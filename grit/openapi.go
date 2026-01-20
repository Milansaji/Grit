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

			// 🔥 BODY FOR ALL ENDPOINTS (NO METHOD CHECK)
			schema := d.Body
			if schema == nil {
				schema = AuthRequestSchema()
			}

			methodObj["requestBody"] = map[string]interface{}{
				"required": true,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": schema,
					},
				},
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
