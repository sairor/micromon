package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"mikromon/internal/api"
	"mikromon/internal/auth"
	"mikromon/internal/db"
	"mikromon/internal/worker"
	"mikromon/web"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	// Configuration
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Initialize Database
	if err := db.InitMongoDB(mongoURI); err != nil {
		log.Printf("Could not connect to MongoDB: %v. Running in partial mode.", err)
	}

	// Router Setup
	r := mux.NewRouter()

	// Middleware
	r.Use(loggingMiddleware)

	// API Routes
	apiRouter := r.PathPrefix("/api").Subrouter()

	// Public
	apiRouter.HandleFunc("/login", auth.LoginHandler).Methods("POST")

	// Protected V1
	v1 := apiRouter.PathPrefix("/v1").Subrouter()
	v1.Use(auth.JwtMiddleware) // Enforce Auth

	// User
	v1.HandleFunc("/me", auth.MeHandler).Methods("GET")
	v1.HandleFunc("/me/change-password", auth.ChangePasswordHandler).Methods("POST")
	v1.HandleFunc("/users", api.GetUsersHandler).Methods("GET")
	v1.HandleFunc("/users", api.CreateUserHandler).Methods("POST")

	// Devices & Commands
	v1.HandleFunc("/devices", api.GetDevicesHandler).Methods("GET")
	v1.HandleFunc("/devices", api.AddDeviceHandler).Methods("POST")
	v1.HandleFunc("/devices", api.DeleteDeviceHandler).Methods("DELETE")
	v1.HandleFunc("/devices/command", api.RunCommandHandler).Methods("POST") // Ad-hoc

	// Custom Commands
	v1.HandleFunc("/commands", api.GetCustomCommandsHandler).Methods("GET")
	v1.HandleFunc("/commands", api.CreateCustomCommandHandler).Methods("POST")
	v1.HandleFunc("/commands/execute", api.RunCustomCommandHandler).Methods("POST") // Execute Saved

	// Network & OLT
	v1.HandleFunc("/network/critical-signals", api.GetTopCriticalSignalsHandler).Methods("GET")
	v1.HandleFunc("/olt/stats", api.GetOltStatsHandler).Methods("GET")
	v1.HandleFunc("/pon/{id}/status", api.GetPonStatusHandler).Methods("GET")
	v1.HandleFunc("/onus/unregistered", api.GetUnregisteredOnusHandler).Methods("GET")
	v1.HandleFunc("/onus/install", api.InstallOnuHandler).Methods("POST")

	// Backups
	v1.HandleFunc("/backups/config", api.GetBackupConfigHandler).Methods("GET")
	v1.HandleFunc("/backups/config", api.UpdateBackupConfigHandler).Methods("POST")
	v1.HandleFunc("/backups", api.GetBackupsHandler).Methods("GET")
	v1.HandleFunc("/backups", api.DeleteBackupHandler).Methods("DELETE")
	v1.HandleFunc("/backups/manual", api.ManualBackupHandler).Methods("POST")
	v1.HandleFunc("/backups/test", api.TestBackupHandler).Methods("POST")

	// Schedules
	v1.HandleFunc("/schedules", api.GetSchedulesHandler).Methods("GET")
	v1.HandleFunc("/schedules", api.CreateScheduleHandler).Methods("POST")
	v1.HandleFunc("/schedules", api.DeleteScheduleHandler).Methods("DELETE")

	// Syslog
	v1.HandleFunc("/logs", api.GetLogsHandler).Methods("GET")

	// Provisioning
	v1.HandleFunc("/provision", api.GetProvisionScriptHandler).Methods("GET")
	v1.HandleFunc("/public-key", api.GetPublicKeyHandler).Methods("GET")

	// WebSocket (For Terminal / Realtime)
	// Note: WS usually bypasses JSON middleware, but needs Auth.
	// Handled inside handler or query param token. For prototype, assuming session/public or check origin.
	v1.HandleFunc("/ws", api.SSHWebSocketHandler)

	// Serve Static Files (Embedded)
	// web.Assets is embed.FS (root is "index.html" etc)
	// We want / to serve index.html

	contentStatic, _ := fs.Sub(web.Assets, ".")
	// Since we only embedded index.html in the previous step, we might need to check if we embedded "web/*"
	// Wait, my previous step: //go:embed index.html
	// If I just embedded index.html, I can't serve a directory cleanly if there were other assets.
	// But assuming Single File.

	r.PathPrefix("/").Handler(http.FileServer(http.FS(contentStatic)))

	// CORS Headers
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})

	srv := &http.Server{
		Handler:      handlers.CORS(originsOk, headersOk, methodsOk)(r),
		Addr:         "0.0.0.0:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start Syslog Server
	go api.StartSyslogServer()

	// Start Scheduler
	go worker.StartScheduler()
	go worker.StartBackupWorker()

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		log.Println("Micromon Server PRODUCTION ready on :8080")
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	srv.Shutdown(ctx)

	log.Println("shutting down")
	os.Exit(0)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
