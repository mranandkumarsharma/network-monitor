package collector

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"network-monitor/internal/storage"
)

type DeviceInfo struct {
	IP       string
	MAC      string
	Hostname string
}

type DeviceCollector struct {
	store *storage.Store
}

func NewDeviceCollector(store *storage.Store) *DeviceCollector {
	return &DeviceCollector{store: store}
}

func (dc *DeviceCollector) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Println("Device collector started")

	for range ticker.C {
		dc.discoverDevices()
	}
}

func (dc *DeviceCollector) discoverDevices() {
	devices := dc.parseARPTable()
	subnet := dc.getLocalSubnet()

	if subnet != "" {
		for _, ip := range dc.pingSweep(subnet) {
			if _, exists := devices[ip]; !exists {
				devices[ip] = &DeviceInfo{IP: ip}
			}
		}
	}

	for _, device := range devices {
		if device.Hostname == "" {
			device.Hostname = dc.resolveHostname(device.IP)
		}
		dc.store.UpdateDevice(device.IP, device.MAC, device.Hostname)
	}
}

// parseARPTable fetches IP-MAC pairs using the system arp command (cross-platform)
func (dc *DeviceCollector) parseARPTable() map[string]*DeviceInfo {
	devices := make(map[string]*DeviceInfo)
	cmd := exec.Command("arp", "-a")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		log.Printf("Error executing arp: %v", err)
		return devices
	}

	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		ip := extractIP(parts[0])
		mac := extractMAC(line)

		if ip != "" && mac != "" {
			devices[ip] = &DeviceInfo{IP: ip, MAC: mac}
		}
	}

	return devices
}

func extractIP(s string) string {
	s = strings.Trim(s, "()")
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return ""
}

func extractMAC(s string) string {
	// Looks for MAC in string like: "00:1a:2b:3c:4d:5e"
	for _, word := range strings.Fields(s) {
		if strings.Count(word, ":") == 5 {
			return strings.ToLower(word)
		}
	}
	return ""
}

// getLocalSubnet returns the first usable IPv4 subnet
func (dc *DeviceCollector) getLocalSubnet() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Println("Error fetching interfaces:", err)
		return ""
	}

	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.String()
			}
		}
	}

	return ""
}

// pingSweep pings IPs in subnet to discover active hosts
func (dc *DeviceCollector) pingSweep(subnet string) []string {
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil
	}

	var active []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ipStr := ip.String()
		if strings.HasSuffix(ipStr, ".0") || strings.HasSuffix(ipStr, ".255") {
			continue
		}
		if dc.ping(ipStr) {
			active = append(active, ipStr)
		}
	}
	return active
}

// incIP increases IP by one
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}

// ping sends a single ICMP ping
func (dc *DeviceCollector) ping(ip string) bool {
	cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
	if isWindows() {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	}
	return cmd.Run() == nil
}

// resolveHostname resolves hostname from IP
func (dc *DeviceCollector) resolveHostname(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

// isWindows returns true if the OS is Windows
func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}
