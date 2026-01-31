package api

import (
    "encoding/json"
    "net/http"
    "math/rand"
    "fmt"
    "time"
    "github.com/gorilla/mux"
)

type PonStatus struct {
    Port       string  `json:"port"` // 0/1/0
    Connected  int     `json:"connected_clients"`
    TotalSlots int     `json:"total_slots"`
    AvgSignal  float64 `json:"avg_signal"`
}

type UnregisteredOnu struct {
    SerialNumber string  `json:"serial_number"`
    PonPort      string  `json:"pon_port"`
    Signal       float64 `json:"signal_dbm"`
}

func GetPonStatusHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    port := vars["id"] // Just echoing for now
    
    // Mock Data Logic
    rand.Seed(time.Now().UnixNano())
    connected := rand.Intn(64)
    
    status := PonStatus{
        Port:       port,
        Connected:  connected,
        TotalSlots: 128,
        AvgSignal:  -15.0 - rand.Float64()*10, // -15 to -25
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

func GetUnregisteredOnusHandler(w http.ResponseWriter, r *http.Request) {
    // Mock Discovery
    onus := []UnregisteredOnu{
        {SerialNumber: "HWTC1A2B3C4D", PonPort: "0/1/2", Signal: -22.4},
        {SerialNumber: "ZTEG98765432", PonPort: "0/1/4", Signal: -19.8},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(onus)
}

func InstallOnuHandler(w http.ResponseWriter, r *http.Request) {
    var req struct {
        SerialNumber string `json:"serial_number"`
        UserRef      string `json:"user_ref"` // Customer ID
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Mock Installation Logic
    fmt.Printf("Installing ONU %s for User %s\n", req.SerialNumber, req.UserRef)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
