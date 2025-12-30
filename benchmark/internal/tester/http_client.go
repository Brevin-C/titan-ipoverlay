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

		// 1. TCP Connect to Proxy
		start := time.Now()
		tcpConn, err := baseDialer.DialContext(ctx, network, proxyURL.Host)
		if err != nil {
			return nil, err
		}
		tcpElapsed := time.Since(start)
		if timings != nil {
			timings.tcpConnect = tcpElapsed
		}

		// 2. SOCKS5 Handshake
		start = time.Now()
		dialer, err := proxy.SOCKS5(network, proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			tcpConn.Close()
			return nil, err
		}

		// We need to measure just the handshake.
		// A better way is to use a custom SOCKS5 implementation, but since we are using proxy.SOCKS5:
		conn, err := dialer.Dial(network, addr)
		if err != nil {
			return nil, err
		}

		if timings != nil {
			// Total dial time minus TCP connect time is the handshake time
			// Actually, proxy.Dialer.Dial(network, addr) does both: connects to proxy and handshakes.
			// Since we want to distinguish:
			timings.handshake = time.Since(start) - tcpElapsed
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

type timingDialer struct {
	proxy.Dialer
	tcpDialer *net.Dialer
	timings   *dialTiming
}

func (d *timingDialer) Dial(network, addr string) (net.Conn, error) {
	start := time.Now()
	// TCP Connect
	tcpConn, err := d.tcpDialer.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	d.timings.tcpConnect = time.Since(start)

	// SOCKS5 Handshake happens when we use this dialer as 'forward' or wrap it.
	// Actually, the proxy.Dialer returned by proxy.SOCKS5 uses the forward dialer to connect to THE PROXY.
	// So we need to measure the wrap.
	return tcpConn, nil
}

type socksTimingDialer struct {
	socksDialer proxy.Dialer
	timings     *dialTiming
}

func (d *socksTimingDialer) Dial(network, addr string) (net.Conn, error) {
	start := time.Now()
	conn, err := d.socksDialer.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	totalDial := time.Since(start)
	d.timings.handshake = totalDial - d.timings.tcpConnect
	return conn, nil
}

// MakeRequest performs an HTTP request and collects timing metrics
func (c *HTTPClient) MakeRequest(ctx context.Context, targetURL string) (*LatencyMetrics, error) {
	metrics := &LatencyMetrics{
		Success: false,
	}

	// Use a pointer to collect dial timings
	timings := &dialTiming{}

	// Setup custom dialer for this request if it's a proxy request
	// Note: In NewHTTPClient we set the dialer. Here we might need to override it per request
	// but Transport.Dial is fixed.
	// Optimization: We can't easily change Transport.Dial per request without creating a new Transport.
	// However, we are already disabling KeepAlives, so we can afford a bit more overhead.
	// Alternative: Use context to pass a "timing collector" to the dialer already set in Transport.

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

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

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
		if err != nil {
			return nil, err
		}
		if timings != nil {
			timings.tcpConnect = time.Since(start)
		}
		return conn, nil
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
