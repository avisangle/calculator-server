package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"calculator-server/internal/types"
)

const (
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
)

type Server struct {
	tools map[string]ToolHandler
}

type ToolHandler func(params map[string]interface{}) (interface{}, error)

func NewServer() *Server {
	return &Server{
		tools: make(map[string]ToolHandler),
	}
}

func (s *Server) RegisterTool(name string, description string, inputSchema map[string]interface{}, handler ToolHandler) {
	s.tools[name] = handler
}

func (s *Server) HandleRequest(req types.MCPRequest) types.MCPResponse {
	response := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		response.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "calculator-server",
				"version": "1.0.0",
			},
		}
	case "tools/list":
		tools := []types.Tool{}
		for name, _ := range s.tools {
			tool := s.getToolDefinition(name)
			tools = append(tools, tool)
		}
		response.Result = types.ListToolsResult{Tools: tools}
	case "tools/call":
		var params types.CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			response.Error = &types.MCPError{
				Code:    ErrorCodeInvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			}
			return response
		}

		handler, exists := s.tools[params.Name]
		if !exists {
			response.Error = &types.MCPError{
				Code:    ErrorCodeMethodNotFound,
				Message: "Tool not found",
				Data:    params.Name,
			}
			return response
		}

		result, err := handler(params.Arguments)
		if err != nil {
			response.Error = &types.MCPError{
				Code:    ErrorCodeInternalError,
				Message: "Tool execution failed",
				Data:    err.Error(),
			}
			return response
		}

		resultJSON, _ := json.Marshal(result)
		response.Result = types.CallToolResult{
			Content: []types.ContentBlock{
				{
					Type: "text",
					Text: string(resultJSON),
				},
			},
		}
	default:
		response.Error = &types.MCPError{
			Code:    ErrorCodeMethodNotFound,
			Message: "Method not found",
			Data:    req.Method,
		}
	}

	return response
}

func (s *Server) getToolDefinition(name string) types.Tool {
	// Tool definitions with schemas
	toolDefinitions := map[string]types.Tool{
		"basic_math": {
			Name:        "basic_math",
			Description: "Perform basic mathematical operations (add, subtract, multiply, divide)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type": "string",
						"enum": []string{"add", "subtract", "multiply", "divide"},
					},
					"operands": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "number",
						},
						"minItems": 2,
					},
					"precision": map[string]interface{}{
						"type": "integer",
						"minimum": 0,
						"maximum": 15,
						"default": 2,
					},
				},
				"required": []string{"operation", "operands"},
			},
		},
		"advanced_math": {
			Name:        "advanced_math",
			Description: "Perform advanced mathematical functions (trigonometry, logarithms, etc.)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"function": map[string]interface{}{
						"type": "string",
						"enum": []string{"sin", "cos", "tan", "asin", "acos", "atan", "log", "log10", "ln", "sqrt", "abs", "factorial", "pow", "exp"},
					},
					"value": map[string]interface{}{
						"type": "number",
					},
					"unit": map[string]interface{}{
						"type": "string",
						"enum": []string{"radians", "degrees"},
						"default": "radians",
					},
				},
				"required": []string{"function", "value"},
			},
		},
		"expression_eval": {
			Name:        "expression_eval",
			Description: "Evaluate mathematical expressions with variable substitution",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type": "string",
					},
					"variables": map[string]interface{}{
						"type": "object",
						"patternProperties": map[string]interface{}{
							"^[a-zA-Z][a-zA-Z0-9_]*$": map[string]interface{}{
								"type": "number",
							},
						},
					},
				},
				"required": []string{"expression"},
			},
		},
		"statistics": {
			Name:        "statistics",
			Description: "Perform statistical analysis on data sets",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "number",
						},
						"minItems": 1,
					},
					"operation": map[string]interface{}{
						"type": "string",
						"enum": []string{"mean", "median", "mode", "std_dev", "variance", "percentile"},
					},
				},
				"required": []string{"data", "operation"},
			},
		},
		"unit_conversion": {
			Name:        "unit_conversion",
			Description: "Convert between different units of measurement",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"value": map[string]interface{}{
						"type": "number",
					},
					"fromUnit": map[string]interface{}{
						"type": "string",
					},
					"toUnit": map[string]interface{}{
						"type": "string",
					},
					"category": map[string]interface{}{
						"type": "string",
						"enum": []string{"length", "weight", "temperature", "volume", "area"},
					},
				},
				"required": []string{"value", "fromUnit", "toUnit", "category"},
			},
		},
		"financial": {
			Name:        "financial",
			Description: "Perform financial calculations (interest, loans, ROI)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type": "string",
						"enum": []string{"compound_interest", "simple_interest", "loan_payment", "roi", "present_value", "future_value"},
					},
					"principal": map[string]interface{}{
						"type": "number",
						"minimum": 0,
					},
					"rate": map[string]interface{}{
						"type": "number",
						"minimum": 0,
					},
					"time": map[string]interface{}{
						"type": "number",
						"minimum": 0,
					},
					"periods": map[string]interface{}{
						"type": "integer",
						"minimum": 1,
					},
					"futureValue": map[string]interface{}{
						"type": "number",
						"minimum": 0,
					},
				},
				"required": []string{"operation"},
			},
		},
	}

	if tool, exists := toolDefinitions[name]; exists {
		return tool
	}
	return types.Tool{}
}

func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req types.MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			response := types.MCPResponse{
				JSONRPC: "2.0",
				Error: &types.MCPError{
					Code:    ErrorCodeInvalidRequest,
					Message: "Parse error",
					Data:    err.Error(),
				},
			}
			s.writeResponse(response)
			continue
		}

		response := s.HandleRequest(req)
		s.writeResponse(response)
	}

	return scanner.Err()
}

func (s *Server) writeResponse(response types.MCPResponse) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
		return
	}
	
	fmt.Println(string(responseJSON))
}