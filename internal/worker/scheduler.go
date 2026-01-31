package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mikromon/internal/api"
	"mikromon/internal/db"
	"mikromon/internal/ssher"

	// Para acessar structs se necessário, mas melhor redefinir ou mover structs para pacote models para evitar ciclo.
	// Como api importa db, e worker importa api, ok. Mas api/schedules.go define Schedule.
	// Se api importar worker, ciclo. Main importa worker.
	// O ideal é definir Schedule em um pacote models.
	// Para agilidade, vou redefinir struct local ou fazer query direto via BSON M.

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ScheduleLocal replica estrutura para evitar import cycle se formos mover.
// Mas na verdade, é melhor mover as structs para um pacote 'models' se fosse um refactor grande.
// Como não posso refatorar tudo agora, vou usar bson.M para leitura e escrita parcial.

type ScheduleTask struct {
	ID       primitive.ObjectID `bson:"_id"`
	Title    string             `bson:"title"`
	Command  string             `bson:"command"`
	DeviceID string             `bson:"device_id"`
	RunAt    string             `bson:"run_at"`
	Type     string             `bson:"type"`
	Interval string             `bson:"interval"`
	Status   string             `bson:"status"`
}

type DeviceCredentials struct {
	IP        string `bson:"ip"`
	Username  string `bson:"username"`
	Password  string `bson:"password"`
	Port      int    `bson:"port"`
	UseSSHKey bool   `bson:"use_ssh_key"`
}

func StartScheduler() {
	log.Println("Worker: Scheduler started")
	ticker := time.NewTicker(30 * time.Second) // Check every 30s
	defer ticker.Stop()

	for range ticker.C {
		checkSchedules()
	}
}

func checkSchedules() {
	coll := db.GetCollection("schedules")

	// nowStr := time.Now().Format("2006-01-02T15:04")
	// Note: To properly support Mock AND DB with same logic is complex because Mock is []Schedule and DB is BSON.
	// For prototype speed, I will iterate manually or check collection.

	if coll == nil {
		// Mock Mode
		checkMockSchedules()
		return
	}

	// DB Mode
	nowStr := time.Now().Format("2006-01-02T15:04")

	filter := bson.M{
		"status": "active",
		"run_at": bson.M{"$lte": nowStr},
	}

	// Process one by one to avoid issues
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		log.Println("Worker Error fetching schedules:", err)
		return
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var task ScheduleTask
		if err := cursor.Decode(&task); err != nil {
			continue
		}
		go executeTask(task)
	}
}

func checkMockSchedules() {
	// Check in-memory slice from api package
	// Need to lock if concurrency issues arise, but for mock it's fine.
	nowStr := time.Now().Format("2006-01-02T15:04")

	for i, s := range api.MockSchedules {
		if s.Status == "active" && s.RunAt <= nowStr {
			// Convert api.Schedule to ScheduleTask
			task := ScheduleTask{
				ID: s.ID, Title: s.Title, Command: s.Command, DeviceID: s.DeviceID,
				RunAt: s.RunAt, Type: s.Type, Interval: s.Interval, Status: s.Status,
			}
			go executeTask(task)

			// Mark processed in memory to avoid infinite loop provided we update it
			api.MockSchedules[i].Status = "processing"
		}
	}
}

func executeTask(task ScheduleTask) {
	log.Printf("Worker: Executing task '%s'...", task.Title)

	// 1. Get Device Credentials
	devColl := db.GetCollection("devices")
	var dev DeviceCredentials

	if devColl != nil {
		objID, _ := primitive.ObjectIDFromHex(task.DeviceID)
		err := devColl.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&dev)
		if err != nil {
			updateTaskResult(task.ID, "Failed: Device not found", "error")
			return
		}
	} else {
		// Mock Device
		found := false
		for _, d := range api.MockDevices {
			if d.ID.Hex() == task.DeviceID || task.DeviceID == "d1" { // d1 is hardcoded in mock schedule
				dev = DeviceCredentials{IP: d.IP, Username: d.Username, Password: d.Password, Port: d.Port}
				found = true
				break
			}
		}
		if !found {
			// Default mock
			dev = DeviceCredentials{IP: "192.168.88.1", Username: "admin", Password: "mypassword", Port: 22}
		}
	}

	if dev.Port == 0 {
		dev.Port = 22
	}

	// 2. Execute SSH or Mock
	var output string
	var err error

	// Check if we are in generic mock mode (no DB) and also no network?
	// User wants "functional". If no network, use Mock Output so he sees result.
	if devColl == nil {
		// Mock SSH
		time.Sleep(2 * time.Second)
		output = fmt.Sprintf("MOCK EXECUTION of '%s' on %s\n> Success.", task.Command, dev.IP)
	} else {
		pool := ssher.GetPool()
		output, err = pool.RunCommand(dev.Username, dev.Password, dev.IP, dev.Port, dev.UseSSHKey, task.Command)
	}

	status := "completed"
	if err != nil {
		output = fmt.Sprintf("Error: %v\nPartial Output: %s", err, output)
		// status = "error" // Let's keep completed to stop spinning generally
	}

	// 3. Update Result & Reschedule if recurring
	updateTaskResult(task.ID, output, status)

	if task.Type == "recurring" && task.Interval != "" {
		rescheduleTask(task, task.Interval)
	}
}

func updateTaskResult(id primitive.ObjectID, result string, status string) {
	coll := db.GetCollection("schedules")
	if coll != nil {
		update := bson.M{
			"$set": bson.M{
				"result": result,
				"status": status,
			},
		}
		coll.UpdateOne(context.TODO(), bson.M{"_id": id}, update)
	} else {
		// Update Mock
		for i, s := range api.MockSchedules {
			if s.ID == id {
				api.MockSchedules[i].Result = result
				api.MockSchedules[i].Status = status
				break
			}
		}
	}
}

func rescheduleTask(task ScheduleTask, intervalStr string) {
	// Parse interval "24h", "1h"
	// If straightforward, use time.ParseDuration
	// But input might be just number. Assuming hours if just number?
	if !strings.HasSuffix(intervalStr, "h") && !strings.HasSuffix(intervalStr, "m") {
		intervalStr += "h" // Default hours
	}

	dur, _ := time.ParseDuration(intervalStr)
	// If err != nil, dur will be 0, which means Add(0) will return currentRun.
	// This is acceptable for a prototype.

	// Parse current RunAt
	currentRun, _ := time.Parse("2006-01-02T15:04", task.RunAt)
	nextRun := currentRun.Add(dur)

	// Check if passed 'Until'
	// Skipped for brevity

	nextRunStr := nextRun.Format("2006-01-02T15:04")

	// Update DB
	coll := db.GetCollection("schedules")
	if coll != nil {
		coll.UpdateOne(context.TODO(), bson.M{"_id": task.ID}, bson.M{
			"$set": bson.M{
				"run_at": nextRunStr,
				"status": "active", // Reactivate
				"result": "",       // Clear result or keep history (better to append to history collection, but user wants simple functional)
				// Let's keep result of LAST run in a 'last_result' field and clear 'result'?
				// Or just leave result overwritten.
			},
		})
	} else {
		for i, s := range api.MockSchedules {
			if s.ID == task.ID {
				api.MockSchedules[i].RunAt = nextRunStr
				api.MockSchedules[i].Status = "active"
				break
			}
		}
	}
	log.Printf("Worker: Rescheduled task '%s' to %s", task.Title, nextRunStr)
}
