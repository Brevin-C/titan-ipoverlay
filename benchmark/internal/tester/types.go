package tester

import (
	"time"
)

// LatencyMetrics contains all timing metrics for a request
type LatencyMetrics struct {
	// Proxy connection metrics (when using SOCKS5)
	ProxyDNS        time.Duration // DNS resolution of proxy server (if domain is used)
	ProxyTCP        time.Duration // TCP connection to proxy server
	SOCKS5Handshake time.Duration // SOCKS5 proxy handshake

	// Target website metrics
	DNSLookup    time.Duration // DNS resolution of target website
	TCPConnect   time.Duration // TCP connection to target (through proxy or direct)
	TLSHandshake time.Duration // TLS handshake time
	TTFB         time.Duration // Time to first byte
	TotalTime    time.Duration // Total end-to-end time

	// Request result
	Success    bool   // Whether the request succeeded
	Error      string // Error message if failed
	StatusCode int    // HTTP status code
}

// TestResult represents the aggregated results for a test run
type TestResult struct {
	TestName     string           // Name of the test
	ProxyName    string           // Name of the proxy used
	TargetURL    string           // Target URL tested
	TotalCount   int              // Total number of requests
	SuccessCount int              // Number of successful requests
	FailedCount  int              // Number of failed requests
	Metrics      []LatencyMetrics // Individual request metrics
	StartTime    time.Time        // When the test started
	EndTime      time.Time        // When the test ended
	Duration     time.Duration    // Total test duration
}

// Stats represents statistical analysis of latency data
type Stats struct {
	Mean   time.Duration
	Median time.Duration // P50
	P95    time.Duration
	P99    time.Duration
	Min    time.Duration
	Max    time.Duration
}

// ComparisonResult represents comparison between two proxies
type ComparisonResult struct {
	TitanResult      *TestResult
	CompetitorResult *TestResult
	TitanStats       map[string]*Stats // Key: metric name
	CompetitorStats  map[string]*Stats
	Differences      map[string]Difference // Key: metric name
}

// Difference represents the difference between two metric values
type Difference struct {
	Absolute   time.Duration // Absolute difference (Titan - Competitor)
	Percentage float64       // Percentage difference ((Titan-Competitor)/Competitor * 100)
}
