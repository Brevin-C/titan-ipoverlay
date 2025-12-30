package tester

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// HTTPClient wraps http.Client with metric collection capabilities
type HTTPClient struct {
	client    *http.Client
	proxyAddr string
	proxyName string
	username  string
	password  string
	timeout   time.Duration
}

// NewHTTPClient creates a new HTTP client with SOCKS5 proxy support
func NewHTTPClient(proxyAddr, proxyName, username, password string, timeout time.Duration) (*HTTPClient, error) {
	// Parse proxy address
	proxyURL, err := url.Parse(fmt.Sprintf("socks5://%s", proxyAddr))
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address: %w", err)
	}

	// Create SOCKS5 dialer
	var auth *proxy.Auth
	if username != "" || password != "" {
		auth = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP transport with the SOCKS5 dialer
	transport := &http.Transport{
		Dial: dialer.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DisableKeepAlives:     true, // Ensure every request establishes a new connection
		MaxIdleConns:          -1,
		IdleConnTimeout:       1 * time.Nanosecond,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &HTTPClient{
		client:    httpClient,
		proxyAddr: proxyAddr,
		proxyName: proxyName,
		username:  username,
		password:  password,
		timeout:   timeout,
	}, nil
}

// MakeRequest performs an HTTP request and collects timing metrics
func (c *HTTPClient) MakeRequest(ctx context.Context, targetURL string) (*LatencyMetrics, error) {
	metrics := &LatencyMetrics{
		Success: false,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		metrics.Error = fmt.Sprintf("failed to create request: %v", err)
		return metrics, err
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Track timing using httptrace
	var (
		dnsStart     time.Time
		dnsDone      time.Time
		connectStart time.Time
		connectDone  time.Time
		tlsStart     time.Time
		tlsDone      time.Time
		gotFirstByte time.Time
		requestStart = time.Now()
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, err error) {
			connectDone = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
			// If connectDone hasn't been set yet (can happen in some Go versions with custom dialers)
			if connectDone.IsZero() {
				connectDone = tlsStart
			}
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsDone = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

	// Track actual start of the request call
	actualCallStart := time.Now()
	resp, err := c.client.Do(req)
	requestEnd := time.Now()

	// Ensure connectStart/Done are set if the trace missed them
	if connectStart.IsZero() {
		connectStart = actualCallStart
	}
	if connectDone.IsZero() {
		if !tlsStart.IsZero() {
			connectDone = tlsStart
		} else if !gotFirstByte.IsZero() {
			connectDone = gotFirstByte
		} else {
			connectDone = requestEnd
		}
	}

	if err != nil {
		metrics.Error = fmt.Sprintf("request failed: %v", err)
		metrics.TotalTime = requestEnd.Sub(requestStart)
		return metrics, err
	}
	defer resp.Body.Close()

	// Calculate timing metrics
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		metrics.DNSLookup = dnsDone.Sub(dnsStart)
	}

	if !connectStart.IsZero() && !connectDone.IsZero() {
		metrics.TCPConnect = connectDone.Sub(connectStart)

		// Estimate SOCKS5 handshake time (part of TCP connect when using proxy)
		// This is an approximation - actual SOCKS5 handshake is embedded in the connect phase
		if !tlsStart.IsZero() {
			// SOCKS5 handshake is between TCP connect and TLS start
			metrics.SOCKS5Handshake = tlsStart.Sub(connectDone)
		}
	}

	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		metrics.TLSHandshake = tlsDone.Sub(tlsStart)
	}

	if !gotFirstByte.IsZero() {
		metrics.TTFB = gotFirstByte.Sub(requestStart)
	}

	metrics.TotalTime = requestEnd.Sub(requestStart)
	metrics.StatusCode = resp.StatusCode
	metrics.Success = resp.StatusCode >= 200 && resp.StatusCode < 400

	if !metrics.Success {
		metrics.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return metrics, nil
}

// NewDirectHTTPClient creates an HTTP client without proxy (for direct connection testing)
func NewDirectHTTPClient(timeout time.Duration) *HTTPClient {
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &HTTPClient{
		client:    httpClient,
		proxyName: "Direct Connection",
		timeout:   timeout,
	}
}
