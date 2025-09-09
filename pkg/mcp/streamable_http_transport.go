package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"calculator-server/internal/types"
)

// StreamableHTTPTransport implements MCP-compliant streamable HTTP transport
type StreamableHTTPTransport struct {
	server      *http.Server
	mcpServer   *Server
	config      *StreamableHTTPConfig
	sessions    map[string]*types.Session
	sessionsMux sync.RWMutex
	connections int32
}

// StreamableHTTPConfig contains MCP-compliant HTTP transport configuration
type StreamableHTTPConfig struct {
	Host             string
	Port             int
	SessionTimeout   time.Duration
	MaxConnections   int
	CORSEnabled      bool
	CORSOrigins      []string
}

// NewStreamableHTTPTransport creates a new MCP-compliant HTTP transport instance
func NewStreamableHTTPTransport(mcpServer *Server, config *StreamableHTTPConfig) *StreamableHTTPTransport {
	if config == nil {
		config = &StreamableHTTPConfig{
			Host:             "127.0.0.1",
			Port:             8080,
			SessionTimeout:   5 * time.Minute,
			MaxConnections:   100,
			CORSEnabled:      true,
			CORSOrigins:      []string{"*"},
		}
	}

	transport := &StreamableHTTPTransport{
		mcpServer: mcpServer,
		config:    config,
		sessions:  make(map[string]*types.Session),
	}

	mux := http.NewServeMux()
	transport.setupRoutes(mux)

	transport.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: transport.corsMiddleware(mux),
	}

	// Start session cleanup goroutine
	go transport.cleanupExpiredSessions()

	return transport
}

// setupRoutes configures MCP-compliant HTTP routes
func (t *StreamableHTTPTransport) setupRoutes(mux *http.ServeMux) {
	// Single MCP endpoint as per specification
	mux.HandleFunc("/mcp", t.handleMCP)
}

// corsMiddleware adds CORS headers if enabled
func (t *StreamableHTTPTransport) corsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t.config.CORSEnabled {
			origin := r.Header.Get("Origin")
			if t.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, MCP-Protocol-Version, Mcp-Session-Id")
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
func (t *StreamableHTTPTransport) isOriginAllowed(origin string) bool {
	for _, allowed := range t.config.CORSOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// handleMCP handles MCP requests according to the streamable HTTP specification
func (t *StreamableHTTPTransport) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Validate MCP Protocol Version
	protocolVersion := r.Header.Get("MCP-Protocol-Version")
	if protocolVersion == "" {
		http.Error(w, "MCP-Protocol-Version header required", http.StatusBadRequest)
		return
	}

	// Handle session management
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		if !t.isValidSession(sessionID) {
			http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
			return
		}
		t.updateSessionActivity(sessionID)
	}

	switch r.Method {
	case http.MethodPost:
		t.handlePOST(w, r, sessionID)
	case http.MethodGet:
		t.handleGET(w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePOST handles POST requests with JSON-RPC
func (t *StreamableHTTPTransport) handlePOST(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Validate Accept header
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "Accept header must include application/json or text/event-stream", http.StatusBadRequest)
		return
	}

	// Read and parse JSON-RPC request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var mcpReq types.MCPRequest
	if err := json.Unmarshal(body, &mcpReq); err != nil {
		t.writeErrorResponse(w, nil, ErrorCodeInvalidRequest, "Invalid JSON-RPC request", err.Error())
		return
	}

	// Process MCP request
	response := t.mcpServer.HandleRequest(mcpReq)

	// Check if client accepts SSE streaming
	if strings.Contains(accept, "text/event-stream") && t.shouldStream(&mcpReq) {
		t.writeSSEResponse(w, response, sessionID)
	} else {
		t.writeJSONResponse(w, response)
	}
}

// handleGET handles GET requests for SSE streams
func (t *StreamableHTTPTransport) handleGET(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Validate Accept header for SSE
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "Accept header must include text/event-stream for GET requests", http.StatusBadRequest)
		return
	}

	// Create new session if not provided
	if sessionID == "" {
		sessionID = t.createSession()
		log.Printf("Created new session: %s", sessionID)
	}

	// Setup SSE stream
	t.setupSSEStream(w, r, sessionID)
}

// shouldStream determines if a request should use SSE streaming
func (t *StreamableHTTPTransport) shouldStream(req *types.MCPRequest) bool {
	// For now, we'll stream for tool calls that might take longer
	return req.Method == "tools/call"
}

// writeSSEResponse writes a response using Server-Sent Events
func (t *StreamableHTTPTransport) writeSSEResponse(w http.ResponseWriter, response types.MCPResponse, sessionID string) {
	// Setup SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if sessionID != "" {
		w.Header().Set("Mcp-Session-Id", sessionID)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Server does not support streaming", http.StatusInternalServerError)
		return
	}

	// Write SSE event
	eventID := t.generateEventID()
	responseJSON, _ := json.Marshal(response)
	
	fmt.Fprintf(w, "id: %s\n", eventID)
	fmt.Fprintf(w, "event: message\n")
	fmt.Fprintf(w, "data: %s\n\n", responseJSON)
	flusher.Flush()
}

// setupSSEStream establishes an SSE stream connection
func (t *StreamableHTTPTransport) setupSSEStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Setup SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Mcp-Session-Id", sessionID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Server does not support streaming", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "id: %s\n", t.generateEventID())
	fmt.Fprintf(w, "event: connection\n")
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"session_id\":\"%s\"}\n\n", sessionID)
	flusher.Flush()

	// Keep connection alive with periodic heartbeats
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "id: %s\n", t.generateEventID())
			fmt.Fprintf(w, "event: heartbeat\n")
			fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
			flusher.Flush()
		}
	}
}

// writeJSONResponse writes a standard JSON response
func (t *StreamableHTTPTransport) writeJSONResponse(w http.ResponseWriter, response types.MCPResponse) {
	w.Header().Set("Content-Type", "application/json")
	
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

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// writeErrorResponse writes an error response
func (t *StreamableHTTPTransport) writeErrorResponse(w http.ResponseWriter, id interface{}, code int, message, data string) {
	response := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	t.writeJSONResponse(w, response)
}

// Session Management Functions

// createSession generates a new cryptographically secure session ID
func (t *StreamableHTTPTransport) createSession() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	sessionID := hex.EncodeToString(bytes)

	t.sessionsMux.Lock()
	defer t.sessionsMux.Unlock()

	t.sessions[sessionID] = &types.Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		Active:    true,
	}

	return sessionID
}

// isValidSession checks if a session ID is valid and active
func (t *StreamableHTTPTransport) isValidSession(sessionID string) bool {
	t.sessionsMux.RLock()
	defer t.sessionsMux.RUnlock()

	session, exists := t.sessions[sessionID]
	if !exists || !session.Active {
		return false
	}

	// Check if session has expired
	if time.Since(session.LastSeen) > t.config.SessionTimeout {
		return false
	}

	return true
}

// updateSessionActivity updates the last seen time for a session
func (t *StreamableHTTPTransport) updateSessionActivity(sessionID string) {
	t.sessionsMux.Lock()
	defer t.sessionsMux.Unlock()

	if session, exists := t.sessions[sessionID]; exists {
		session.LastSeen = time.Now()
	}
}

// cleanupExpiredSessions removes expired sessions periodically
func (t *StreamableHTTPTransport) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		t.sessionsMux.Lock()
		now := time.Now()
		for id, session := range t.sessions {
			if now.Sub(session.LastSeen) > t.config.SessionTimeout {
				delete(t.sessions, id)
				log.Printf("Cleaned up expired session: %s", id)
			}
		}
		t.sessionsMux.Unlock()
	}
}

// generateEventID generates a unique event ID for SSE
func (t *StreamableHTTPTransport) generateEventID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Transport interface implementation

// Start starts the HTTP server
func (t *StreamableHTTPTransport) Start() error {
	log.Printf("Starting MCP streamable HTTP server on %s", t.server.Addr)
	return t.server.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server
func (t *StreamableHTTPTransport) Stop(ctx context.Context) error {
	log.Println("Shutting down MCP streamable HTTP server...")
	return t.server.Shutdown(ctx)
}

// GetAddr returns the server address
func (t *StreamableHTTPTransport) GetAddr() string {
	return t.server.Addr
}