package main

import (
	"log"
	"net/http"
	"time"

	"network-monitor/internal/api"
	"network-monitor/internal/collector"
	"network-monitor/internal/storage"

	"github.com/gorilla/mux"
)

func main() {
	store := storage.NewStore()

	trafficCollector := collector.NewTrafficCollector(store)
	deviceCollector := collector.NewDeviceCollector(store)
	pingCollector := collector.NewPingCollector(store)

	go trafficCollector.Start(2 * time.Second)
	go deviceCollector.Start(10 * time.Second)
	go pingCollector.Start(5 * time.Second)

	apiHandler := api.NewHandler(store)

	r := mux.NewRouter()

	// API routes with /api prefix
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/traffic", apiHandler.GetTraffic).Methods("GET")
	apiRouter.HandleFunc("/traffic/{interface}", apiHandler.GetInterfaceTraffic).Methods("GET")
	apiRouter.HandleFunc("/devices", apiHandler.GetDevices).Methods("GET")
	apiRouter.HandleFunc("/devices/active", apiHandler.GetActiveDevices).Methods("GET")
	apiRouter.HandleFunc("/ping/{host}", apiHandler.GetPing).Methods("GET")
	apiRouter.HandleFunc("/ping", apiHandler.GetAllPings).Methods("GET")
	apiRouter.HandleFunc("/export/csv", apiHandler.ExportCSV).Methods("GET")

	// WebSocket route
	r.HandleFunc("/ws", apiHandler.HandleWebSocket)

	// Serve individual static files with correct names
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/index.html") // serves as index.html
	})
	r.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "./web/app.js")
	})
	r.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		http.ServeFile(w, r, "./web/styles.css")
	})

	log.Println("Network Monitor Dashboard starting on :8080")
	log.Println("Dashboard: http://localhost:8080")
	log.Println("API: http://localhost:8080/api/")

		log.Fatal(http.ListenAndServe(":8080", r))
	}
