package api

import (
	"encoding/json"
	"net/http"
	"mikromon/internal/db"
	"context"
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
	ID         string `json:"id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Filename   string `json:"filename"`
	Size       string `json:"size"`
	CreatedAt  string `json:"created_at"`
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
	// In real app, fetch from DB or filesystem
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MockBackups)
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

	MockBackups = append([]Backup{newBackup}, MockBackups...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newBackup)
}
