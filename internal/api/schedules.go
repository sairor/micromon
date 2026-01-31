package api

import (
	"encoding/json"
	"net/http"
	"mikromon/internal/db"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Schedule struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Title        string             `json:"title" bson:"title"`
	Command      string             `json:"command" bson:"command"`
	DeviceID     string             `json:"device_id" bson:"device_id"`
	DeviceName   string             `json:"device_name" bson:"device_name"`
	RunAt        string             `json:"run_at" bson:"run_at"` // ISO Date/Time
	Type         string             `json:"type" bson:"type"`     // single, recurring
	Interval     string             `json:"interval" bson:"interval,omitempty"` // e.g. "1h", "24h"
	Until        string             `json:"until" bson:"until,omitempty"`       // ISO Date/Time
	Status       string             `json:"status" bson:"status"`               // pending, active, completed
	Result       string             `json:"result" bson:"result"`               // Captured output
}

var MockSchedules = []Schedule{
	{
		ID:         primitive.NewObjectID(),
		Title:      "Verificar Interfaces",
		Command:    "/interface print",
		DeviceID:   "d1",
		DeviceName: "Borda-Mikrotik",
		RunAt:      "2026-01-30T10:00:00",
		Type:       "single",
		Status:     "completed",
		Result:     "Flags: D - dynamic, X - disabled, R - running, S - slave \n #     NAME                                TYPE       ACTUAL-MTU L2MTU  MAX-L2MTU\n 0  R  ether1                              ether            1500  1592       4064\n 1  RS ether2                              ether            1500  1592       4064\n 2  RS ether3                              ether            1500  1592       4064",
	},
}

func GetSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("schedules")
	var schedules []Schedule

	if collection == nil {
		schedules = MockSchedules
	} else {
		cursor, err := collection.Find(context.TODO(), bson.M{})
		if err == nil {
			cursor.All(context.TODO(), &schedules)
		}
	}

	if schedules == nil {
		schedules = []Schedule{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

func CreateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	var s Schedule
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	s.ID = primitive.NewObjectID()
	s.Status = "active"

	collection := db.GetCollection("schedules")
	if collection == nil {
		MockSchedules = append(MockSchedules, s)
	} else {
		_, err := collection.InsertOne(context.TODO(), s)
		if err != nil {
			http.Error(w, "Error saving schedule", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

func DeleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := primitive.ObjectIDFromHex(idStr)

	collection := db.GetCollection("schedules")
	if collection == nil {
		for i, s := range MockSchedules {
			if s.ID == id {
				MockSchedules = append(MockSchedules[:i], MockSchedules[i+1:]...)
				break
			}
		}
	} else {
		collection.DeleteOne(context.TODO(), bson.M{"_id": id})
	}

	w.WriteHeader(http.StatusOK)
}
