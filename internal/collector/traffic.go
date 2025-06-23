package collector

import (
	"log"
	"time"

	"network-monitor/internal/storage"

	"github.com/shirou/gopsutil/v3/net"
)

type TrafficCollector struct {
	store *storage.Store
}

func NewTrafficCollector(store *storage.Store) *TrafficCollector {
	return &TrafficCollector{store: store}
}

func (tc *TrafficCollector) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("Traffic collector started")

	for {
		tc.collectTrafficStats()
		<-ticker.C
	}
}

func (tc *TrafficCollector) collectTrafficStats() {
	interfaces, err := net.IOCounters(true)
	if err != nil {
		log.Printf("Error collecting network stats: %v", err)
		return
	}

	for _, iface := range interfaces {
		// Skip loopback interfaces (Linux and Windows)
		if iface.Name == "lo" || iface.Name == "Loopback Pseudo-Interface 1" {
			continue
		}

		tc.store.UpdateInterface(
			iface.Name,
			iface.BytesRecv,
			iface.BytesSent,
			iface.PacketsRecv,
			iface.PacketsSent,
		)
	}
}
