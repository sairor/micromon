package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

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

// OltStats represents density and usage of all PON ports
type OltStats struct {
	OltName      string      `json:"olt_name"`
	TotalPorts   int         `json:"total_ports"`
	UsedPorts    int         `json:"used_ports"`
	TotalClients int         `json:"total_clients"`
	PortDensity  []PonStatus `json:"port_density"`
}

func GetOltStatsHandler(w http.ResponseWriter, r *http.Request) {
	// In a real scenario, this would aggregate data from all OLTs in DB
	// For now, return mock data request by user

	ports := []PonStatus{}
	totalClients := 0

	// Simulate 8 PON ports
	for i := 0; i < 8; i++ {
		connected := rand.Intn(100)
		totalClients += connected
		ports = append(ports, PonStatus{
			Port:       fmt.Sprintf("0/1/%d", i),
			Connected:  connected,
			TotalSlots: 128,
			AvgSignal:  -18.5 - rand.Float64()*5,
		})
	}

	stats := OltStats{
		OltName:      "Huawei-OLT-Core",
		TotalPorts:   16, // Physical ports
		UsedPorts:    8,  // Configured
		TotalClients: totalClients,
		PortDensity:  ports,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func GetPonStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	port := vars["id"] // Just echoing for now

	// Mock Data Logic
	// rand.Seed is deprecated since Go 1.20 but okay for legacy/simple
	// We can use NewSource if strict, but let's keep simple
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Body", http.StatusBadRequest)
		return
	}

	// Mock Installation Logic
	// fmt.Printf("Installing ONU %s for User %s\n", req.SerialNumber, req.UserRef)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
