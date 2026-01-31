package api

import (
	"context"
	"encoding/json"
	"net/http"

	// "sort"

	"mikromon/internal/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SignalMetric struct {
	DeviceName string  `json:"device_name" bson:"device_name"`
	OnuSerial  string  `json:"onu_serial" bson:"onu_serial"`
	RxPower    float64 `json:"rx_power" bson:"rx_power"` // dBm
	Port       string  `json:"port" bson:"port"`
	Timestamp  int64   `json:"timestamp" bson:"timestamp"`
}

// GetTopCriticalSignalsHandler returns the APIs with worst RX power
func GetTopCriticalSignalsHandler(w http.ResponseWriter, r *http.Request) {
	collection := db.GetCollection("historico_sinal")
	var critical []SignalMetric

	if collection == nil {
		// Mock Fallback
		critical = []SignalMetric{
			{DeviceName: "OLT-Main", OnuSerial: "HWTC123456", RxPower: -28.5, Port: "PON 0/1"},
			{DeviceName: "OLT-Main", OnuSerial: "HWTC999999", RxPower: -31.2, Port: "PON 0/2"},
			{DeviceName: "OLT-Backup", OnuSerial: "ZTE11111", RxPower: -26.0, Port: "PON 1/1"},
		}
	} else {
		// Find signals where RxPower < -27 (warning threshold)
		// Sort by RxPower ascending (lower is worse, e.g. -30 < -20)
		// Limit 10
		opts := options.Find().SetSort(bson.D{{Key: "rx_power", Value: 1}}).SetLimit(10)
		filter := bson.M{"rx_power": bson.M{"$lt": -27.0}}

		cursor, err := collection.Find(context.TODO(), filter, opts)
		if err != nil {
			http.Error(w, "Error fetching signals", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		if err = cursor.All(context.TODO(), &critical); err != nil {
			http.Error(w, "Error decoding signals", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(critical)
}
