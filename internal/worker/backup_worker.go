package worker

import (
	"context"
	"fmt"
	"log"
	"mikromon/internal/api"
	"mikromon/internal/db"
	"mikromon/internal/ssher"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func StartBackupWorker() {
	log.Println("Worker: Backup Automation started")

	// Check every 5 minutes to see if any device needs a backup
	// (either it never had one or the last one was > 1 hour ago)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run once at start
	go runBackupCheck()

	for range ticker.C {
		go runBackupCheck()
	}
}

func runBackupCheck() {
	log.Println("Worker: Running backup check...")
	devices := getAllDevices()

	for _, dev := range devices {
		lastBackupTime := getLastBackupTime(dev.ID.Hex())

		log.Printf("[DEBUG] Worker checking device: %s (%s) with user: %s", dev.Name, dev.IP, dev.Username)

		// If never backed up OR last backup was more than 55 minutes ago
		if lastBackupTime.IsZero() || time.Since(lastBackupTime) > 55*time.Minute {
			log.Printf("Worker: Triggering backup for %s (Last run: %v)", dev.Name, lastBackupTime)
			err := executeBackupForDevice(dev)
			if err != nil {
				log.Printf("Worker: Failed backup for %s: %v", dev.Name, err)
			}
		}
	}

	log.Println("Worker: Starting retention cleanup...")
	applyRetentionPolicy()
}

func getLastBackupTime(deviceID string) time.Time {
	coll := db.GetCollection("backups")
	var backup api.Backup

	if coll != nil {
		// Find latest backup for this device
		opts := options.FindOne().SetSort(bson.M{"created_at": -1})
		err := coll.FindOne(context.TODO(), bson.M{"device_id": deviceID}, opts).Decode(&backup)
		if err != nil {
			return time.Time{}
		}
	} else {
		// Mock check
		for _, b := range api.MockBackups {
			if b.DeviceID == deviceID {
				t, _ := time.Parse("2006-01-02 15:04", b.CreatedAt)
				return t // Should ideally check all and find max, but for mock first is usually latest
			}
		}
		return time.Time{}
	}

	t, _ := time.Parse("2006-01-02 15:04", backup.CreatedAt)
	return t
}

type BackupDevice struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	IP        string             `bson:"ip"`
	Username  string             `bson:"username"`
	Password  string             `bson:"password"`
	Port      int                `bson:"port"`
	Type      string             `bson:"type"`
	UseSSHKey bool               `bson:"use_ssh_key"`
}

func getAllDevices() []BackupDevice {
	coll := db.GetCollection("devices")
	var devices []BackupDevice
	if coll != nil {
		cursor, err := coll.Find(context.TODO(), bson.M{})
		if err == nil {
			cursor.All(context.TODO(), &devices)
		}
	} else {
		// Mock devices if no DB
		for _, d := range api.MockDevices {
			devices = append(devices, BackupDevice{
				ID: d.ID, Name: d.Name, IP: d.IP, Username: d.Username, Password: d.Password, Port: d.Port, Type: d.Type,
			})
		}
	}
	return devices
}

func executeBackupForDevice(dev BackupDevice) error {
	if dev.Port == 0 {
		dev.Port = 22
	}

	timestamp := time.Now().Format("20060102_1504")
	filename := fmt.Sprintf("mikromon_auto_%s.backup", timestamp)

	// Command for MikroTik. For OLT or Switch it might differ, but assuming MikroTik as per user request context.
	cmd := fmt.Sprintf("/system backup save name=%s", filename)

	var err error
	if db.GetCollection("devices") != nil {
		pool := ssher.GetPool()
		_, err = pool.RunCommand(dev.Username, dev.Password, dev.IP, dev.Port, dev.UseSSHKey, cmd)
	} else {
		// Mock success
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return err
	}

	// Register backup in DB
	backup := api.Backup{
		ID:         primitive.NewObjectID().Hex(),
		DeviceID:   dev.ID.Hex(),
		DeviceName: dev.Name,
		Filename:   filename,
		Size:       "Calculando...",
		CreatedAt:  time.Now().Format("2006-01-02 15:04"),
	}

	coll := db.GetCollection("backups")
	if coll != nil {
		coll.InsertOne(context.TODO(), backup)
	} else {
		api.MockBackups = append([]api.Backup{backup}, api.MockBackups...)
	}

	return nil
}

func applyRetentionPolicy() {
	coll := db.GetCollection("backups")
	var backups []api.Backup

	if coll != nil {
		cursor, err := coll.Find(context.TODO(), bson.M{})
		if err == nil {
			cursor.All(context.TODO(), &backups)
		}
	} else {
		backups = api.MockBackups
	}

	now := time.Now()
	for _, b := range backups {
		created, err := time.Parse("2006-01-02 15:04", b.CreatedAt)
		if err != nil {
			continue
		}

		age := now.Sub(created)
		keep := true

		// 1 month = 30 days
		// 3 months = 90 days
		// 1 year = 365 days
		// 3 years = 1095 days

		days := age.Hours() / 24

		if days <= 30 {
			// keep everything
			keep = true
		} else if days <= 90 {
			// keep 12/day (bi-hourly)
			keep = (created.Hour()%2 == 0)
		} else if days <= 365 {
			// keep 6/day (every 4h)
			keep = (created.Hour()%4 == 0)
		} else if days <= 1095 {
			// keep 1/day
			keep = (created.Hour() == 0)
		} else {
			// keep 1/week (e.g. Sunday)
			keep = (created.Hour() == 0 && created.Weekday() == time.Sunday)
		}

		if !keep {
			deleteBackup(b.ID)
		}
	}
}

func deleteBackup(id string) {
	coll := db.GetCollection("backups")
	if coll != nil {
		coll.DeleteOne(context.TODO(), bson.M{"id": id})
	} else {
		for i, b := range api.MockBackups {
			if b.ID == id {
				api.MockBackups = append(api.MockBackups[:i], api.MockBackups[i+1:]...)
				break
			}
		}
	}
}
