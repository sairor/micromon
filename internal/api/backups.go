package api

import (
	"context"
	"encoding/json"
	"fmt"
	"mikromon/internal/db"
	"mikromon/internal/ssher"
	"net/http"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BackupRetention struct {
	BackupsPerDay int `json:"backups_per_day"` // Default hourly choice
	AfterWeek     int `json:"after_week"`      // Limit to N per day
	AfterMonth    int `json:"after_month"`     // Limit to N per day
	AfterYear     int `json:"after_year"`      // Limit to N per week
	AfterThree    int `json:"after_three"`     // Limit to N per month
}

type Backup struct {
	ID         string `json:"id" bson:"id"`
	DeviceID   string `json:"device_id" bson:"device_id"`
	DeviceName string `json:"device_name" bson:"device_name"`
	Filename   string `json:"filename" bson:"filename"`
	Size       string `json:"size" bson:"size"`
	CreatedAt  string `json:"created_at" bson:"created_at"`
	IsTest     bool   `json:"is_test" bson:"is_test"` // Flag for test backups
}

type BackupConfig struct {
	Enabled   bool            `json:"enabled"`
	Retention BackupRetention `json:"retention"`
}

var MockBackups = []Backup{
	{ID: "1", DeviceID: "d1", DeviceName: "OLT-Centro", Filename: "OLT-Centro_20260130_1200.backup", Size: "342 KB", CreatedAt: "2026-01-30 12:00"},
	{ID: "2", DeviceID: "d2", DeviceName: "Borda-Mikrotik", Filename: "Borda-Mikrotik_20260130_1145.backup", Size: "128 KB", CreatedAt: "2026-01-30 11:45"},
}

var MockBackupConfig = BackupConfig{
	Enabled: true,
	Retention: BackupRetention{
		BackupsPerDay: 4,
		AfterWeek:     3,
		AfterMonth:    1,
		AfterYear:     1,
		AfterThree:    2,
	},
}

func GetBackupConfigHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("backup_config")
	if collection == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MockBackupConfig)
		return
	}

	var config BackupConfig
	err := collection.FindOne(context.TODO(), bson.M{}).Decode(&config)
	if err != nil {
		// Return default if not found
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MockBackupConfig)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func UpdateBackupConfigHandler(w http.ResponseWriter, r *http.Request) {
	var config BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	collection := db.GetCollection("backup_config")
	if collection == nil {
		MockBackupConfig = config
	} else {
		// Simple upsert logic
		_, err := collection.UpdateOne(context.TODO(), bson.M{}, bson.M{"$set": config}, nil)
		if err != nil {
			http.Error(w, "Error saving config", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func GetBackupsHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("backups")
	var backups []Backup

	if collection == nil {
		backups = MockBackups
	} else {
		cursor, err := collection.Find(context.TODO(), bson.M{})
		if err == nil {
			cursor.All(context.TODO(), &backups)
		}
	}

	if backups == nil {
		backups = []Backup{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backups)
}

func ManualBackupHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Mock backup creation
	newBackup := Backup{
		ID:         primitive.NewObjectID().Hex(),
		DeviceID:   input.DeviceID,
		DeviceName: "Equipamento Manual", // In real app, fetch name from device ID
		Filename:   "Manual_Backup_" + primitive.NewObjectID().Hex()[:8] + ".backup",
		Size:       "256 KB",
		CreatedAt:  "Recentemente",
	}

	collection := db.GetCollection("backups")
	if collection == nil {
		MockBackups = append([]Backup{newBackup}, MockBackups...)
	} else {
		_, err := collection.InsertOne(context.TODO(), newBackup)
		if err != nil {
			http.Error(w, "Error saving backup", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newBackup)
}

func TestBackupHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Details string `json:"details,omitempty"`
		Command string `json:"command,omitempty"`
	}

	response.Command = "/system backup save name=test_connection"

	if input.DeviceID == "" {
		response.Success = false
		response.Message = "Erro: Equipamento não selecionado."
		json.NewEncoder(w).Encode(response)
		return
	}

	// 1. Cleanup old test backup for this device
	collection := db.GetCollection("backups")
	if collection != nil {
		collection.DeleteMany(context.TODO(), bson.M{"device_id": input.DeviceID, "is_test": true})
	} else {
		newMock := []Backup{}
		for _, b := range MockBackups {
			if b.DeviceID == input.DeviceID && b.IsTest {
				continue
			}
			newMock = append(newMock, b)
		}
		MockBackups = newMock
	}

	// 2. Perform Real or Mock Backup
	var output string
	var err error
	var device Device

	// Get Device Details
	devColl := db.GetCollection("devices")
	if devColl != nil {
		objID, _ := primitive.ObjectIDFromHex(input.DeviceID)
		err = devColl.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&device)
	} else {
		// Mock logic fallback
		if input.DeviceID == "fail" {
			err = fmt.Errorf("forced mock failure")
		} else {
			device = Device{Name: "Mock Device", IP: "127.0.0.1", Username: "admin", Port: 22}
		}
	}

	if err == nil {
		if device.Port == 0 {
			device.Port = 22
		}

		pool := ssher.GetPool()
		// A. Execute Backup Command
		output, err = pool.RunCommand(device.Username, device.Password, device.IP, device.Port, device.UseSSHKey, response.Command)

		if err == nil {
			// B. Download the file to local server
			localPath := filepath.Join("data", "backups", input.DeviceID, "test_connection.backup")
			// MikroTik saves to root usually or specific path if provided.
			err = pool.DownloadFile(device.Username, device.Password, device.IP, device.Port, device.UseSSHKey, "test_connection.backup", localPath)
			if err != nil {
				output += "\n[Warning] command success but file download failed: " + err.Error()
			} else {
				output += "\n[Success] File downloaded to server successfully."
			}
		}
	}

	if err != nil {
		response.Success = false
		response.Message = "Falha no diagnóstico SSH/SFTP."
		response.Details = fmt.Sprintf("Erro: %v\nSaída parcial: %s", err, output)
	} else {
		response.Success = true
		response.Message = "Sucesso: Diagnóstico concluído e arquivo baixado."
		response.Details = "Conexão SSH/SFTP OK.\nO arquivo 'test_connection.backup' foi baixado para o armazenamento local do servidor."

		// 3. Register the new Test Backup
		newTestBackup := Backup{
			ID:         primitive.NewObjectID().Hex(),
			DeviceID:   input.DeviceID,
			DeviceName: device.Name,
			Filename:   "test_connection.backup",
			Size:       "Calculando...",
			CreatedAt:  time.Now().Format("2006-01-02 15:04"),
			IsTest:     true,
		}

		if collection != nil {
			collection.InsertOne(context.TODO(), newTestBackup)
		} else {
			MockBackups = append([]Backup{newTestBackup}, MockBackups...)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	collection := db.GetCollection("backups")
	if collection == nil {
		// Mock Delete
		for i, b := range MockBackups {
			if b.ID == id {
				MockBackups = append(MockBackups[:i], MockBackups[i+1:]...)
				break
			}
		}
	} else {
		// MongoDB Delete
		_, err := collection.DeleteOne(context.TODO(), bson.M{"id": id})
		if err != nil {
			http.Error(w, "Error deleting backup", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
