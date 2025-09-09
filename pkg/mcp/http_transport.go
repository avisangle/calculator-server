package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"calculator-server/internal/types"
)

// HTTPTransport implements HTTP transport for MCP protocol
type HTTPTransport struct {
	server   *http.Server
	mcpServer *Server
	config   *HTTPConfig
}

// HTTPConfig contains HTTP transport configuration
type HTTPConfig struct {
	Host         string
	Port         int
	CORSEnabled  bool
	CORSOrigins  []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewHTTPTransport creates a new HTTP transport instance
func NewHTTPTransport(mcpServer *Server, config *HTTPConfig) *HTTPTransport {
	if config == nil {
		config = &HTTPConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			CORSEnabled:  true,
			CORSOrigins:  []string{"*"},
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
	}

	transport := &HTTPTransport{
		mcpServer: mcpServer,
		config:    config,
	}

	mux := http.NewServeMux()
	transport.setupRoutes(mux)

	transport.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      transport.corsMiddleware(mux),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return transport
}

// setupRoutes configures HTTP routes
func (t *HTTPTransport) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/mcp", t.handleMCP)
	mux.HandleFunc("/health", t.handleHealth)
	mux.HandleFunc("/tools", t.handleToolsList)
	mux.HandleFunc("/metrics", t.handleMetrics)
}

// corsMiddleware adds CORS headers if enabled
func (t *HTTPTransport) corsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t.config.CORSEnabled {
			origin := r.Header.Get("Origin")
			if t.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if the origin is allowed for CORS
func (t *HTTPTransport) isOriginAllowed(origin string) bool {
	for _, allowed := range t.config.CORSOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// handleMCP handles MCP JSON-RPC requests
func (t *HTTPTransport) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse MCP request
	var mcpReq types.MCPRequest
	if err := json.Unmarshal(body, &mcpReq); err != nil {
		// Try to extract ID from the raw JSON for better error reporting
		var rawMap map[string]interface{}
		var responseID interface{}
		if json.Unmarshal(body, &rawMap) == nil {
			if id, exists := rawMap["id"]; exists {
				responseID = id
			}
		}
		
		response := types.MCPResponse{
			JSONRPC: "2.0",
			ID:      responseID, // Include ID if we could extract it
			Error: &types.MCPError{
				Code:    ErrorCodeInvalidRequest,
				Message: "Invalid JSON-RPC request",
				Data:    err.Error(),
			},
		}
		t.writeJSONResponse(w, response, http.StatusBadRequest)
		return
	}

	// Handle MCP request
	response := t.mcpServer.HandleRequest(mcpReq)
	
	// Determine HTTP status code based on response
	statusCode := http.StatusOK
	if response.Error != nil {
		switch response.Error.Code {
		case ErrorCodeInvalidRequest:
			statusCode = http.StatusBadRequest
		case ErrorCodeMethodNotFound:
			statusCode = http.StatusNotFound
		case ErrorCodeInvalidParams:
			statusCode = http.StatusBadRequest
		case ErrorCodeInternalError:
			statusCode = http.StatusInternalServerError
		default:
			statusCode = http.StatusInternalServerError
		}
	}

	t.writeJSONResponse(w, response, statusCode)
}

// handleHealth handles health check requests
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.1.0",
	}

	t.writeJSONResponse(w, health, http.StatusOK)
}

// handleToolsList handles tools list requests (convenience endpoint)
func (t *HTTPTransport) handleToolsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create a tools/list MCP request
	mcpReq := types.MCPRequest{
		JSONRPC: "2.0",
		ID:      "tools-list",
		Method:  "tools/list",
	}

	response := t.mcpServer.HandleRequest(mcpReq)
	
	statusCode := http.StatusOK
	if response.Error != nil {
		statusCode = http.StatusInternalServerError
	}

	t.writeJSONResponse(w, response, statusCode)
}

// handleMetrics handles basic metrics requests
func (t *HTTPTransport) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := map[string]interface{}{
		"server": map[string]interface{}{
			"uptime":    time.Since(time.Now()).String(), // This would be tracked properly in production
			"version":   "1.1.0",
			"transport": "http",
		},
		"requests": map[string]interface{}{
			"total":   0, // This would be tracked with proper metrics in production
			"success": 0,
			"errors":  0,
		},
	}

	t.writeJSONResponse(w, metrics, http.StatusOK)
}

// writeJSONResponse writes a JSON response with proper headers
func (t *HTTPTransport) writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// Start starts the HTTP server
func (t *HTTPTransport) Start() error {
	log.Printf("Starting HTTP server on %s", t.server.Addr)
	return t.server.ListenAndServe()
}

// StartTLS starts the HTTP server with TLS
func (t *HTTPTransport) StartTLS(certFile, keyFile string) error {
	log.Printf("Starting HTTPS server on %s", t.server.Addr)
	return t.server.ListenAndServeTLS(certFile, keyFile)
}

// Stop gracefully shuts down the HTTP server
func (t *HTTPTransport) Stop(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")
	return t.server.Shutdown(ctx)
}

// GetAddr returns the server address
func (t *HTTPTransport) GetAddr() string {
	return t.server.Addr
}