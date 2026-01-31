package api

import (
	"context"
	"encoding/json"
	"net/http"

	"mikromon/internal/db"

	"go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type CustomCommand struct {
    ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
    Title       string             `json:"title" bson:"title"`
    Command     string             `json:"command" bson:"command"` // e.g. "interface print"
    Category    string             `json:"category" bson:"category"` // OLT, ROUTER
    Description string             `json:"description" bson:"description"`
}

func GetCustomCommandsHandler(w http.ResponseWriter, r *http.Request) {
    collection := db.GetCollection("custom_commands")
    var commands []CustomCommand

    if collection == nil {
        commands = []CustomCommand{
            {Title: "Check Optical Power", Command: "show interface optical-module", Category: "OLT", Description: "Show standard optical levels"},
            {Title: "Reboot ONU", Command: "reboot onu", Category: "OLT", Description: "Restart specific ONU unit"},
            {Title: "PPPoE Scan", Command: "interface pppoe-client scan", Category: "ROUTER", Description: "Scan for pppoe servers"},
        }
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
    _, err := collection.InsertOne(context.TODO(), cmd)
    if err != nil {
        http.Error(w, "Error saving command", http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusCreated)
}
