package exporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"titan-ipoverlay/benchmark/internal/tester"
)

// exportHTML exports a single test result to HTML
func (e *Exporter) exportHTML(result *tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".html")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"formatDuration": func(d time.Duration) string {
			if d == 0 {
				return "0.00"
			}
			return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000.0)
		},
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(singleReportTemplate)
	if err != nil {
		return err
	}

	data := prepareSingleReportData(result)
	if err := tmpl.Execute(file, data); err != nil {
		return err
	}

	fmt.Printf("✓ HTML report exported to: %s\n", filename)
	return nil
}

// exportBatchHTML exports multiple test results to an interactive HTML report
func (e *Exporter) exportBatchHTML(results []*tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".html")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"formatDuration": func(d time.Duration) string {
			return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000.0)
		},
	}

	tmpl, err := template.New("batch_report").Funcs(funcMap).Parse(batchReportTemplate)
	if err != nil {
		return err
	}

	data := prepareBatchReportData(results)
	if err := tmpl.Execute(file, data); err != nil {
		return err
	}

	fmt.Printf("✓ Batch HTML report exported to: %s\n", filename)
	return nil
}

// ProxyData holds data for a single proxy in the report
type ProxyData struct {
	Name        string
	ProxyServer string // SOCKS5 server address
	TestType    string // Test type: "Single" or "Concurrent"
	Concurrency int    // Concurrency level (0 for single)
	TargetURL   string
	TotalCount  int
	SuccessRate float64
	FailedCount int
	// Averages
	AvgDNS    float64
	AvgTCP    float64
	AvgSOCKS5 float64
	AvgTLS    float64
	AvgTTFB   float64
	AvgTotal  float64
	// Extended Stats (Total Latency)
	MinTotal    float64
	MaxTotal    float64
	MedianTotal float64
	P95Total    float64
	P99Total    float64
	IsBest      bool
	IsWorst     bool
}

// BatchReportData holds all data for the batch report
type BatchReportData struct {
	GeneratedAt  string
	TotalProxies int
	Proxies      []ProxyData
}

func prepareSingleReportData(result *tester.TestResult) map[string]interface{} {
	stats := calculateAverages(result)
	allStats := tester.CalculateAllStats(result)
	totalStats := allStats["total"]

	successRate := 0.0
	if result.TotalCount > 0 {
		successRate = float64(result.SuccessCount) / float64(result.TotalCount) * 100
	}

	// Calculate 'Server Processing' time for breakdown chart: TTFB - (DNS + TCP + SOCKS5 + TLS)
	// This makes the breakdown more logically accurate as a sum of parts.
	processing := stats["ttfb"] - (stats["dns"] + stats["tcp"] + stats["socks5"] + stats["tls"])
	if processing < 0 {
		processing = 0
	}

	// Determine test type from test name
	testType := "Sequential Sampling (10-worker pool)"
	concurrency := 0
	if strings.Contains(strings.ToLower(result.TestName), "并发") || strings.Contains(strings.ToLower(result.TestName), "concurrent") {
		testType = "Concurrent Load Test"
		// Try to extract concurrency number from test name
		for _, word := range strings.Fields(result.TestName) {
			if num, err := strconv.Atoi(strings.TrimSuffix(word, "并发")); err == nil {
				concurrency = num
				break
			}
		}
	}

	return map[string]interface{}{
		"ProxyName":    result.ProxyName,
		"ProxyServer":  result.ProxyServer,
		"TestName":     result.TestName,
		"TestType":     testType,
		"Concurrency":  concurrency,
		"TargetURL":    result.TargetURL,
		"GeneratedAt":  time.Now().Format("2006-01-02 15:04:05"),
		"TotalCount":   result.TotalCount,
		"SuccessCount": result.SuccessCount,
		"FailedCount":  result.FailedCount,
		"SuccessRate":  successRate,
		// Averages (Floats)
		"AvgProxyDNS": stats["proxy_dns"],
		"AvgProxyTCP": stats["proxy_tcp"],
		"AvgSOCKS5":   stats["socks5"],
		"AvgDNS":      stats["dns"],
		"AvgTCP":      stats["tcp"],
		"AvgTLS":      stats["tls"],
		"AvgProc":     processing,
		"AvgTTFB":     stats["ttfb"],
		"AvgTotal":    stats["total"],
		// Stats (Floats)
		"MinTotal": float64(totalStats.Min.Microseconds()) / 1000.0,
		"MaxTotal": float64(totalStats.Max.Microseconds()) / 1000.0,
		"P50Total": float64(totalStats.Median.Microseconds()) / 1000.0,
		"P95Total": float64(totalStats.P95.Microseconds()) / 1000.0,
		"P99Total": float64(totalStats.P99.Microseconds()) / 1000.0,
		"Metrics":  result.Metrics,
	}
}

func prepareBatchReportData(results []*tester.TestResult) BatchReportData {
	proxies := make([]ProxyData, len(results))

	var bestIdx, worstIdx int
	bestTotal := float64(^uint(0) >> 1) // max float
	worstTotal := 0.0

	for i, result := range results {
		stats := calculateAverages(result)
		allStats := tester.CalculateAllStats(result)
		totalStats := allStats["total"]

		successRate := 0.0
		if result.TotalCount > 0 {
			successRate = float64(result.SuccessCount) / float64(result.TotalCount) * 100
		}

		proxies[i] = ProxyData{
			Name:        result.ProxyName,
			TargetURL:   result.TargetURL,
			TotalCount:  result.TotalCount,
			SuccessRate: successRate,
			FailedCount: result.FailedCount,
			AvgDNS:      stats["dns"],
			AvgTCP:      stats["tcp"],
			AvgSOCKS5:   stats["socks5"],
			AvgTLS:      stats["tls"],
			AvgTTFB:     stats["ttfb"],
			AvgTotal:    stats["total"],
			MinTotal:    float64(totalStats.Min.Microseconds()) / 1000.0,
			MaxTotal:    float64(totalStats.Max.Microseconds()) / 1000.0,
			MedianTotal: float64(totalStats.Median.Microseconds()) / 1000.0,
			P95Total:    float64(totalStats.P95.Microseconds()) / 1000.0,
			P99Total:    float64(totalStats.P99.Microseconds()) / 1000.0,
		}

		// Track best and worst performers
		if successRate > 90 && stats["total"] < bestTotal && stats["total"] > 0 {
			bestTotal = stats["total"]
			bestIdx = i
		}
		if stats["total"] > worstTotal {
			worstTotal = stats["total"]
			worstIdx = i
		}
	}

	if len(proxies) > 0 && bestTotal < float64(^uint(0)>>1) {
		proxies[bestIdx].IsBest = true
	}
	if len(proxies) > 0 && worstTotal > 0 {
		proxies[worstIdx].IsWorst = true
	}

	return BatchReportData{
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
		TotalProxies: len(results),
		Proxies:      proxies,
	}
}

const singleReportTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proxy Performance Report - {{.ProxyName}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        :root {
            --primary: #6366f1;
            --primary-dark: #4f46e5;
            --secondary: #ec4899;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
            --background: #f3f4f6;
            --card-bg: #ffffff;
            --text-main: #1f2937;
            --text-muted: #6b7280;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, system-ui, sans-serif;
            background-color: var(--background);
            color: var(--text-main);
            line-height: 1.5;
            padding: 2rem;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        .header {
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            padding: 3rem;
            border-radius: 1.5rem;
            color: white;
            box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.1);
            margin-bottom: 2rem;
            position: relative;
            overflow: hidden;
        }
        .header::after {
            content: '';
            position: absolute;
            top: -50%;
            right: -10%;
            width: 300px;
            height: 300px;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 50%;
        }

        .header h1 { font-size: 2.5rem; margin-bottom: 0.5rem; display: flex; align-items: center; gap: 0.75rem; }
        .header p { opacity: 0.9; font-size: 1.1rem; }
        .header .meta { margin-top: 1.5rem; display: flex; gap: 2rem; font-size: 0.9rem; opacity: 0.8; }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }

        .stat-card {
            background: var(--card-bg);
            padding: 1.5rem;
            border-radius: 1rem;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
            transition: transform 0.2s;
        }
        .stat-card:hover { transform: translateY(-4px); }
        .stat-label { color: var(--text-muted); font-size: 0.875rem; font-weight: 600; text-transform: uppercase; margin-bottom: 0.5rem; }
        .stat-value { font-size: 2rem; font-weight: 700; color: var(--primary); }
        .stat-value.success { color: var(--success); }
        .stat-unit { font-size: 1rem; color: var(--text-muted); margin-left: 0.25rem; }

        .main-grid {
            display: grid;
            grid-template-columns: 2fr 1fr;
            gap: 2rem;
            margin-bottom: 2rem;
        }

        .card {
            background: var(--card-bg);
            padding: 2rem;
            border-radius: 1.5rem;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
        }

        .section-title {
            font-size: 1.25rem;
            font-weight: 700;
            margin-bottom: 1.5rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            color: var(--text-main);
            border-left: 4px solid var(--primary);
            padding-left: 1rem;
        }

        .chart-container { position: relative; height: 350px; }

        .details-section { margin-top: 2rem; }
        
        table { width: 100%; border-collapse: collapse; margin-top: 1rem; font-size: 0.9rem; }
        th { background: #f9fafb; padding: 1rem; text-align: left; font-weight: 600; color: var(--text-muted); border-bottom: 2px solid #e5e7eb; }
        td { padding: 1rem; border-bottom: 1px solid #e5e7eb; color: var(--text-main); }
        tr:hover { background-color: #f9fafb; }

        .badge {
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
        }
        .badge-success { background: #d1fae5; color: #065f46; }
        .badge-error { background: #fee2e2; color: #991b1b; }

        .metric-cell { font-family: ui-monospace, monospace; font-weight: 500; }
        
        @media (max-width: 1024px) {
            .main-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Proxy Performance Report</h1>
            <p>Comprehensive latency analysis for your proxy infrastructure</p>
            <div class="meta">
                <span><strong>Proxy:</strong> {{.ProxyName}}</span>
                <span><strong>Server:</strong> {{.ProxyServer}}</span>
                <span><strong>Target:</strong> {{.TargetURL}}</span>
                <span><strong>Generated:</strong> {{.GeneratedAt}}</span>
            </div>
            <div class="meta" style="margin-top: 8px; padding-top: 8px; border-top: 1px solid rgba(99, 102, 241, 0.2);">
                <span><strong>Test:</strong> {{.TestName}}</span>
                <span><strong>Type:</strong> {{.TestType}}{{if gt .Concurrency 0}} ({{.Concurrency}} concurrent){{end}}</span>
                <span><strong>Samples:</strong> {{.TotalCount}}</span>
            </div>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Success Rate</div>
                <div class="stat-value success">{{printf "%.2f" .SuccessRate}}<span class="stat-unit">%</span></div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Avg. Total Latency</div>
                <div class="stat-value">{{printf "%.2f" .AvgTotal}}<span class="stat-unit">ms</span></div>
            </div>
            <div class="stat-card">
                <div class="stat-label">P95 Latency</div>
                <div class="stat-value">{{printf "%.2f" .P95Total}}<span class="stat-unit">ms</span></div>
            </div>
            <div class="stat-card">
                <div class="stat-label">P99 Latency</div>
                <div class="stat-value">{{printf "%.2f" .P99Total}}<span class="stat-unit">ms</span></div>
            </div>
        </div>

        <div class="main-grid">
            <div class="card">
                <div class="section-title">⏱️ Latency Breakdown (Average)</div>
                <div class="chart-container">
                    <canvas id="latencyChart"></canvas>
                </div>
            </div>
            <div class="card">
                <div class="section-title">📊 Percentile Analysis</div>
                <table style="margin-top: 0">
                    <tr><td>Minimum</td><td class="metric-cell">{{printf "%.2f" .MinTotal}} ms</td></tr>
                    <tr><td>Median (P50)</td><td class="metric-cell">{{printf "%.2f" .P50Total}} ms</td></tr>
                    <tr><td>Average</td><td class="metric-cell">{{printf "%.2f" .AvgTotal}} ms</td></tr>
                    <tr><td>P95</td><td class="metric-cell">{{printf "%.2f" .P95Total}} ms</td></tr>
                    <tr><td>P99</td><td class="metric-cell">{{printf "%.2f" .P99Total}} ms</td></tr>
                    <tr><td>Maximum</td><td class="metric-cell">{{printf "%.2f" .MaxTotal}} ms</td></tr>
                </table>
            </div>
        </div>

        <div class="card details-section">
            <div class="section-title">📋 Detailed Request Log (Last 50)</div>
            <div style="overflow-x: auto;">
                <table>
                    <thead>
                        <tr>
                            <th>#</th>
                            <th>Status</th>
                            <th>Proxy DNS</th>
                            <th>Proxy TCP</th>
                            <th>SOCKS5</th>
                            <th>Tgt DNS</th>
                            <th>Tgt TCP</th>
                            <th>TLS</th>
                            <th>TTFB</th>
                            <th>Total</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $index, $m := .Metrics}}
                        {{if lt $index 50}}
                        <tr>
                            <td>{{add $index 1}}</td>
                            <td>
                                {{if $m.Success}}
                                <span class="badge badge-success">{{$m.StatusCode}} OK</span>
                                {{else}}
                                <span class="badge badge-error">{{if eq $m.StatusCode 0}}ERR{{else}}{{$m.StatusCode}}{{end}}</span>
                                {{end}}
                            </td>
                            <td class="metric-cell">{{formatDuration $m.ProxyDNS}}</td>
                            <td class="metric-cell">{{formatDuration $m.ProxyTCP}}</td>
                            <td class="metric-cell">{{formatDuration $m.SOCKS5Handshake}}</td>
                            <td class="metric-cell">{{formatDuration $m.DNSLookup}}</td>
                            <td class="metric-cell">{{formatDuration $m.TCPConnect}}</td>
                            <td class="metric-cell">{{formatDuration $m.TLSHandshake}}</td>
                            <td class="metric-cell">{{formatDuration $m.TTFB}}</td>
                            <td class="metric-cell"><strong>{{formatDuration $m.TotalTime}}</strong></td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <script>
        const ctx = document.getElementById('latencyChart').getContext('2d');
        new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['Proxy DNS', 'Proxy TCP', 'SOCKS5', 'Target DNS', 'Target TCP', 'TLS', 'Server Proc'],
                datasets: [{
                    label: 'Latency (ms)',
                    data: [{{.AvgProxyDNS}}, {{.AvgProxyTCP}}, {{.AvgSOCKS5}}, {{.AvgDNS}}, {{.AvgTCP}}, {{.AvgTLS}}, {{.AvgProc}}],
                    backgroundColor: [
                        'rgba(139, 92, 246, 0.8)',   // Purple for Proxy DNS
                        'rgba(99, 102, 241, 0.8)',   // Indigo for Proxy TCP
                        'rgba(59, 130, 246, 0.8)',   // Blue for SOCKS5
                        'rgba(14, 165, 233, 0.8)',   // Sky for Target DNS
                        'rgba(6, 182, 212, 0.8)',    // Cyan for Target TCP
                        'rgba(236, 72, 153, 0.8)',    // Pink for TLS
                        'rgba(249, 115, 22, 0.8)'    // Orange for Server Processing
                    ],
                    borderRadius: 8,
                    barThickness: 40
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        padding: 12,
                        backgroundColor: 'rgba(31, 41, 55, 0.9)',
                        titleFont: { size: 14, weight: 'bold' },
                        bodyFont: { size: 13 },
                        callbacks: {
                            label: (context) => {
                                return ' ' + context.dataset.label + ': ' + context.parsed.x.toFixed(2) + ' ms';
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        beginAtZero: true,
                        grid: { display: false },
                        ticks: { callback: v => v + ' ms' }
                    },
                    y: { grid: { display: false } }
                }
            }
        });
    </script>
</body>
</html>`

const batchReportTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Batch Proxy Performance Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        :root {
            --primary: #6366f1;
            --primary-dark: #4f46e5;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
            --background: #f8fafc;
            --card-bg: #ffffff;
            --text-main: #1e293b;
            --text-muted: #64748b;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', -apple-system, sans-serif;
            background-color: var(--background);
            color: var(--text-main);
            padding: 2.5rem;
            line-height: 1.6;
        }

        .container { max-width: 1400px; margin: 0 auto; }

        .header {
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            padding: 3.5rem;
            border-radius: 2rem;
            color: white;
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
            margin-bottom: 3rem;
        }
        .header h1 { font-size: 3rem; margin-bottom: 0.5rem; display: flex; align-items: center; gap: 1rem; }
        .header p { opacity: 0.9; font-size: 1.2rem; }

        .section-title {
            font-size: 1.5rem;
            font-weight: 700;
            margin: 3rem 0 1.5rem;
            display: flex;
            align-items: center;
            gap: 0.75rem;
            color: var(--text-main);
        }

        .chart-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
            gap: 2rem;
        }

        .card {
            background: var(--card-bg);
            padding: 2rem;
            border-radius: 1.5rem;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
        }

        .chart-container { position: relative; height: 350px; }

        .table-responsive {
            background: var(--card-bg);
            border-radius: 1.5rem;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
            overflow: hidden;
            margin-top: 1rem;
        }

        table { width: 100%; border-collapse: collapse; }
        th { background: #f1f5f9; padding: 1.25rem 1rem; text-align: left; font-weight: 600; color: var(--text-muted); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; }
        td { padding: 1.25rem 1rem; border-bottom: 1px solid #e2e8f0; font-size: 0.95rem; }
        tr:last-child td { border-bottom: none; }
        tr:hover { background-color: #f8fafc; }

        .proxy-info { display: flex; align-items: center; gap: 0.5rem; }
        .proxy-name { font-weight: 700; color: var(--primary); }
        
        .badge {
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 700;
            text-transform: uppercase;
        }
        .badge-best { background: #d1fae5; color: #065f46; border: 1px solid #34d399; }
        .badge-worst { background: #fee2e2; color: #991b1b; border: 1px solid #f87171; }

        .success-rate {
            font-weight: 700;
            padding: 0.4rem 0.8rem;
            border-radius: 0.5rem;
        }
        .success-high { background: #ecfdf5; color: #059669; }
        .success-mid  { background: #fffbeb; color: #d97706; }
        .success-low  { background: #fef2f2; color: #dc2626; }

        .metric-val { font-family: ui-monospace, monospace; font-weight: 500; text-align: right; }
        .metric-val.total { font-weight: 700; color: var(--primary-dark); }
        
        @media (max-width: 768px) {
            body { padding: 1rem; }
            .chart-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 Batch Proxy Report</h1>
            <p>Comparative analysis of {{.TotalProxies}} proxy nodes | Generated at {{.GeneratedAt}}</p>
        </div>

        <div class="section-title">📈 Performance Comparison</div>
        <div class="chart-grid">
            <div class="card">
                <h3 style="margin-bottom: 1.5rem">⚡ Average TTFB (ms)</h3>
                <div class="chart-container">
                    <canvas id="ttfbChart"></canvas>
                </div>
            </div>
            <div class="card">
                <h3 style="margin-bottom: 1.5rem">⏱️ P95 Total Latency (ms)</h3>
                <div class="chart-container">
                    <canvas id="p95Chart"></canvas>
                </div>
            </div>
        </div>

        <div class="section-title">📋 Detailed Performance Matrix</div>
        <div class="table-responsive">
            <table>
                <thead>
                    <tr>
                        <th>Proxy Node</th>
                        <th style="text-align: center">Success</th>
                        <th style="text-align: right">Avg DNS</th>
                        <th style="text-align: right">SOCKS5</th>
                        <th style="text-align: right">TTFB</th>
                        <th style="text-align: right">P50 Total</th>
                        <th style="text-align: right">P95 Total</th>
                        <th style="text-align: right">Avg Total</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Proxies}}
                    <tr>
                        <td>
                            <div class="proxy-info">
                                <span class="proxy-name">{{.Name}}</span>
                                {{if .IsBest}}<span class="badge badge-best">⭐ Best</span>{{end}}
                                {{if .IsWorst}}<span class="badge badge-worst">⚠️ Slow</span>{{end}}
                            </div>
                        </td>
                        <td style="text-align: center">
                            <span class="success-rate {{if ge .SuccessRate 98.0}}success-high{{else if ge .SuccessRate 90.0}}success-mid{{else}}success-low{{end}}">
                                {{printf "%.1f" .SuccessRate}}%
                            </span>
                        </td>
                        <td class="metric-val">{{printf "%.2f" .AvgDNS}}</td>
                        <td class="metric-val">{{printf "%.2f" .AvgSOCKS5}}</td>
                        <td class="metric-val">{{printf "%.2f" .AvgTTFB}}</td>
                        <td class="metric-val">{{printf "%.2f" .MedianTotal}}</td>
                        <td class="metric-val">{{printf "%.2f" .P95Total}}</td>
                        <td class="metric-val total">{{printf "%.2f" .AvgTotal}} ms</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>

    <script>
        const proxyNames = [{{range .Proxies}}'{{.Name}}',{{end}}];
        
        const chartOptions = {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: false },
                tooltip: {
                    padding: 12,
                    backgroundColor: 'rgba(30, 41, 59, 1)',
                    titleFont: { size: 14, weight: 'bold' }
                }
            },
            scales: {
                y: { beginAtZero: true, grid: { color: 'rgba(0,0,0,0.05)' }, ticks: { callback: v => v + ' ms' } },
                x: { grid: { display: false } }
            }
        };

        const colors = [
            'rgba(99, 102, 241, 0.8)',
            'rgba(16, 185, 129, 0.8)',
            'rgba(245, 158, 11, 0.8)',
            'rgba(239, 68, 68, 0.8)',
            'rgba(139, 92, 246, 0.8)',
            'rgba(236, 72, 153, 0.8)',
            'rgba(20, 184, 166, 0.8)'
        ];

        // TTFB Chart
        new Chart(document.getElementById('ttfbChart'), {
            type: 'bar',
            data: {
                labels: proxyNames,
                datasets: [{
                    data: [{{range .Proxies}}{{.AvgTTFB}},{{end}}],
                    backgroundColor: colors,
                    borderRadius: 12
                }]
            },
            options: chartOptions
        });

        // P95 Chart
        new Chart(document.getElementById('p95Chart'), {
            type: 'bar',
            data: {
                labels: proxyNames,
                datasets: [{
                    data: [{{range .Proxies}}{{.P95Total}},{{end}}],
                    backgroundColor: colors,
                    borderRadius: 12
                }]
            },
            options: chartOptions
        });
    </script>
</body>
</html>`
