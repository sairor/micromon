package api

import (
    "encoding/json"
    "net/http"
    "sort"
    
    // "mikromon/internal/db"
)

type SignalMetric struct {
    DeviceName string  `json:"device_name"`
    OnuSerial  string  `json:"onu_serial"`
    RxPower    float64 `json:"rx_power"` // dBm
    Port       string  `json:"port"`
}

// GetTopCriticalSignalsHandler returns the APIs with worst RX power
func GetTopCriticalSignalsHandler(w http.ResponseWriter, r *http.Request) {
    // In a real app, this would query a "telemetry" collection in MongoDB
    // For this prototype, we will return mock data or calculate from a small memory set
    
    mockData := []SignalMetric{
        {"OLT-Main", "HWTC123456", -28.5, "PON 0/1"},
        {"OLT-Main", "HWTC999999", -31.2, "PON 0/2"},
        {"OLT-Backup", "ZTE11111", -26.0, "PON 1/1"},
        {"OLT-Main", "HWTC777777", -18.0, "PON 0/1"}, // Good signal
    }
    
    // Filter for critical (e.g., < -27) and Sort
    var critical []SignalMetric
    for _, m := range mockData {
        if m.RxPower < -27.0 {
            critical = append(critical, m)
        }
    }
    
    // Sort ascending (worst signal first, e.g. -31 is worse than -28)
    sort.Slice(critical, func(i, j int) bool {
        return critical[i].RxPower < critical[j].RxPower
    })
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(critical)
}
