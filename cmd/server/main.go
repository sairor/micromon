package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"mikromon/internal/db"
	"mikromon/internal/auth"
    "mikromon/internal/api"

	"github.com/gorilla/mux"
    "github.com/gorilla/handlers"
)

func main() {
	// Configuration
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Initialize Database
	if err := db.InitMongoDB(mongoURI); err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}

	// Router Setup
	r := mux.NewRouter()

	// Middleware
	r.Use(loggingMiddleware)
    // CORS is handled by gorilla/handlers wrapper below

	// API Routes
	apiRouter := r.PathPrefix("/api").Subrouter()
	
    // Public Routes
    apiRouter.HandleFunc("/login", auth.LoginHandler).Methods("POST")

    // Protected Routes
    protected := apiRouter.PathPrefix("/v1").Subrouter()
    protected.Use(auth.JwtMiddleware)
    protected.HandleFunc("/me", auth.MeHandler).Methods("GET")
    protected.HandleFunc("/me/change-password", auth.ChangePasswordHandler).Methods("POST")
    
    // Devices
    protected.HandleFunc("/devices", api.GetDevicesHandler).Methods("GET")
    protected.HandleFunc("/devices", api.AddDeviceHandler).Methods("POST")
    protected.HandleFunc("/devices/command", api.RunCommandHandler).Methods("POST")

    // Custom Commands
    protected.HandleFunc("/commands", api.GetCustomCommandsHandler).Methods("GET")
    protected.HandleFunc("/commands", api.CreateCustomCommandHandler).Methods("POST")

    // Maintenance
    protected.HandleFunc("/maintenance/critical-signals", api.GetTopCriticalSignalsHandler).Methods("GET")

    // Provisioning
    protected.HandleFunc("/provision/script", api.ProvisionScriptHandler).Methods("POST")

    // Admin - Users
    protected.HandleFunc("/users", api.GetUsersHandler).Methods("GET")
    protected.HandleFunc("/users", api.CreateUserHandler).Methods("POST")

    // Tech - PON & ONUs
    protected.HandleFunc("/pon/{id}/status", api.GetPonStatusHandler).Methods("GET")
    protected.HandleFunc("/onus/unregistered", api.GetUnregisteredOnusHandler).Methods("GET")
    protected.HandleFunc("/onus/install", api.InstallOnuHandler).Methods("POST")

    // Backups
    protected.HandleFunc("/backups/config", api.GetBackupConfigHandler).Methods("GET")
    protected.HandleFunc("/backups/config", api.UpdateBackupConfigHandler).Methods("POST")
    protected.HandleFunc("/backups", api.GetBackupsHandler).Methods("GET")
    protected.HandleFunc("/backups/manual", api.ManualBackupHandler).Methods("POST")

    // Schedules
    protected.HandleFunc("/schedules", api.GetSchedulesHandler).Methods("GET")
    protected.HandleFunc("/schedules", api.CreateScheduleHandler).Methods("POST")
    protected.HandleFunc("/schedules", api.DeleteScheduleHandler).Methods("DELETE")

    // Syslog
    protected.HandleFunc("/logs", api.GetLogsHandler).Methods("GET")

	// Serve Static Files (SPA)
	// Assuming running from project root (/var/www/html/mikromon)
	spa := spaHandler{staticPath: "web", indexPath: "index.html"}
	r.PathPrefix("/").Handler(spa)

    // CORS Headers
    headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
    originsOk := handlers.AllowedOrigins([]string{"*"}) // Restrict in production
    methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})

    srv := &http.Server{
        Handler: handlers.CORS(originsOk, headersOk, methodsOk)(r),
        Addr:    "0.0.0.0:8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

	log.Println("Micromon Server starting on :8080")
    
    // Start Syslog Server (UDP 514)
    go api.StartSyslogServer()

	log.Fatal(srv.ListenAndServe())
}

// spaHandler serves the SPA
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Default to index.html for SPA (catch-all)
    // If request has extension, try to serve file directly
    
    // Hardcoded relative path for prototype simplicity: "web/index.html"
    // This assumes CWD is /var/www/html/mikromon
    path := "web/index.html"
    
    // Verify file exists
    if _, err := os.Stat(path); os.IsNotExist(err) {
        log.Printf("ERROR: Static file not found at: %s (CWD: %s)", path, getCwd())
        http.Error(w, "Static file not found. Check server logs.", http.StatusNotFound)
        return
    }
    
    http.ServeFile(w, r, path)
}

func getCwd() string {
    d, _ := os.Getwd()
    return d
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
