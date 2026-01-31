package api

import (
	"context"
	"encoding/json"
    "fmt"
	"net/http"
    "time"

	"mikromon/internal/db"
    "mikromon/internal/ssher"
    "mikromon/internal/audit"

	"go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type Device struct {
    ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
    Name        string             `json:"name" bson:"name"`
    IP          string             `json:"ip" bson:"ip"`
    Type        string             `json:"type" bson:"type"` // OLT, ROUTER
    Username    string             `json:"username" bson:"username"` // Encrypt in prod
    Password    string             `json:"password" bson:"password"` // Encrypt in prod
    Port        int                `json:"port" bson:"port"`
}

type CommandRequest struct {
    DeviceID string `json:"device_id"`
    Command  string `json:"command"`
}

func GetDevicesHandler(w http.ResponseWriter, r *http.Request) {
    collection := db.GetCollection("devices")
    var devices []Device

    if collection == nil {
        // Mock Data
        devices = []Device{
            {ID: primitive.NewObjectID(), Name: "OLT-Huawei-Principal", IP: "192.168.88.254", Type: "OLT"},
            {ID: primitive.NewObjectID(), Name: "OLT-ZTE-Bairro-Norte", IP: "10.50.0.1", Type: "OLT"},
            {ID: primitive.NewObjectID(), Name: "Router-Mk-Main", IP: "10.0.0.1", Type: "ROUTER"},
            {ID: primitive.NewObjectID(), Name: "Router-Borda-IX", IP: "172.16.0.1", Type: "ROUTER"},
            {ID: primitive.NewObjectID(), Name: "SW-Core-Datacenter", IP: "10.0.0.2", Type: "SWITCH"},
            {ID: primitive.NewObjectID(), Name: "SW-Distrib-Torre-1", IP: "10.0.0.10", Type: "SWITCH"},
        }
    } else {
        cursor, err := collection.Find(context.TODO(), bson.M{})
        if err != nil {
            http.Error(w, "Error fetching devices", http.StatusInternalServerError)
            return
        }
        defer cursor.Close(context.TODO())
        if err = cursor.All(context.TODO(), &devices); err != nil {
            http.Error(w, "Error decoding devices", http.StatusInternalServerError)
            return
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(devices)
}

func AddDeviceHandler(w http.ResponseWriter, r *http.Request) {
    var device Device
    if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }

    collection := db.GetCollection("devices")
    _, err := collection.InsertOne(context.TODO(), device)
    if err != nil {
        http.Error(w, "Error adding device", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
}

func RunCommandHandler(w http.ResponseWriter, r *http.Request) {
    var req CommandRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }

    // 1. Get Device
    collection := db.GetCollection("devices")
    var device Device
    
    if collection == nil {
        // Mock Device Lookup
        device = Device{
            Name: "MOCK-OLT",
            IP: "192.168.88.99",
            Username: "admin",
            Password: "password",
            Port: 22,
        }
    } else {
        oid, _ := primitive.ObjectIDFromHex(req.DeviceID)
        err := collection.FindOne(context.TODO(), bson.M{"_id": oid}).Decode(&device)
        if err != nil {
            http.Error(w, "Device not found", http.StatusNotFound)
            return
        }
    }

    var output string
    var err error

    // 2. Execute SSH or Mock
    if collection == nil {
        // Mock Execution
        time.Sleep(1 * time.Second) // Simulate network lag
        output = fmt.Sprintf("MOCK OUTPUT from %s\n> %s\nResult: Success (Signal: -22dBm)", device.Name, req.Command)
    } else {
         client := ssher.NewSSHClient(device.Username, device.Password, device.IP, device.Port)
         output, err = client.RunCommand(req.Command)
    }
    
    // Log It
    // Using "admin" as placeholder until we extract from JWT middleware context
    audit.LogAction("admin", "run_command", device.Name + " (" + device.IP + ")", req.Command)
    
    if err != nil {
        http.Error(w, "Command failed: " + err.Error(), http.StatusInternalServerError)
        return
    }

    // 3. Return Output
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "output": output,
        "device": device.Name,
        "timestamp": time.Now().String(),
    })
}
