package grit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// OllamaRequest represents the request body for the Ollama generate API.
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// EmbeddingsRequest represents the request body for the Ollama embeddings API.
type EmbeddingsRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbeddingsResponse represents the response body from the Ollama embeddings API.
type EmbeddingsResponse struct {
	Embedding []float32 `json:"embedding"`
}

// OllamaResponse represents the response body from the Ollama generate API.
type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// LLMConfig holds the configuration for the local LLM.
type LLMConfig struct {
	BaseURL string
}

var defaultConfig = LLMConfig{
	BaseURL: "http://localhost:11434/api/generate",
}

// SetLLMBaseURL allows overriding the default Ollama URL.
func SetLLMBaseURL(url string) {
	defaultConfig.BaseURL = url
}

// Prompt sends a prompt to the local Ollama instance and returns the response.
// It uses the default model and URL unless overridden.
// If the environment variable OLLAMA_HOST is set, it will be used as the base URL.
func Prompt(model string, prompt string) (string, error) {
	url := defaultConfig.BaseURL
	if envHost := os.Getenv("OLLAMA_HOST"); envHost != "" {
		url = fmt.Sprintf("%s/api/generate", envHost)
	}

	reqBody := OllamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama: %v (is it running?)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return ollamaResp.Response, nil
}

// Embed generates an embedding for a given prompt using the local Ollama instance.
func Embed(model string, prompt string) ([]float32, error) {
	url := "http://localhost:11434/api/embeddings"
	if envHost := os.Getenv("OLLAMA_HOST"); envHost != "" {
		url = fmt.Sprintf("%s/api/embeddings", envHost)
	}

	reqBody := EmbeddingsRequest{
		Model:  model,
		Prompt: prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	var embedResp EmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return embedResp.Embedding, nil
}

// --- External AI APIs (OpenAI, Gemini) ---

// OpenAIResponse represents a simplified OpenAI chat completion response.
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// PromptOpenAI sends a prompt to OpenAI's Chat Completion API.
// Requires OPENAI_API_KEY environment variable.
func PromptOpenAI(model string, prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is not set")
	}

	url := "https://api.openai.com/v1/chat/completions"
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai error: %s", string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return "", err
	}

	if len(openAIResp.Choices) > 0 {
		return openAIResp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response from openai")
}

// PromptGemini sends a prompt to Google's Gemini API.
// Requires GEMINI_API_KEY environment variable.
func PromptGemini(model string, prompt string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	// Default to gemini-1.5-flash if not specified
	if model == "" {
		model = "gemini-1.5-flash"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini error: %s", string(body))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("no response from gemini")
}
