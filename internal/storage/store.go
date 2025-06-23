package storage

import (
	"sync"
	"time"
)

const (
	MaxHistoryPoints = 100
	MaxDevices       = 256
)

type Store struct {
	mu          sync.RWMutex
	Interfaces  map[string]*InterfaceStats
	Devices     map[string]*Device
	PingResults map[string]*PingStats
	LastUpdated time.Time
}

type InterfaceStats struct {
	Name      string       `json:"name"`
	BytesRx   uint64       `json:"bytes_rx"`
	BytesTx   uint64       `json:"bytes_tx"`
	PacketsRx uint64       `json:"packets_rx"`
	PacketsTx uint64       `json:"packets_tx"`
	SpeedRx   float64      `json:"speed_rx"`   // bytes per second
	SpeedTx   float64      `json:"speed_tx"`   // bytes per second
	History   []DataPoint  `json:"history"`
	LastCheck time.Time    `json:"last_check"`
}

type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	BytesRx   uint64    `json:"bytes_rx"`
	BytesTx   uint64    `json:"bytes_tx"`
	SpeedRx   float64   `json:"speed_rx"`
	SpeedTx   float64   `json:"speed_tx"`
}

type Device struct {
	IP       string    `json:"ip"`
	MAC      string    `json:"mac"`
	Hostname string    `json:"hostname"`
	LastSeen time.Time `json:"last_seen"`
	IsActive bool      `json:"is_active"`
	Vendor   string    `json:"vendor,omitempty"`
}

type PingStats struct {
	Host         string        `json:"host"`
	LastLatency  time.Duration `json:"last_latency"`
	AvgLatency   time.Duration `json:"avg_latency"`
	PacketLoss   float64       `json:"packet_loss"`
	TotalPings   int           `json:"total_pings"`
	FailedPings  int           `json:"failed_pings"`
	History      []PingPoint   `json:"history"`
	LastUpdated  time.Time     `json:"last_updated"`
}

type PingPoint struct {
	Timestamp time.Time     `json:"timestamp"`
	Latency   time.Duration `json:"latency"`
	Success   bool          `json:"success"`
}

func NewStore() *Store {
	return &Store{
		Interfaces:  make(map[string]*InterfaceStats),
		Devices:     make(map[string]*Device),
		PingResults: make(map[string]*PingStats),
		LastUpdated: time.Now(),
	}
}

func (s *Store) UpdateInterface(name string, bytesRx, bytesTx, packetsRx, packetsTx uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	
	if iface, exists := s.Interfaces[name]; exists {
		// Calculate speeds
		timeDiff := now.Sub(iface.LastCheck).Seconds()
		if timeDiff > 0 {
			iface.SpeedRx = float64(bytesRx-iface.BytesRx) / timeDiff
			iface.SpeedTx = float64(bytesTx-iface.BytesTx) / timeDiff
		}

		// Add to history
		point := DataPoint{
			Timestamp: now,
			BytesRx:   bytesRx,
			BytesTx:   bytesTx,
			SpeedRx:   iface.SpeedRx,
			SpeedTx:   iface.SpeedTx,
		}
		
		iface.History = append(iface.History, point)
		if len(iface.History) > MaxHistoryPoints {
			iface.History = iface.History[1:]
		}

		// Update current values
		iface.BytesRx = bytesRx
		iface.BytesTx = bytesTx
		iface.PacketsRx = packetsRx
		iface.PacketsTx = packetsTx
		iface.LastCheck = now
	} else {
		// New interface
		s.Interfaces[name] = &InterfaceStats{
			Name:      name,
			BytesRx:   bytesRx,
			BytesTx:   bytesTx,
			PacketsRx: packetsRx,
			PacketsTx: packetsTx,
			SpeedRx:   0,
			SpeedTx:   0,
			History:   []DataPoint{},
			LastCheck: now,
		}
	}
	
	s.LastUpdated = now
}

func (s *Store) UpdateDevice(ip, mac, hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	
	if device, exists := s.Devices[ip]; exists {
		device.LastSeen = now
		device.IsActive = true
		if hostname != "" && device.Hostname == "" {
			device.Hostname = hostname
		}
		if mac != "" && device.MAC == "" {
			device.MAC = mac
		}
	} else {
		s.Devices[ip] = &Device{
			IP:       ip,
			MAC:      mac,
			Hostname: hostname,
			LastSeen: now,
			IsActive: true,
		}
	}
}

func (s *Store) UpdatePing(host string, latency time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	
	if ping, exists := s.PingResults[host]; exists {
		ping.TotalPings++
		if !success {
			ping.FailedPings++
		} else {
			ping.LastLatency = latency
		}
		
		// Calculate packet loss
		ping.PacketLoss = float64(ping.FailedPings) / float64(ping.TotalPings) * 100
		
		// Calculate average latency (only successful pings)
		if success && len(ping.History) > 0 {
			total := latency
			count := 1
			for _, point := range ping.History {
				if point.Success {
					total += point.Latency
					count++
				}
			}
			ping.AvgLatency = total / time.Duration(count)
		}
		
		// Add to history
		point := PingPoint{
			Timestamp: now,
			Latency:   latency,
			Success:   success,
		}
		
		ping.History = append(ping.History, point)
		if len(ping.History) > MaxHistoryPoints {
			ping.History = ping.History[1:]
		}
		
		ping.LastUpdated = now
	} else {
		// New ping target
		s.PingResults[host] = &PingStats{
			Host:        host,
			LastLatency: latency,
			TotalPings:  1,
			FailedPings: 0,
			History:     []PingPoint{{Timestamp: now, Latency: latency, Success: success}},
			LastUpdated: now,
		}
		
		if !success {
			s.PingResults[host].FailedPings = 1
			s.PingResults[host].PacketLoss = 100.0
		}
	}
}
// ...existing code...

// Add this method to your Store struct
func (s *Store) StorePingData(host string, rtt time.Duration, success bool, method string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()
    
    if ping, exists := s.PingResults[host]; exists {
        ping.TotalPings++
        if !success {
            ping.FailedPings++
        } else {
            ping.LastLatency = rtt
        }
        
        // Calculate packet loss
        ping.PacketLoss = float64(ping.FailedPings) / float64(ping.TotalPings) * 100
        
        // Calculate average latency (only successful pings)
        if success && len(ping.History) > 0 {
            total := rtt
            count := 1
            for _, point := range ping.History {
                if point.Success {
                    total += point.Latency
                    count++
                }
            }
            ping.AvgLatency = total / time.Duration(count)
        }
        
        // Add to history
        point := PingPoint{
            Timestamp: now,
            Latency:   rtt,
            Success:   success,
        }
        
        ping.History = append(ping.History, point)
        if len(ping.History) > MaxHistoryPoints {
            ping.History = ping.History[1:]
        }
        
        ping.LastUpdated = now
    } else {
        // New ping target
        avgLatency := time.Duration(0)
        packetLoss := 0.0
        failedPings := 0
        
        if success {
            avgLatency = rtt
        } else {
            failedPings = 1
            packetLoss = 100.0
        }
        
        s.PingResults[host] = &PingStats{
            Host:        host,
            LastLatency: rtt,
            AvgLatency:  avgLatency,
            PacketLoss:  packetLoss,
            TotalPings:  1,
            FailedPings: failedPings,
            History:     []PingPoint{{Timestamp: now, Latency: rtt, Success: success}},
            LastUpdated: now,
        }
    }
    
    s.LastUpdated = now
}


func (s *Store) GetInterfaces() map[string]*InterfaceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*InterfaceStats)
	for k, v := range s.Interfaces {
		result[k] = v
	}
	return result
}

func (s *Store) GetDevices() map[string]*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Mark devices inactive if not seen for 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	result := make(map[string]*Device)
	
	for k, v := range s.Devices {
		device := *v // copy
		if device.LastSeen.Before(cutoff) {
			device.IsActive = false
		}
		result[k] = &device
	}
	return result
}

func (s *Store) GetPings() map[string]*PingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*PingStats)
	for k, v := range s.PingResults {
		result[k] = v
	}
	return result
}