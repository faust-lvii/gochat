package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	"gochat/internal/websocket"
)

// Configuration constants
const (
	readBufferSize    = 4096
	writeBufferSize   = 4096
	handshakeTimeout  = 10 * time.Second
	maxMessageSize    = 32 * 1024 // 32KB
	pongWaitTime      = 60 * time.Second
	writeWaitTime     = 10 * time.Second
	defaultUsername   = "Anonymous"
	minUsernameLength = 3
	maxUsernameLength = 20
)

// WebSocket upgrader with security settings
var upgrader = gorilla.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should implement proper origin checking
		// For development, we allow all origins
		return true
	},
	HandshakeTimeout: handshakeTimeout,
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *websocket.Hub
}

// NewWebSocketHandler creates a new WebSocketHandler instance
func NewWebSocketHandler(hub *websocket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// HandleWebSocket upgrades HTTP connection to WebSocket and manages the connection
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	// Configure connection settings
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWaitTime))
	conn.SetWriteDeadline(time.Now().Add(writeWaitTime))

	// Get username from query parameter
	username := sanitizeUsername(c.Query("username"))

	// Check if username is already taken
	if h.hub.IsUsernameTaken(username) {
		// Send error message and close connection
		closeMessage := websocket.Message{
			Type:    websocket.MessageTypeError,
			Content: "Username is already taken. Please choose another one.",
			Sender:  "System",
			SentAt:  time.Now(),
		}
		
		messageJSON, _ := json.Marshal(closeMessage)
		conn.WriteMessage(gorilla.TextMessage, messageJSON)
		conn.Close()
		return
	}

	// Create new client
	client := websocket.NewClient(conn, h.hub, username)

	// Register client to hub
	h.hub.Register <- client

	// Start goroutines for reading and writing
	go client.ReadPump()
	go client.WritePump()
}

// sanitizeUsername validates and sanitizes the username
func sanitizeUsername(username string) string {
	// Trim whitespace
	username = strings.TrimSpace(username)
	
	// If empty or too short, use default
	if username == "" || len(username) < minUsernameLength {
		return defaultUsername
	}
	
	// If too long, truncate
	if len(username) > maxUsernameLength {
		username = username[:maxUsernameLength]
	}
	
	// Remove any potentially dangerous characters
	username = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, username)
	
	// If after filtering it's empty or too short, use default
	if username == "" || len(username) < minUsernameLength {
		return defaultUsername
	}
	
	return username
}