package exporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
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

	tmpl, err := template.New("report").Parse(singleReportTemplate)
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

	tmpl, err := template.New("batch_report").Parse(batchReportTemplate)
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
	TargetURL   string
	TotalCount  int
	SuccessRate float64
	FailedCount int
	AvgDNS      float64
	AvgTCP      float64
	AvgSOCKS5   float64
	AvgTLS      float64
	AvgTTFB     float64
	AvgTotal    float64
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
	successRate := 0.0
	if result.TotalCount > 0 {
		successRate = float64(result.SuccessCount) / float64(result.TotalCount) * 100
	}

	return map[string]interface{}{
		"ProxyName":    result.ProxyName,
		"TargetURL":    result.TargetURL,
		"GeneratedAt":  time.Now().Format("2006-01-02 15:04:05"),
		"TotalCount":   result.TotalCount,
		"SuccessCount": result.SuccessCount,
		"FailedCount":  result.FailedCount,
		"SuccessRate":  fmt.Sprintf("%.2f", successRate),
		"AvgDNS":       fmt.Sprintf("%.2f", stats["dns"]),
		"AvgTCP":       fmt.Sprintf("%.2f", stats["tcp"]),
		"AvgSOCKS5":    fmt.Sprintf("%.2f", stats["socks5"]),
		"AvgTLS":       fmt.Sprintf("%.2f", stats["tls"]),
		"AvgTTFB":      fmt.Sprintf("%.2f", stats["ttfb"]),
		"AvgTotal":     fmt.Sprintf("%.2f", stats["total"]),
		"Metrics":      result.Metrics,
	}
}

func prepareBatchReportData(results []*tester.TestResult) BatchReportData {
	proxies := make([]ProxyData, len(results))

	var bestIdx, worstIdx int
	bestTotal := float64(^uint(0) >> 1) // max float
	worstTotal := 0.0

	for i, result := range results {
		stats := calculateAverages(result)
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
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 40px;
            color: white;
        }
        .header h1 {
            font-size: 32px;
            margin-bottom: 10px;
        }
        .header p {
            opacity: 0.9;
            font-size: 14px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            padding: 30px;
            background: #f8f9fa;
        }
        .stat-card {
            background: white;
            padding: 20px;
            border-radius: 12px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            transition: transform 0.2s;
        }
        .stat-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        }
        .stat-label {
            color: #6c757d;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 8px;
        }
        .stat-value {
            font-size: 28px;
            font-weight: bold;
            color: #667eea;
        }
        .stat-unit {
            font-size: 14px;
            color: #6c757d;
            margin-left: 4px;
        }
        .chart-section {
            padding: 30px;
        }
        .chart-container {
            position: relative;
            height: 400px;
            margin-top: 20px;
        }
        .section-title {
            font-size: 20px;
            font-weight: 600;
            color: #333;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #667eea;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Proxy Performance Report</h1>
            <p>Proxy: {{.ProxyName}} | Generated at: {{.GeneratedAt}}</p>
            <p>Target: {{.TargetURL}}</p>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Total Requests</div>
                <div class="stat-value">{{.TotalCount}}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Success Rate</div>
                <div class="stat-value">{{.SuccessRate}}<span class="stat-unit">%</span></div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Avg TTFB</div>
                <div class="stat-value">{{.AvgTTFB}}<span class="stat-unit">ms</span></div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Avg Total Time</div>
                <div class="stat-value">{{.AvgTotal}}<span class="stat-unit">ms</span></div>
            </div>
        </div>

        <div class="chart-section">
            <div class="section-title">⏱️ Latency Breakdown</div>
            <div class="chart-container">
                <canvas id="latencyChart"></canvas>
            </div>
        </div>
    </div>

    <script>
        const ctx = document.getElementById('latencyChart').getContext('2d');
        new Chart(ctx, {
            type: 'bar',
            data: {
                labels: ['DNS Lookup', 'TCP Connect', 'SOCKS5 Handshake', 'TLS Handshake', 'TTFB'],
                datasets: [{
                    label: 'Average Latency (ms)',
                    data: [{{.AvgDNS}}, {{.AvgTCP}}, {{.AvgSOCKS5}}, {{.AvgTLS}}, {{.AvgTTFB}}],
                    backgroundColor: [
                        'rgba(102, 126, 234, 0.8)',
                        'rgba(118, 75, 162, 0.8)',
                        'rgba(237, 100, 166, 0.8)',
                        'rgba(255, 154, 158, 0.8)',
                        'rgba(250, 208, 196, 0.8)'
                    ],
                    borderColor: [
                        'rgba(102, 126, 234, 1)',
                        'rgba(118, 75, 162, 1)',
                        'rgba(237, 100, 166, 1)',
                        'rgba(255, 154, 158, 1)',
                        'rgba(250, 208, 196, 1)'
                    ],
                    borderWidth: 2,
                    borderRadius: 8
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(0, 0, 0, 0.8)',
                        padding: 12,
                        titleFont: { size: 14 },
                        bodyFont: { size: 13 },
                        borderColor: 'rgba(102, 126, 234, 0.5)',
                        borderWidth: 1
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: 'rgba(0, 0, 0, 0.05)' },
                        ticks: {
                            callback: function(value) {
                                return value + ' ms';
                            }
                        }
                    },
                    x: {
                        grid: { display: false }
                    }
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
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 1600px;
            margin: 0 auto;
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 40px;
            color: white;
        }
        .header h1 {
            font-size: 36px;
            margin-bottom: 10px;
            display: flex;
            align-items: center;
            gap: 15px;
        }
        .header p {
            opacity: 0.9;
            font-size: 16px;
        }
        .content {
            padding: 40px;
        }
        .section-title {
            font-size: 24px;
            font-weight: 600;
            color: #333;
            margin-bottom: 25px;
            padding-bottom: 10px;
            border-bottom: 3px solid #667eea;
        }
        .chart-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(600px, 1fr));
            gap: 30px;
            margin-bottom: 40px;
        }
        .chart-container {
            background: #f8f9fa;
            padding: 25px;
            border-radius: 12px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .chart-container h3 {
            color: #667eea;
            margin-bottom: 20px;
            font-size: 18px;
        }
        .chart-wrapper {
            position: relative;
            height: 350px;
        }
        table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0;
            background: white;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        thead {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        th {
            padding: 16px;
            text-align: left;
            font-weight: 600;
            font-size: 13px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        td {
            padding: 16px;
            border-bottom: 1px solid #e9ecef;
        }
        tbody tr {
            transition: all 0.2s;
        }
        tbody tr:hover {
            background: #f8f9fa;
            transform: scale(1.01);
        }
        tbody tr:last-child td {
            border-bottom: none;
        }
        .proxy-name {
            font-weight: 600;
            color: #667eea;
        }
        .best-badge {
            display: inline-block;
            background: #10b981;
            color: white;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 11px;
            font-weight: 600;
            margin-left: 8px;
            text-transform: uppercase;
        }
        .worst-badge {
            display: inline-block;
            background: #ef4444;
            color: white;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 11px;
            font-weight: 600;
            margin-left: 8px;
            text-transform: uppercase;
        }
        .success-rate {
            display: inline-block;
            padding: 6px 12px;
            border-radius: 6px;
            font-weight: 600;
        }
        .success-high { background: #d1fae5; color: #065f46; }
        .success-medium { background: #fef3c7; color: #92400e; }
        .success-low { background: #fee2e2; color: #991b1b; }
        .metric-value {
            font-family: 'Monaco', 'Courier New', monospace;
            color: #4b5563;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 Batch Proxy Performance Report</h1>
            <p>Generated at: {{.GeneratedAt}} | Total Proxies Tested: {{.TotalProxies}}</p>
        </div>

        <div class="content">
            <div class="section-title">📈 Performance Charts</div>
            <div class="chart-grid">
                <div class="chart-container">
                    <h3>⚡ Average TTFB Comparison</h3>
                    <div class="chart-wrapper">
                        <canvas id="ttfbChart"></canvas>
                    </div>
                </div>
                <div class="chart-container">
                    <h3>⏱️ Total Latency Comparison</h3>
                    <div class="chart-wrapper">
                        <canvas id="totalChart"></canvas>
                    </div>
                </div>
            </div>

            <div class="section-title">📋 Detailed Comparison Table</div>
            <table>
                <thead>
                    <tr>
                        <th>Proxy Name</th>
                        <th>Success Rate</th>
                        <th>DNS (ms)</th>
                        <th>TCP (ms)</th>
                        <th>SOCKS5 (ms)</th>
                        <th>TLS (ms)</th>
                        <th>TTFB (ms)</th>
                        <th>Total (ms)</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Proxies}}
                    <tr>
                        <td class="proxy-name">
                            {{.Name}}
                            {{if .IsBest}}<span class="best-badge">⭐ Best</span>{{end}}
                            {{if .IsWorst}}<span class="worst-badge">⚠️ Slow</span>{{end}}
                        </td>
                        <td>
                            <span class="success-rate {{if ge .SuccessRate 95.0}}success-high{{else if ge .SuccessRate 80.0}}success-medium{{else}}success-low{{end}}">
                                {{printf "%.1f" .SuccessRate}}%
                            </span>
                        </td>
                        <td class="metric-value">{{printf "%.2f" .AvgDNS}}</td>
                        <td class="metric-value">{{printf "%.2f" .AvgTCP}}</td>
                        <td class="metric-value">{{printf "%.2f" .AvgSOCKS5}}</td>
                        <td class="metric-value">{{printf "%.2f" .AvgTLS}}</td>
                        <td class="metric-value">{{printf "%.2f" .AvgTTFB}}</td>
                        <td class="metric-value"><strong>{{printf "%.2f" .AvgTotal}}</strong></td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>

    <script>
        const proxyNames = [{{range .Proxies}}'{{.Name}}',{{end}}];
        const ttfbData = [{{range .Proxies}}{{.AvgTTFB}},{{end}}];
        const totalData = [{{range .Proxies}}{{.AvgTotal}},{{end}}];

        const colors = [
            'rgba(102, 126, 234, 0.8)',
            'rgba(118, 75, 162, 0.8)',
            'rgba(237, 100, 166, 0.8)',
            'rgba(255, 154, 158, 0.8)',
            'rgba(250, 208, 196, 0.8)',
            'rgba(72, 187, 120, 0.8)',
            'rgba(99, 179, 237, 0.8)',
        ];

        // TTFB Chart
        new Chart(document.getElementById('ttfbChart'), {
            type: 'bar',
            data: {
                labels: proxyNames,
                datasets: [{
                    label: 'TTFB (ms)',
                    data: ttfbData,
                    backgroundColor: colors,
                    borderWidth: 0,
                    borderRadius: 8
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(0, 0, 0, 0.8)',
                        padding: 12,
                        borderColor: 'rgba(102, 126, 234, 0.5)',
                        borderWidth: 1
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: 'rgba(0, 0, 0, 0.05)' },
                        ticks: { callback: value => value + ' ms' }
                    },
                    x: { grid: { display: false } }
                }
            }
        });

        // Total Latency Chart
        new Chart(document.getElementById('totalChart'), {
            type: 'bar',
            data: {
                labels: proxyNames,
                datasets: [{
                    label: 'Total Latency (ms)',
                    data: totalData,
                    backgroundColor: colors,
                    borderWidth: 0,
                    borderRadius: 8
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: 'rgba(0, 0, 0, 0.8)',
                        padding: 12,
                        borderColor: 'rgba(102, 126, 234, 0.5)',
                        borderWidth: 1
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: 'rgba(0, 0, 0, 0.05)' },
                        ticks: { callback: value => value + ' ms' }
                    },
                    x: { grid: { display: false } }
                }
            }
        });
    </script>
</body>
</html>`
