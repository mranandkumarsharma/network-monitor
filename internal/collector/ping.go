package collector

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"network-monitor/internal/storage"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// PingResult holds the result of a ping operation
type PingResult struct {
    Host     string
    RTT      time.Duration
    Success  bool
    Method   string // "ICMP" or "TCP"
    Error    error
}

// PingCollector handles ping monitoring
type PingCollector struct {
    store   *storage.Store
    targets []string
}

// NewPingCollector creates a new ping collector
func NewPingCollector(store *storage.Store) *PingCollector {
    targets := []string{"8.8.8.8", "1.1.1.1", "127.0.0.1"}
    
    // Add gateway IP if available
    if gateway := getGatewayIP(); gateway != "" {
        targets = append([]string{gateway}, targets...)
        log.Printf("Gateway IP detected: %s", gateway)
    }
    
    log.Printf("OS: %s, Initialized ping collector with targets: %v", runtime.GOOS, targets)
    
    return &PingCollector{
        store:   store,
        targets: targets,
    }
}

// Start begins the ping collection process
func (pc *PingCollector) Start(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    log.Printf("Starting ping collector with interval %v", interval)
    
    // Run initial collection
    pc.collectPingData()
    
    for range ticker.C {
        pc.collectPingData()
    }
}

// collectPingData performs ping tests on all targets
func (pc *PingCollector) collectPingData() {
    for _, target := range pc.targets {
        result := pingHost(target)
        
        // Store the ping result
        pc.store.StorePingData(target, result.RTT, result.Success, result.Method)
        
        if result.Success {
            log.Printf("✓ %s ping to %s: RTT = %v", result.Method, result.Host, result.RTT)
        } else {
            log.Printf("✗ Ping to %s failed: %v", result.Host, result.Error)
        }
    }
}

// pingHost attempts ICMP ping first, then falls back to TCP ping
func pingHost(host string) PingResult {
    // Try ICMP first
    rtt, err := pingICMP(host)
    if err == nil {
        return PingResult{
            Host:    host,
            RTT:     rtt,
            Success: true,
            Method:  "ICMP",
        }
    }

    log.Printf("ICMP ping failed for %s: %v", host, err)
    log.Printf("Trying TCP ping fallback...")

    // Try TCP ping on common ports
    tcpPorts := []string{"80", "443", "53", "22"}
    
    for _, port := range tcpPorts {
        rtt, err := tcpPing(host, port, 3*time.Second)
        if err == nil {
            return PingResult{
                Host:    host,
                RTT:     rtt,
                Success: true,
                Method:  fmt.Sprintf("TCP:%s", port),
            }
        }
    }

    return PingResult{
        Host:    host,
        Success: false,
        Method:  "FAILED",
        Error:   fmt.Errorf("both ICMP and TCP ping failed"),
    }
}

// pingICMP performs ICMP ping
func pingICMP(host string) (time.Duration, error) {
    // Resolve IP address
    ipAddr, err := net.ResolveIPAddr("ip4", host)
    if err != nil {
        return 0, fmt.Errorf("resolve IP: %w", err)
    }

    // Create ICMP connection
    var conn *icmp.PacketConn
    if runtime.GOOS == "windows" {
        // Windows typically allows unprivileged ICMP
        conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    } else {
        // Unix systems usually require root for raw sockets
        conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
        if err != nil {
            // Try unprivileged ping socket (Linux 3.0+)
            conn, err = icmp.ListenPacket("udp4", "0.0.0.0")
        }
    }
    
    if err != nil {
        return 0, fmt.Errorf("listen ICMP (may need root/admin): %w", err)
    }
    defer conn.Close()

    // Create ICMP message
    msg := icmp.Message{
        Type: ipv4.ICMPTypeEcho,
        Code: 0,
        Body: &icmp.Echo{
            ID:   os.Getpid() & 0xffff,
            Seq:  1,
            Data: []byte("ping-test-data"),
        },
    }

    msgBytes, err := msg.Marshal(nil)
    if err != nil {
        return 0, fmt.Errorf("marshal ICMP: %w", err)
    }

    // Send ping
    start := time.Now()
    _, err = conn.WriteTo(msgBytes, ipAddr)
    if err != nil {
        return 0, fmt.Errorf("send ICMP: %w", err)
    }

    // Set read timeout
    err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    if err != nil {
        return 0, fmt.Errorf("set deadline: %w", err)
    }

    // Read reply
    reply := make([]byte, 1500)
    n, peer, err := conn.ReadFrom(reply)
    if err != nil {
        return 0, fmt.Errorf("read ICMP reply: %w", err)
    }

    duration := time.Since(start)

    // Verify the reply is from our target
    if peer.String() != ipAddr.String() {
        return 0, fmt.Errorf("reply from wrong host: got %s, expected %s", peer.String(), ipAddr.String())
    }

    // Parse ICMP reply
    var parsedMsg *icmp.Message
    if runtime.GOOS == "windows" {
        // Windows includes IP header in raw socket
        if n < 20 {
            return 0, fmt.Errorf("reply too short")
        }
        parsedMsg, err = icmp.ParseMessage(1, reply[20:n]) // 1 is the ICMP protocol number
    } else {
        parsedMsg, err = icmp.ParseMessage(1, reply[:n]) // 1 is the ICMP protocol number
    }
    
    if err != nil {
        return 0, fmt.Errorf("parse ICMP reply: %w", err)
    }

    // Check if it's an echo reply
    switch parsedMsg.Type {
    case ipv4.ICMPTypeEchoReply:
        // Verify it's our ping
        if echo, ok := parsedMsg.Body.(*icmp.Echo); ok {
            if echo.ID == (os.Getpid() & 0xffff) {
                return duration, nil
            }
        }
        return 0, fmt.Errorf("echo reply ID mismatch")
    case ipv4.ICMPTypeDestinationUnreachable:
        return 0, fmt.Errorf("destination unreachable")
    case ipv4.ICMPTypeTimeExceeded:
        return 0, fmt.Errorf("time exceeded")
    default:
        return 0, fmt.Errorf("unexpected ICMP type: %v", parsedMsg.Type)
    }
}

// tcpPing performs TCP connectivity test
func tcpPing(host, port string, timeout time.Duration) (time.Duration, error) {
    start := time.Now()
    
    // Attempt TCP connection
    conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
    if err != nil {
        return 0, fmt.Errorf("TCP connect to %s:%s: %w", host, port, err)
    }
    defer conn.Close()
    
    duration := time.Since(start)
    return duration, nil
}

// getGatewayIP detects the default gateway IP address
func getGatewayIP() string {
    log.Println("Detecting gateway IP...")
    
    // Try to get default route
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Printf("Could not detect gateway via UDP dial: %v", err)
        return ""
    }
    defer conn.Close()
    
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    localIP := localAddr.IP.String()
    
    // Parse the local IP to determine likely gateway
    ip := net.ParseIP(localIP)
    if ip == nil {
        return ""
    }
    
    // For typical home networks, gateway is usually x.x.x.1
    if ip.To4() != nil {
        octets := ip.To4()
        gateway := fmt.Sprintf("%d.%d.%d.1", octets[0], octets[1], octets[2])
        
        // Verify this IP responds before returning it
        if isValidGateway(gateway) {
            log.Printf("Gateway IP found: %s", gateway)
            return gateway
        }
    }
    
    log.Printf("No valid gateway IP found")
    return ""
}

// isValidGateway checks if the IP responds to ping or TCP
func isValidGateway(ip string) bool {
    // Quick TCP check on port 80 or 53
    conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "80"), 2*time.Second)
    if err == nil {
        conn.Close()
        return true
    }
    
    // Try DNS port
    conn, err = net.DialTimeout("tcp", net.JoinHostPort(ip, "53"), 2*time.Second)
    if err == nil {
        conn.Close()
        return true
    }
    
    return false
}