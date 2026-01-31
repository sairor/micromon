package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"mikromon/internal/db"
	"mikromon/internal/ssher"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CustomCommand struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Command     string             `json:"command" bson:"command"`   // e.g. "interface print"
	Category    string             `json:"category" bson:"category"` // OLT, ROUTER, SWITCH
	Description string             `json:"description" bson:"description"`
	Icon        string             `json:"icon" bson:"icon"` // Emoji or URL
}

var MockCommands = []CustomCommand{
	{ID: primitive.NewObjectID(), Title: "Potência Óptica", Command: "show interface optical-module", Category: "OLT", Description: "Verifica níveis de sinal", Icon: "🔆"},
	{ID: primitive.NewObjectID(), Title: "Reiniciar ONU", Command: "reboot onu %id%", Category: "OLT", Description: "Reinicia unidade ONU específica", Icon: "🔄"},
	{ID: primitive.NewObjectID(), Title: "Scan PPPoE", Command: "/interface pppoe-client scan", Category: "ROUTER", Description: "Varredura de servidores", Icon: "🔍"},
	{ID: primitive.NewObjectID(), Title: "Monitorar CPU", Command: "/system resource monitor", Category: "ROUTER", Description: "Uso de CPU em tempo real", Icon: "📊"},
	{ID: primitive.NewObjectID(), Title: "Estado Portas", Command: "/interface ethernet print", Category: "SWITCH", Description: "Status físico das portas", Icon: "🔌"},
	{ID: primitive.NewObjectID(), Title: "VLAN Check", Command: "/interface vlan print", Category: "SWITCH", Description: "Lista de VLANs configuradas", Icon: "🏢"},
}

func GetCustomCommandsHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("custom_commands")
	var commands []CustomCommand

	if collection == nil {
		commands = MockCommands
	} else {
		cursor, err := collection.Find(context.TODO(), bson.M{})
		if err != nil {
			http.Error(w, "Error fetching commands", http.StatusInternalServerError)
			return
		}
		if err = cursor.All(context.TODO(), &commands); err != nil {
			http.Error(w, "Error decoding commands", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(commands)
}

func CreateCustomCommandHandler(w http.ResponseWriter, r *http.Request) {
	var cmd CustomCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	collection := db.GetCollection("custom_commands")
	if collection == nil {
		cmd.ID = primitive.NewObjectID()
		MockCommands = append(MockCommands, cmd)
	} else {
		_, err := collection.InsertOne(context.TODO(), cmd)
		if err != nil {
			http.Error(w, "Error saving command", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// ExecuteCommandRequest defines payload for execution
type ExecuteCommandRequest struct {
	CommandID string            `json:"command_id"`
	Host      string            `json:"host"`
	Port      int               `json:"port"`
	User      string            `json:"user"`
	Password  string            `json:"password"`
	UseSSHKey bool              `json:"use_ssh_key"`
	Params    map[string]string `json:"params"` // For simple placeholders replacement
}

// RunCustomCommandHandler executes a saved command via SSH
func RunCustomCommandHandler(w http.ResponseWriter, r *http.Request) {
	var req ExecuteCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// 1. Fetch Command from DB
	collection := db.GetCollection("custom_commands")
	var cmd CustomCommand
	if collection != nil {
		objID, err := primitive.ObjectIDFromHex(req.CommandID)
		if err == nil {
			err = collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&cmd)
		}
		if err != nil {
			// Try to see if it is in Mock
			found := false
			for _, mc := range MockCommands {
				if mc.ID.Hex() == req.CommandID {
					cmd = mc
					found = true
					break
				}
			}
			if !found {
				http.Error(w, "Command not found", http.StatusNotFound)
				return
			}
		}
	} else {
		// Mock lookup
		found := false
		for _, mc := range MockCommands {
			if mc.ID.Hex() == req.CommandID {
				cmd = mc
				found = true
				break
			}
		}
		if !found && len(MockCommands) > 0 {
			cmd = MockCommands[0]
		}
	}

	// 2. Prepare Command (Replace placeholders if any)
	finalCmd := cmd.Command
	for k, v := range req.Params {
		finalCmd = strings.ReplaceAll(finalCmd, "%"+k+"%", v)
	}

	// 3. Execute via SSH Pool
	if req.Port == 0 {
		req.Port = 22
	}

	pool := ssher.GetPool()
	output, err := pool.RunCommand(req.User, req.Password, req.Host, req.Port, req.UseSSHKey, finalCmd)

	if err != nil {
		http.Error(w, "Command execution failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"output":    output,
		"command":   finalCmd,
		"timestamp": time.Now().String(),
	})
}
