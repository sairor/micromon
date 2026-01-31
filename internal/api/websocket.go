package api

import (
	"log"
	"net/http"

	"mikromon/internal/ssher"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Create a custom CheckOrigin function
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should check the origin of the request
		// to ensure it matches your domain. For now, allow all.
		return true
	},
}

// WSCommandRequest represents the command structure coming from WS
type WSCommandRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Command   string `json:"command"`
	UseSSHKey bool   `json:"use_ssh_key"`
}

// SSHWebSocketHandler handles the websocket connection for terminal output
func SSHWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WS Upgrade Error:", err)
		return
	}
	defer conn.Close()

	var req WSCommandRequest
	if err := conn.ReadJSON(&req); err != nil {
		log.Println("WS Read Error:", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error reading request"))
		return
	}

	// Use the SSH Pool
	pool := ssher.GetPool()

	// Execute command (Stream output not fully implemented in ssher yet,
	// but we can send the final result or implement a streaming reader)
	// For now, let's send "Executing..." and then the result.
	conn.WriteMessage(websocket.TextMessage, []byte("Executing command: "+req.Command+"\r\n"))

	// Validate inputs
	if req.Port == 0 {
		req.Port = 22
	}

	output, err := pool.RunCommand(req.User, req.Password, req.Host, req.Port, req.UseSSHKey, req.Command)
	if err != nil {
		log.Printf("Command execution failed: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()+"\r\n"))
		// We might still want to show output (stderr is logged in RunCommand, but strictly we return stdout string)
		// If ssher returns partial output, we send it.
	}

	// Send output
	// Xterm.js expects \r\n for newlines often.
	conn.WriteMessage(websocket.TextMessage, []byte(output))
	conn.WriteMessage(websocket.TextMessage, []byte("\r\nCommand Completed.\r\n"))
}
