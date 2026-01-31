package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type LogEntry struct {
	ID        int       `json:"id"`
	DeviceIP  string    `json:"device_ip"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
}

var (
	Logs      []LogEntry
	LogMutex  sync.Mutex
	logCounter int
)

func StartSyslogServer() {
	addr := net.UDPAddr{
		Port: 1514,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Printf("ERROR starting Syslog Server: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("Syslog Server listening on UDP 514")

	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		message := string(buf[:n])
		processLog(remoteAddr.IP.String(), message)
	}
}

func processLog(ip string, message string) {
	LogMutex.Lock()
	defer LogMutex.Unlock()

	logCounter++
	entry := LogEntry{
		ID:        logCounter,
		DeviceIP:  ip,
		Timestamp: time.Now(),
		Message:   message,
		Level:     "info", // Default, could be parsed from syslog header
	}

	// Keep last 1000 logs in memory for now
	Logs = append([]LogEntry{entry}, Logs...)
	if len(Logs) > 1000 {
		Logs = Logs[:1000]
	}
}

func GetLogsHandler(w http.ResponseWriter, r *http.Request) {
	LogMutex.Lock()
	logs := Logs
	if logs == nil {
		logs = []LogEntry{}
	}
	LogMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
