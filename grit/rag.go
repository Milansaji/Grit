package grit

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
)

// VectorStoreItem represents a text chunk and its embedding.
type VectorStoreItem struct {
	Text      string
	Embedding []float32
}

// SimpleVectorStore is an in-memory vector store for RAG.
type SimpleVectorStore struct {
	Items []VectorStoreItem
}

// NewSimpleVectorStore creates a new in-memory vector store.
func NewSimpleVectorStore() *SimpleVectorStore {
	return &SimpleVectorStore{
		Items: []VectorStoreItem{},
	}
}

// AddDocument adds a text chunk and its embedding to the store.
func (s *SimpleVectorStore) AddDocument(text string, embedding []float32) {
	s.Items = append(s.Items, VectorStoreItem{
		Text:      text,
		Embedding: embedding,
	})
}

// CosineSimilarity calculates the similarity between two vectors.
func CosineSimilarity(v1, v2 []float32) float32 {
	if len(v1) != len(v2) {
		return 0
	}
	var dotProduct, mag1, mag2 float64
	for i := range v1 {
		dotProduct += float64(v1[i]) * float64(v2[i])
		mag1 += float64(v1[i]) * float64(v1[i])
		mag2 += float64(v2[i]) * float64(v2[i])
	}
	if mag1 == 0 || mag2 == 0 {
		return 0
	}
	return float32(dotProduct / (math.Sqrt(mag1) * math.Sqrt(mag2)))
}

// Search find the topK most similar documents to the query embedding.
func (s *SimpleVectorStore) Search(queryEmbedding []float32, topK int) []string {
	type result struct {
		Text  string
		Score float32
	}
	results := make([]result, len(s.Items))

	for i, item := range s.Items {
		results[i] = result{
			Text:  item.Text,
			Score: CosineSimilarity(queryEmbedding, item.Embedding),
		}
	}

	// Simple sort (not the most efficient but fine for small stores)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > len(results) {
		topK = len(results)
	}

	output := make([]string, topK)
	for i := 0; i < topK; i++ {
		output[i] = results[i].Text
	}
	return output
}

// SplitText breaks down a large string into smaller chunks.
func SplitText(text string, chunkSize int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	words := strings.Fields(text)
	var currentChunk strings.Builder
	currentLen := 0

	for _, word := range words {
		if currentLen+len(word)+1 > chunkSize && currentLen > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentLen = 0
		}
		if currentLen > 0 {
			currentChunk.WriteString(" ")
			currentLen++
		}
		currentChunk.WriteString(word)
		currentLen += len(word)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// ============================================================
// 🌐 Built-in API Handlers
// ============================================================

// RAGIngestRequest represents the input for RAGIngestHandler.
type RAGIngestRequest struct {
	Text      string `json:"text"`
	ChunkSize int    `json:"chunk_size"` // optional, default 200
}

// RAGQueryRequest represents the input for RAGQueryHandler.
type RAGQueryRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"` // optional, default 2
}

// RAGIngestHandler returns a handler that chunks, embeds, and stores text.
func RAGIngestHandler(store *SimpleVectorStore, embedModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RAGIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RenderJSON(w, map[string]interface{}{"success": false, "error": "Invalid JSON"})
			return
		}

		if req.Text == "" {
			RenderJSON(w, map[string]interface{}{"success": false, "error": "text is required"})
			return
		}

		if req.ChunkSize <= 0 {
			req.ChunkSize = 200
		}

		chunks := SplitText(req.Text, req.ChunkSize)
		for _, chunk := range chunks {
			embedding, err := Embed(embedModel, chunk)
			if err != nil {
				RenderJSON(w, map[string]interface{}{"success": false, "error": fmt.Sprintf("Embedding error: %v", err)})
				return
			}
			store.AddDocument(chunk, embedding)
		}

		RenderJSON(w, map[string]interface{}{
			"success": true,
			"message": "Ingested successfully",
			"chunks":  len(chunks),
		})
	}
}

// RAGQueryHandler returns a handler that searches context and generates an LLM response.
func RAGQueryHandler(store *SimpleVectorStore, llmModel, embedModel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RAGQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RenderJSON(w, map[string]interface{}{"success": false, "error": "Invalid JSON"})
			return
		}

		if req.Query == "" {
			RenderJSON(w, map[string]interface{}{"success": false, "error": "query is required"})
			return
		}

		if req.TopK <= 0 {
			req.TopK = 2
		}

		// 1. Embed Query
		queryEmbedding, err := Embed(embedModel, req.Query)
		if err != nil {
			RenderJSON(w, map[string]interface{}{"success": false, "error": fmt.Sprintf("Query embedding error: %v", err)})
			return
		}

		// 2. Retrieve Context
		relevantChunks := store.Search(queryEmbedding, req.TopK)
		context := strings.Join(relevantChunks, "\n")

		// 3. Generate Prompt
		prompt := fmt.Sprintf(`Use the following context to answer the question.
Context:
%s

Question: %s
Answer:`, context, req.Query)

		// 4. Get LLM Response
		response, err := Prompt(llmModel, prompt)
		if err != nil {
			RenderJSON(w, map[string]interface{}{"success": false, "error": fmt.Sprintf("LLM error: %v", err)})
			return
		}

		RenderJSON(w, map[string]interface{}{
			"success":  true,
			"response": response,
			"context":  relevantChunks,
		})
	}
}
