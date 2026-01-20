package grit

func AuthRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type":    "string",
				"example": "test@gmail.com",
			},
			"password": map[string]interface{}{
				"type":    "string",
				"example": "test@1221",
			},
		},
		"required": []string{"email", "password"},
	}
}
