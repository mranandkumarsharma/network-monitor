package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"network-monitor/internal/storage"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Handler struct {
	store    *storage.Store
	upgrader websocket.Upgrader
}

type APIResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewHandler(store *storage.Store) *Handler {
	return &Handler{
		store: store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for simplicity
			},
		},
	}
}

func (h *Handler) sendResponse(w http.ResponseWriter, status string, data interface{}, err string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Status:    status,
		Data:      data,
		Error:     err,
		Timestamp: time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetTraffic(w http.ResponseWriter, r *http.Request) {
	interfaces := h.store.GetInterfaces()
	h.sendResponse(w, "success", map[string]interface{}{
		"interfaces": interfaces,
	}, "", http.StatusOK)
}

func (h *Handler) GetInterfaceTraffic(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	interfaceName := vars["interface"]
	
	interfaces := h.store.GetInterfaces()
	if iface, exists := interfaces[interfaceName]; exists {
		h.sendResponse(w, "success", iface, "", http.StatusOK)
	} else {
		h.sendResponse(w, "error", nil, "Interface not found", http.StatusNotFound)
	}
}

func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
	devices := h.store.GetDevices()
	
	// Convert map to slice for easier frontend handling
	deviceList := make([]*storage.Device, 0, len(devices))
	for _, device := range devices {
		deviceList = append(deviceList, device)
	}
	
	h.sendResponse(w, "success", map[string]interface{}{
		"devices": deviceList,
		"total":   len(deviceList),
	}, "", http.StatusOK)
}

func (h *Handler) GetActiveDevices(w http.ResponseWriter, r *http.Request) {
	devices := h.store.GetDevices()
	
	// Filter only active devices
	activeDevices := make([]*storage.Device, 0)
	for _, device := range devices {
		if device.IsActive {
			activeDevices = append(activeDevices, device)
		}
	}
	
	h.sendResponse(w, "success", map[string]interface{}{
		"devices": activeDevices,
		"total":   len(activeDevices),
	}, "", http.StatusOK)
}

func (h *Handler) GetPing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	host := vars["host"]
	
	pings := h.store.GetPings()
	if ping, exists := pings[host]; exists {
		h.sendResponse(w, "success", ping, "", http.StatusOK)
	} else {
		h.sendResponse(w, "error", nil, "Host not found", http.StatusNotFound)
	}
}

func (h *Handler) GetAllPings(w http.ResponseWriter, r *http.Request) {
	pings := h.store.GetPings()
	h.sendResponse(w, "success", map[string]interface{}{
		"pings": pings,
	}, "", http.StatusOK)
}

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "traffic"
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"network_%s_%s.csv\"", dataType, time.Now().Format("2006-01-02_15-04-05")))
	w.Header().Set("Access-Control-Allow-Origin", "*")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	switch dataType {
	case "traffic":
		h.exportTrafficCSV(writer)
	case "devices":
		h.exportDevicesCSV(writer)
	case "ping":
		h.exportPingCSV(writer)
	default:
		http.Error(w, "Invalid export type", http.StatusBadRequest)
		return
	}
}

func (h *Handler) exportTrafficCSV(writer *csv.Writer) {
	interfaces := h.store.GetInterfaces()
	
	// Write header
	writer.Write([]string{"Timestamp", "Interface", "Bytes_RX", "Bytes_TX", "Speed_RX", "Speed_TX"})
	
	// Write data
	for _, iface := range interfaces {
		for _, point := range iface.History {
			writer.Write([]string{
				point.Timestamp.Format(time.RFC3339),
				iface.Name,
				strconv.FormatUint(point.BytesRx, 10),
				strconv.FormatUint(point.BytesTx, 10),
				fmt.Sprintf("%.2f", point.SpeedRx),
				fmt.Sprintf("%.2f", point.SpeedTx),
			})
		}
	}
}

func (h *Handler) exportDevicesCSV(writer *csv.Writer) {
	devices := h.store.GetDevices()
	
	// Write header
	writer.Write([]string{"IP", "MAC", "Hostname", "Last_Seen", "Active"})
	
	// Write data
	for _, device := range devices {
		writer.Write([]string{
			device.IP,
			device.MAC,
			device.Hostname,
			device.LastSeen.Format(time.RFC3339),
			strconv.FormatBool(device.IsActive),
		})
	}
}

func (h *Handler) exportPingCSV(writer *csv.Writer) {
	pings := h.store.GetPings()
	
	// Write header
	writer.Write([]string{"Timestamp", "Host", "Latency_MS", "Success"})
	
	// Write data
	for host, ping := range pings {
		for _, point := range ping.History {
			latencyMs := float64(point.Latency.Nanoseconds()) / 1000000.0
			writer.Write([]string{
				point.Timestamp.Format(time.RFC3339),
				host,
				fmt.Sprintf("%.2f", latencyMs),
				strconv.FormatBool(point.Success),
			})
		}
	}
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket client connected")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		data := h.getLiveData()
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}

func (h *Handler) getLiveData() map[string]interface{} {
	interfaces := h.store.GetInterfaces()
	devices := h.store.GetDevices()
	pings := h.store.GetPings()

	// Count active devices
	activeCount := 0
	for _, device := range devices {
		if device.IsActive {
			activeCount++
		}
	}

	// Calculate total bandwidth
	totalRx := 0.0
	totalTx := 0.0
	for _, iface := range interfaces {
		totalRx += iface.SpeedRx
		totalTx += iface.SpeedTx
	}

	return map[string]interface{}{
		"timestamp":      time.Now(),
		"interfaces":     interfaces,
		"devices":        devices,
		"pings":          pings,
		"active_devices": activeCount,
		"total_devices":  len(devices),
		"total_rx":       totalRx,
		"total_tx":       totalTx,
	}
}