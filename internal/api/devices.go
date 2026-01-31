package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mikromon/internal/audit"
	"mikromon/internal/db"
	"mikromon/internal/persistence"
	"mikromon/internal/ssher"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Device struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name"`
	IP        string             `json:"ip" bson:"ip"`
	Type      string             `json:"type" bson:"type"`         // OLT, ROUTER
	Username  string             `json:"username" bson:"username"` // Encrypt in prod
	Password  string             `json:"password" bson:"password"` // Encrypt in prod
	Port      int                `json:"port" bson:"port"`
	Owner     string             `json:"owner" bson:"owner"` // Username of the owner
	UseSSHKey bool               `json:"use_ssh_key" bson:"use_ssh_key"`
}

type CommandRequest struct {
	DeviceID string `json:"device_id"`
	Command  string `json:"command"`
}

var MockDevices []Device // Changed to slice, not pre-populated

func init() {
	// Attempt to load from disk on startup
	store := persistence.GetStore()
	err := store.Load(persistence.DevicesFile, &MockDevices)
	if err != nil && os.IsNotExist(err) {
		// Fallback defaults if no file
		MockDevices = []Device{
			{ID: primitive.NewObjectID(), Name: "OLT-Huawei-Principal", IP: "192.168.88.254", Type: "OLT", Owner: "admin"},
			{ID: primitive.NewObjectID(), Name: "OLT-ZTE-Bairro-Norte", IP: "10.50.0.1", Type: "OLT", Owner: "admin"},
			{ID: primitive.NewObjectID(), Name: "Router-Mk-Main", IP: "10.0.0.1", Type: "ROUTER", Owner: "admin"},
		}
		store.Save(persistence.DevicesFile, MockDevices)
	}
}

func GetDevicesHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("devices")
	var devices []Device

	// Extract user from context (set by JwtMiddleware)
	username := "admin" // Default fallback
	if u := r.Context().Value("username"); u != nil {
		username = u.(string)
	}

	if collection == nil {
		// Mock Data (Persistent)
		// Reload in case another process changed it? No, in-mem is single source of truth for this instance.

		// Filter by owner
		for _, d := range MockDevices {
			if d.Owner == username {
				devices = append(devices, d)
			}
		}
	} else {
		// Filter by Owner in DB
		filter := bson.M{"owner": username}
		cursor, err := collection.Find(context.TODO(), filter)
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

	if devices == nil {
		devices = []Device{}
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

	// Assign Owner
	username := "admin"
	if u := r.Context().Value("username"); u != nil {
		username = u.(string)
	}
	device.Owner = username

	collection := db.GetCollection("devices")

	if collection == nil {
		// Mock Add & Persist
		device.ID = primitive.NewObjectID()
		MockDevices = append(MockDevices, device)

		// Save to Disk
		persistence.GetStore().Save(persistence.DevicesFile, MockDevices)

		fmt.Printf("MOCK: Added device %s (%s) for %s to JSON\n", device.Name, device.Type, device.Owner)
	} else {
		_, err := collection.InsertOne(context.TODO(), device)
		if err != nil {
			http.Error(w, "Error adding device", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

func DeleteDeviceHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	username := "admin"
	if u := r.Context().Value("username"); u != nil {
		username = u.(string)
	}

	collection := db.GetCollection("devices")

	if collection == nil {
		// Mock Delete
		newMock := []Device{}
		found := false
		for _, d := range MockDevices {
			if d.ID.Hex() == idStr && d.Owner == username {
				found = true
				continue // Skip (Delete)
			}
			newMock = append(newMock, d)
		}
		MockDevices = newMock
		persistence.GetStore().Save(persistence.DevicesFile, MockDevices)

		if !found {
			http.Error(w, "Device not found or permission denied", http.StatusForbidden)
			return
		}
	} else {
		objID, _ := primitive.ObjectIDFromHex(idStr)
		// Ensure we delete ONLY if it belongs to the user
		res, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objID, "owner": username})
		if err != nil {
			http.Error(w, "Error deleting device", http.StatusInternalServerError)
			return
		}
		if res.DeletedCount == 0 {
			http.Error(w, "Device not found or permission denied", http.StatusForbidden)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
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

	// TODO: Verify Owner ownership logic here too for security
	// For now, assuming if ID is valid we proceed, but correctly we should check owner.

	if collection == nil {
		// Mock Device Lookup
		device = Device{
			Name:     "MOCK-OLT",
			IP:       "192.168.88.99",
			Username: "admin",
			Password: "password",
			Port:     22,
		}
		// In full mock, search MockDevices
		for _, d := range MockDevices {
			if d.ID.Hex() == req.DeviceID {
				device = d
				break
			}
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
		if device.Port == 0 {
			device.Port = 22
		}
		pool := ssher.GetPool()
		output, err = pool.RunCommand(device.Username, device.Password, device.IP, device.Port, device.UseSSHKey, req.Command)
	}

	// Log It
	// Using "admin" as placeholder until we extract from JWT middleware context
	audit.LogAction("admin", "run_command", device.Name+" ("+device.IP+")", req.Command)

	if err != nil {
		http.Error(w, "Command failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Return Output
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"output":    output,
		"device":    device.Name,
		"timestamp": time.Now().String(),
	})
}
