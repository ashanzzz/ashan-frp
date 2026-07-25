package chmlfrp

import (
	"fmt"
	"net"
	"time"
)

type SpeedTestResult struct {
	RealIP    string  `json:"real_ip"`
	LatencyMS int     `json:"latency_ms"`
	SpeedMbps float64 `json:"speed_mbps"`
	Reachable bool    `json:"reachable"`
	Error     string  `json:"error,omitempty"`
}

func MeasureNodeSpeed(targetIP string, targetPort int) SpeedTestResult {
	if targetIP == "" {
		return SpeedTestResult{Reachable: false, Error: "target IP is empty"}
	}
	if targetPort <= 0 {
		targetPort = 80
	}
	address := net.JoinHostPort(targetIP, fmt.Sprintf("%d", targetPort))

	// Measure RTT Latency across 3 TCP connections
	var totalDuration time.Duration
	successfulChecks := 0
	timeout := 3 * time.Second

	for i := 0; i < 3; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			elapsed := time.Since(start)
			conn.Close()
			totalDuration += elapsed
			successfulChecks++
		}
	}

	if successfulChecks == 0 {
		return SpeedTestResult{RealIP: targetIP, Reachable: false, Error: "TCP dial connection timed out"}
	}

	avgLatencyMS := int((totalDuration / time.Duration(successfulChecks)).Milliseconds())

	// Estimate throughput speed (Mbps)
	// Base estimation derived from RTT: lower latency = higher throughput capacity
	estimatedMbps := 1000.0 / float64(avgLatencyMS+1)
	if estimatedMbps > 500.0 {
		estimatedMbps = 500.0
	}

	return SpeedTestResult{
		RealIP:    targetIP,
		LatencyMS: avgLatencyMS,
		SpeedMbps: estimatedMbps,
		Reachable: true,
	}
}
