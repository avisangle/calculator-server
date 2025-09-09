package tests

import (
	"encoding/json"
	"strings"
	"testing"
	
	"calculator-server/internal/handlers"
	"calculator-server/internal/types"
	"calculator-server/pkg/mcp"
)

func TestMCPServer_ToolRegistration(t *testing.T) {
	server := mcp.NewServer()
	mathHandler := handlers.NewMathHandler()
	_ = handlers.NewStatsHandler()
	_ = handlers.NewFinanceHandler()
	
	// Register a basic tool
	server.RegisterTool(
		"basic_math",
		"Test basic math operations",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"add", "subtract"},
				},
				"operands": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{"type": "number"},
				},
			},
			"required": []string{"operation", "operands"},
		},
		mathHandler.HandleBasicMath,
	)
	
	// Test list tools request
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify the tool is registered
	resultBytes, _ := json.Marshal(response.Result)
	var listResult types.ListToolsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		t.Errorf("Failed to unmarshal result: %v", err)
		return
	}
	
	if len(listResult.Tools) == 0 {
		t.Errorf("No tools found")
		return
	}
	
	// Debug output
	t.Logf("Found %d tools:", len(listResult.Tools))
	for _, tool := range listResult.Tools {
		t.Logf("  - %s", tool.Name)
	}
	
	found := false
	for _, tool := range listResult.Tools {
		if tool.Name == "basic_math" {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("Test tool not found in tool list")
	}
}

func TestMCPServer_Initialize(t *testing.T) {
	server := mcp.NewServer()
	
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify response structure
	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Errorf("Expected map result")
		return
	}
	
	if protocolVersion, exists := resultMap["protocolVersion"]; !exists || protocolVersion != "2024-11-05" {
		t.Errorf("Unexpected protocol version: %v", protocolVersion)
	}
	
	if serverInfo, exists := resultMap["serverInfo"]; !exists {
		t.Errorf("Missing serverInfo")
	} else {
		serverInfoMap, ok := serverInfo.(map[string]interface{})
		if !ok {
			t.Errorf("serverInfo should be a map")
		} else {
			if name, exists := serverInfoMap["name"]; !exists || name != "calculator-server" {
				t.Errorf("Unexpected server name: %v", name)
			}
		}
	}
}

func TestMCPServer_CallTool_BasicMath(t *testing.T) {
	server := mcp.NewServer()
	mathHandler := handlers.NewMathHandler()
	
	// Register basic math tool
	server.RegisterTool(
		"basic_math",
		"Perform basic mathematical operations",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"add", "subtract", "multiply", "divide"},
				},
				"operands": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{"type": "number"},
				},
				"precision": map[string]interface{}{
					"type": "integer",
					"default": 2,
				},
			},
			"required": []string{"operation", "operands"},
		},
		mathHandler.HandleBasicMath,
	)
	
	// Test tool call
	params := types.CallToolParams{
		Name: "basic_math",
		Arguments: map[string]interface{}{
			"operation": "add",
			"operands":  []interface{}{5.0, 3.0},
			"precision": 2,
		},
	}
	
	paramsBytes, _ := json.Marshal(params)
	
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsBytes,
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify the result
	resultBytes, _ := json.Marshal(response.Result)
	var callResult types.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Errorf("Failed to unmarshal result: %v", err)
		return
	}
	
	if len(callResult.Content) == 0 {
		t.Errorf("No content in result")
		return
	}
	
	content := callResult.Content[0].Text
	if !strings.Contains(content, "8") { // 5 + 3 = 8
		t.Errorf("Expected result to contain '8', got: %s", content)
	}
}

func TestMCPServer_CallTool_Statistics(t *testing.T) {
	server := mcp.NewServer()
	statsHandler := handlers.NewStatsHandler()
	
	// Register statistics tool
	server.RegisterTool(
		"statistics",
		"Perform statistical analysis",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{"type": "number"},
				},
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"mean", "median", "mode", "std_dev"},
				},
			},
			"required": []string{"data", "operation"},
		},
		statsHandler.HandleStatistics,
	)
	
	// Test tool call
	params := types.CallToolParams{
		Name: "statistics",
		Arguments: map[string]interface{}{
			"data":      []interface{}{1.0, 2.0, 3.0, 4.0, 5.0},
			"operation": "mean",
		},
	}
	
	paramsBytes, _ := json.Marshal(params)
	
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsBytes,
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify the result
	resultBytes, _ := json.Marshal(response.Result)
	var callResult types.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Errorf("Failed to unmarshal result: %v", err)
		return
	}
	
	if len(callResult.Content) == 0 {
		t.Errorf("No content in result")
		return
	}
	
	content := callResult.Content[0].Text
	if !strings.Contains(content, "3") { // Mean of [1,2,3,4,5] = 3
		t.Errorf("Expected result to contain '3', got: %s", content)
	}
}

func TestMCPServer_CallTool_UnitConversion(t *testing.T) {
	server := mcp.NewServer()
	statsHandler := handlers.NewStatsHandler()
	
	// Register unit conversion tool
	server.RegisterTool(
		"unit_conversion",
		"Convert between units",
		map[string]interface{}{
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
					"enum": []string{"length", "weight", "temperature"},
				},
			},
			"required": []string{"value", "fromUnit", "toUnit", "category"},
		},
		statsHandler.HandleUnitConversion,
	)
	
	// Test tool call
	params := types.CallToolParams{
		Name: "unit_conversion",
		Arguments: map[string]interface{}{
			"value":    100.0,
			"fromUnit": "cm",
			"toUnit":   "m",
			"category": "length",
		},
	}
	
	paramsBytes, _ := json.Marshal(params)
	
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsBytes,
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify the result
	resultBytes, _ := json.Marshal(response.Result)
	var callResult types.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Errorf("Failed to unmarshal result: %v", err)
		return
	}
	
	if len(callResult.Content) == 0 {
		t.Errorf("No content in result")
		return
	}
	
	content := callResult.Content[0].Text
	if !strings.Contains(content, "1") { // 100 cm = 1 m
		t.Errorf("Expected result to contain '1', got: %s", content)
	}
}

func TestMCPServer_CallTool_Financial(t *testing.T) {
	server := mcp.NewServer()
	financeHandler := handlers.NewFinanceHandler()
	
	// Register financial tool
	server.RegisterTool(
		"financial",
		"Perform financial calculations",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"simple_interest", "compound_interest"},
				},
				"principal": map[string]interface{}{
					"type": "number",
				},
				"rate": map[string]interface{}{
					"type": "number",
				},
				"time": map[string]interface{}{
					"type": "number",
				},
			},
			"required": []string{"operation", "principal", "rate", "time"},
		},
		financeHandler.HandleFinancialCalculation,
	)
	
	// Test tool call
	params := types.CallToolParams{
		Name: "financial",
		Arguments: map[string]interface{}{
			"operation": "simple_interest",
			"principal": 1000.0,
			"rate":      5.0,
			"time":      2.0,
		},
	}
	
	paramsBytes, _ := json.Marshal(params)
	
	req := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsBytes,
	}
	
	response := server.HandleRequest(req)
	
	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
		return
	}
	
	if response.Result == nil {
		t.Errorf("Expected result, got nil")
		return
	}
	
	// Verify the result
	resultBytes, _ := json.Marshal(response.Result)
	var callResult types.CallToolResult
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Errorf("Failed to unmarshal result: %v", err)
		return
	}
	
	if len(callResult.Content) == 0 {
		t.Errorf("No content in result")
		return
	}
	
	content := callResult.Content[0].Text
	if !strings.Contains(content, "100") { // Simple interest: 1000 * 0.05 * 2 = 100
		t.Errorf("Expected result to contain '100', got: %s", content)
	}
}

func TestMCPServer_ErrorHandling(t *testing.T) {
	server := mcp.NewServer()
	
	testCases := []struct {
		name           string
		request        types.MCPRequest
		expectedError  int
		expectedMethod string
	}{
		{
			name: "Invalid method",
			request: types.MCPRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "invalid_method",
			},
			expectedError: -32601, // Method not found
		},
		{
			name: "Tool not found",
			request: types.MCPRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name": "non_existent_tool"}`),
			},
			expectedError: -32601, // Method not found (tool not found)
		},
		{
			name: "Invalid parameters",
			request: types.MCPRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/call",
				Params:  json.RawMessage(`invalid json`),
			},
			expectedError: -32602, // Invalid params
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := server.HandleRequest(tc.request)
			
			if response.Error == nil {
				t.Errorf("Expected error, but got none")
				return
			}
			
			if response.Error.Code != tc.expectedError {
				t.Errorf("Expected error code %d, got %d", tc.expectedError, response.Error.Code)
			}
		})
	}
}

func TestIntegration_CompleteWorkflow(t *testing.T) {
	// This test simulates a complete workflow with multiple tools
	server := mcp.NewServer()
	mathHandler := handlers.NewMathHandler()
	statsHandler := handlers.NewStatsHandler()
	financeHandler := handlers.NewFinanceHandler()
	
	// Register all tools
	server.RegisterTool("basic_math", "Basic math", getBasicMathTestSchema(), mathHandler.HandleBasicMath)
	server.RegisterTool("statistics", "Statistics", getStatisticsTestSchema(), statsHandler.HandleStatistics)
	server.RegisterTool("financial", "Financial", getFinancialTestSchema(), financeHandler.HandleFinancialCalculation)
	
	// Step 1: Initialize
	initReq := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	
	initResp := server.HandleRequest(initReq)
	if initResp.Error != nil {
		t.Errorf("Initialize failed: %v", initResp.Error)
		return
	}
	
	// Step 2: List tools
	listReq := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	
	listResp := server.HandleRequest(listReq)
	if listResp.Error != nil {
		t.Errorf("List tools failed: %v", listResp.Error)
		return
	}
	
	// Step 3: Call multiple tools
	operations := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "basic_math",
			params: map[string]interface{}{
				"operation": "add",
				"operands":  []interface{}{10.0, 20.0, 30.0},
			},
		},
		{
			name: "statistics",
			params: map[string]interface{}{
				"data":      []interface{}{1.0, 2.0, 3.0, 4.0, 5.0},
				"operation": "mean",
			},
		},
		{
			name: "financial",
			params: map[string]interface{}{
				"operation": "simple_interest",
				"principal": 1000.0,
				"rate":      5.0,
				"time":      2.0,
			},
		},
	}
	
	for i, op := range operations {
		params := types.CallToolParams{
			Name:      op.name,
			Arguments: op.params,
		}
		paramsBytes, _ := json.Marshal(params)
		
		callReq := types.MCPRequest{
			JSONRPC: "2.0",
			ID:      i + 3,
			Method:  "tools/call",
			Params:  paramsBytes,
		}
		
		callResp := server.HandleRequest(callReq)
		if callResp.Error != nil {
			t.Errorf("Tool call %s failed: %v", op.name, callResp.Error)
			continue
		}
		
		if callResp.Result == nil {
			t.Errorf("Tool call %s returned no result", op.name)
		}
	}
}

// Helper schemas for testing
func getBasicMathTestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"add", "subtract", "multiply", "divide"},
			},
			"operands": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{"type": "number"},
			},
		},
		"required": []string{"operation", "operands"},
	}
}

func getStatisticsTestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{"type": "number"},
			},
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"mean", "median", "std_dev"},
			},
		},
		"required": []string{"data", "operation"},
	}
}

func getFinancialTestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"simple_interest", "compound_interest"},
			},
			"principal": map[string]interface{}{
				"type": "number",
			},
			"rate": map[string]interface{}{
				"type": "number",
			},
			"time": map[string]interface{}{
				"type": "number",
			},
		},
		"required": []string{"operation", "principal", "rate", "time"},
	}
}