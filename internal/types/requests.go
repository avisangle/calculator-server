package types

import "encoding/json"

// MCP Protocol Types
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool Types
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Calculator Request Types
type BasicMathRequest struct {
	Operation string    `json:"operation"`
	Operands  []float64 `json:"operands"`
	Precision int       `json:"precision,omitempty"`
}

type AdvancedMathRequest struct {
	Function string  `json:"function"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit,omitempty"`
}

type ExpressionRequest struct {
	Expression string             `json:"expression"`
	Variables  map[string]float64 `json:"variables,omitempty"`
}

type StatisticsRequest struct {
	Data      []float64 `json:"data"`
	Operation string    `json:"operation"`
}

type UnitConversionRequest struct {
	Value    float64 `json:"value"`
	FromUnit string  `json:"fromUnit"`
	ToUnit   string  `json:"toUnit"`
	Category string  `json:"category"`
}

type FinancialRequest struct {
	Operation   string  `json:"operation"`
	Principal   float64 `json:"principal,omitempty"`
	Rate        float64 `json:"rate,omitempty"`
	Time        float64 `json:"time,omitempty"`
	Periods     int     `json:"periods,omitempty"`
	FutureValue float64 `json:"futureValue,omitempty"`
}

// Response Types
type CalculationResult struct {
	Result float64 `json:"result"`
	Unit   string  `json:"unit,omitempty"`
}

type StatisticsResult struct {
	Result interface{} `json:"result"`
	Count  int         `json:"count"`
}

type FinancialResult struct {
	Result      float64                `json:"result"`
	Breakdown   map[string]interface{} `json:"breakdown,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// HTTP-specific types and structures
type HTTPRequestMetadata struct {
	UserAgent    string            `json:"user_agent,omitempty"`
	RemoteAddr   string            `json:"remote_addr,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Timestamp    string            `json:"timestamp,omitempty"`
	RequestID    string            `json:"request_id,omitempty"`
}

type HTTPResponse struct {
	MCPResponse
	Metadata *HTTPRequestMetadata `json:"metadata,omitempty"`
}

type HealthCheckResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime,omitempty"`
}

type MetricsResponse struct {
	Server   ServerMetrics   `json:"server"`
	Requests RequestMetrics  `json:"requests"`
	Tools    []ToolMetrics   `json:"tools,omitempty"`
}

type ServerMetrics struct {
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
	Transport string `json:"transport"`
	StartTime string `json:"start_time,omitempty"`
}

type RequestMetrics struct {
	Total        int64   `json:"total"`
	Success      int64   `json:"success"`
	Errors       int64   `json:"errors"`
	AvgResponse  float64 `json:"avg_response_time_ms,omitempty"`
}

type ToolMetrics struct {
	Name        string  `json:"name"`
	Invocations int64   `json:"invocations"`
	Errors      int64   `json:"errors"`
	AvgDuration float64 `json:"avg_duration_ms"`
}