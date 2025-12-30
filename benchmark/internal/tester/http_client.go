package tester

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
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
	// SOCKS5 auth
	var auth *proxy.Auth
	if username != "" || password != "" {
		auth = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}

	// Base TCP dialer
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Custom dial function for Transport
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		timings, _ := ctx.Value(timingKey{}).(*dialTiming)

		// Create a forward dialer that SOCKS5 will use to connect to the proxy.
		// We wrap it to capture the TCP connection time to the proxy server itself.
		forward := &forwardDialer{
			dialContext: baseDialer.DialContext,
			ctx:         ctx,
			timings:     timings,
		}

		// Create SOCKS5 dialer using our forwarder to connect to proxyAddr
		// Note: we use "tcp" for the proxy connection
		s5, err := proxy.SOCKS5("tcp", proxyAddr, auth, forward)
		if err != nil {
			return nil, err
		}

		// Measure total dial time (TCP to proxy + SOCKS5 handshake)
		start := time.Now()
		conn, err := s5.Dial(network, addr)
		if err != nil {
			return nil, err
		}

		if timings != nil {
			// Handshake time is total time from s5.Dial minus the TCP part recorded in the forwarder
			timings.handshake = time.Since(start) - timings.tcpConnect
			if timings.handshake < 0 {
				timings.handshake = 0
			}
		}

		return conn, nil
	}

	transport := &http.Transport{
		DialContext: dialFunc,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DisableKeepAlives:     true,
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

type timingKey struct{}

type dialTiming struct {
	tcpConnect time.Duration
	handshake  time.Duration
}

type forwardDialer struct {
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	ctx         context.Context
	timings     *dialTiming
}

func (f *forwardDialer) Dial(network, address string) (net.Conn, error) {
	start := time.Now()
	conn, err := f.dialContext(f.ctx, network, address)
	if err == nil && f.timings != nil {
		f.timings.tcpConnect = time.Since(start)
	}
	return conn, err
}

// MakeRequest performs an HTTP request and collects timing metrics
func (c *HTTPClient) MakeRequest(ctx context.Context, targetURL string) (*LatencyMetrics, error) {
	metrics := &LatencyMetrics{
		Success: false,
	}

	// Use a pointer to collect dial timings
	timings := &dialTiming{}
	ctx = context.WithValue(ctx, timingKey{}, timings)

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
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsDone = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	traceCtx := httptrace.WithClientTrace(req.Context(), trace)
	req = req.WithContext(traceCtx)

	// Execute request
	resp, err := c.client.Do(req)
	requestEnd := time.Now()

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

	// Extract timings from context-filled collector
	metrics.TCPConnect = timings.tcpConnect
	metrics.SOCKS5Handshake = timings.handshake

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
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		timings, _ := ctx.Value(timingKey{}).(*dialTiming)
		start := time.Now()
		conn, err := baseDialer.DialContext(ctx, network, addr)
		if err == nil && timings != nil {
			timings.tcpConnect = time.Since(start)
		}
		return conn, err
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialFunc,
			TLSHandshakeTimeout:   10 * time.Second,
			DisableKeepAlives:     true,
			MaxIdleConns:          -1,
			IdleConnTimeout:       1 * time.Nanosecond,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &HTTPClient{
		client:    httpClient,
		proxyName: "Direct Connection",
		timeout:   timeout,
	}
}
