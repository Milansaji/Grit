package grit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MCPRequest represents a JSON-RPC request from an MCP client (e.g., Claude Desktop).
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response to an MCP client.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// MCPTool defines a tool that the MCP server can execute.
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     func(args map[string]interface{}) (string, error)
}

// MCPServer manages tools and handles the MCP protocol over stdio.
type MCPServer struct {
	Name    string
	Version string
	Tools   map[string]MCPTool
}

// NewMCPServer creates a new MCP server instance.
func NewMCPServer(name, version string) *MCPServer {
	return &MCPServer{
		Name:    name,
		Version: version,
		Tools:   make(map[string]MCPTool),
	}
}

// RegisterTool adds a tool to the MCP server.
func (s *MCPServer) RegisterTool(tool MCPTool) {
	s.Tools[tool.Name] = tool
}

// Start runs the MCP server on stdio (blocking).
func (s *MCPServer) Start() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(req)
	}
}

func (s *MCPServer) handleRequest(req MCPRequest) {
	switch req.Method {
	case "initialize":
		s.sendResponse(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]string{"name": s.Name, "version": s.Version},
		})

	case "notifications/initialized":
		// No response needed

	case "tools/list":
		tools := []map[string]interface{}{}
		for _, t := range s.Tools {
			tools = append(tools, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		s.sendResponse(req.ID, map[string]interface{}{"tools": tools})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)

		tool, ok := s.Tools[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, "Method not found")
			return
		}

		result, err := tool.Handler(params.Arguments)
		if err != nil {
			s.sendResponse(req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			})
			return
		}

		s.sendResponse(req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": result},
			},
		})

	default:
		if req.ID != nil {
			s.sendError(req.ID, -32601, "Method not found")
		}
	}
}

func (s *MCPServer) sendResponse(id interface{}, result interface{}) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *MCPServer) sendError(id interface{}, code int, message string) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   map[string]interface{}{"code": code, "message": message},
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
