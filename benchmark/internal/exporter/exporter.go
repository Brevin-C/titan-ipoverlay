package exporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"titan-ipoverlay/benchmark/internal/tester"
)

// ExportFormat represents the export file format
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatJSON ExportFormat = "json"
	FormatHTML ExportFormat = "html"
)

// Exporter handles exporting test results to various formats
type Exporter struct {
	outputDir string
}

// NewExporter creates a new exporter instance
func NewExporter(outputDir string) *Exporter {
	return &Exporter{
		outputDir: outputDir,
	}
}

// Export exports the test results to the specified formats
func (e *Exporter) Export(result *tester.TestResult, formats []ExportFormat) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("%s_%s", result.ProxyName, timestamp)

	for _, format := range formats {
		var err error
		switch format {
		case FormatCSV:
			err = e.exportCSV(result, baseName)
		case FormatJSON:
			err = e.exportJSON(result, baseName)
		case FormatHTML:
			err = e.exportHTML(result, baseName)
		default:
			return fmt.Errorf("unsupported export format: %s", format)
		}
		if err != nil {
			return fmt.Errorf("failed to export as %s: %w", format, err)
		}
	}

	return nil
}

// exportCSV exports results to CSV format
func (e *Exporter) exportCSV(result *tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp",
		"Proxy Name",
		"Target URL",
		"Success",
		"Status Code",
		"Proxy DNS (ms)", // New: Proxy DNS resolution
		"Proxy TCP (ms)", // New: TCP to proxy server
		"SOCKS5 Handshake (ms)",
		"Target DNS (ms)", // Renamed for clarity
		"Target TCP (ms)", // Renamed for clarity
		"TLS Handshake (ms)",
		"TTFB (ms)",
		"Total Time (ms)",
		"Error",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data rows
	for _, metric := range result.Metrics {
		row := []string{
			result.StartTime.Format(time.RFC3339),
			result.ProxyName,
			result.TargetURL,
			fmt.Sprintf("%t", metric.Success),
			fmt.Sprintf("%d", metric.StatusCode),
			fmt.Sprintf("%.2f", float64(metric.ProxyDNS.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.ProxyTCP.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.SOCKS5Handshake.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.DNSLookup.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.TCPConnect.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.TLSHandshake.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.TTFB.Microseconds())/1000.0),
			fmt.Sprintf("%.2f", float64(metric.TotalTime.Microseconds())/1000.0),
			metric.Error,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("✓ CSV report exported to: %s\n", filename)
	return nil
}

// exportJSON exports results to JSON format
func (e *Exporter) exportJSON(result *tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".json")

	// Create a more structured JSON output
	output := map[string]interface{}{
		"test_info": map[string]interface{}{
			"test_name":  result.TestName,
			"proxy_name": result.ProxyName,
			"target_url": result.TargetURL,
			"start_time": result.StartTime.Format(time.RFC3339),
			"end_time":   result.EndTime.Format(time.RFC3339),
			"duration":   result.Duration.String(),
		},
		"summary": map[string]interface{}{
			"total_requests":      result.TotalCount,
			"successful_requests": result.SuccessCount,
			"failed_requests":     result.FailedCount,
			"success_rate":        fmt.Sprintf("%.2f%%", float64(result.SuccessCount)/float64(result.TotalCount)*100),
		},
		"metrics": result.Metrics,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	fmt.Printf("✓ JSON report exported to: %s\n", filename)
	return nil
}

// ExportBatch exports multiple test results with comparison
func (e *Exporter) ExportBatch(results []*tester.TestResult, formats []ExportFormat) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("batch_report_%s", timestamp)

	for _, format := range formats {
		var err error
		switch format {
		case FormatCSV:
			err = e.exportBatchCSV(results, baseName)
		case FormatJSON:
			err = e.exportBatchJSON(results, baseName)
		case FormatHTML:
			err = e.exportBatchHTML(results, baseName)
		default:
			return fmt.Errorf("unsupported export format: %s", format)
		}
		if err != nil {
			return fmt.Errorf("failed to export batch as %s: %w", format, err)
		}
	}

	return nil
}

// exportBatchCSV exports batch results to CSV
func (e *Exporter) exportBatchCSV(results []*tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Proxy Name",
		"Target URL",
		"Total Requests",
		"Success Count",
		"Failed Count",
		"Success Rate %",
		"Avg DNS (ms)",
		"Avg TCP (ms)",
		"Avg SOCKS5 (ms)",
		"Avg TLS (ms)",
		"Avg TTFB (ms)",
		"Avg Total (ms)",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data for each proxy
	for _, result := range results {
		stats := calculateAverages(result)
		row := []string{
			result.ProxyName,
			result.TargetURL,
			fmt.Sprintf("%d", result.TotalCount),
			fmt.Sprintf("%d", result.SuccessCount),
			fmt.Sprintf("%d", result.FailedCount),
			fmt.Sprintf("%.2f", float64(result.SuccessCount)/float64(result.TotalCount)*100),
			fmt.Sprintf("%.2f", stats["dns"]),
			fmt.Sprintf("%.2f", stats["tcp"]),
			fmt.Sprintf("%.2f", stats["socks5"]),
			fmt.Sprintf("%.2f", stats["tls"]),
			fmt.Sprintf("%.2f", stats["ttfb"]),
			fmt.Sprintf("%.2f", stats["total"]),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Batch CSV report exported to: %s\n", filename)
	return nil
}

// exportBatchJSON exports batch results to JSON
func (e *Exporter) exportBatchJSON(results []*tester.TestResult, baseName string) error {
	filename := filepath.Join(e.outputDir, baseName+".json")

	output := map[string]interface{}{
		"report_info": map[string]interface{}{
			"generated_at":  time.Now().Format(time.RFC3339),
			"total_proxies": len(results),
		},
		"results": results,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Batch JSON report exported to: %s\n", filename)
	return nil
}

// calculateAverages calculates average latencies from test result
func calculateAverages(result *tester.TestResult) map[string]float64 {
	if result.SuccessCount == 0 {
		return map[string]float64{
			"proxy_dns": 0, "proxy_tcp": 0, "socks5": 0, "dns": 0, "tcp": 0, "tls": 0, "ttfb": 0, "proc": 0, "total": 0,
		}
	}

	var sumProxyDNS, sumProxyTCP, sumSOCKS5, sumDNS, sumTCP, sumTLS, sumTTFB, sumTotal int64
	count := 0

	for _, m := range result.Metrics {
		if m.Success {
			sumProxyDNS += m.ProxyDNS.Microseconds()
			sumProxyTCP += m.ProxyTCP.Microseconds()
			sumSOCKS5 += m.SOCKS5Handshake.Microseconds()
			sumDNS += m.DNSLookup.Microseconds()
			sumTCP += m.TCPConnect.Microseconds()
			sumTLS += m.TLSHandshake.Microseconds()
			sumTTFB += m.TTFB.Microseconds()
			sumTotal += m.TotalTime.Microseconds()
			count++
		}
	}

	if count == 0 {
		return map[string]float64{
			"proxy_dns": 0, "proxy_tcp": 0, "socks5": 0, "dns": 0, "tcp": 0, "tls": 0, "ttfb": 0, "proc": 0, "total": 0,
		}
	}

	avgProxyDNS := float64(sumProxyDNS) / float64(count) / 1000.0
	avgProxyTCP := float64(sumProxyTCP) / float64(count) / 1000.0
	avgSOCKS5 := float64(sumSOCKS5) / float64(count) / 1000.0
	avgDNS := float64(sumDNS) / float64(count) / 1000.0
	avgTCP := float64(sumTCP) / float64(count) / 1000.0
	avgTLS := float64(sumTLS) / float64(count) / 1000.0
	avgTTFB := float64(sumTTFB) / float64(count) / 1000.0
	avgTotal := float64(sumTotal) / float64(count) / 1000.0

	// Server Processing = TTFB - (Proxy DNS + Proxy TCP + SOCKS5 + Target DNS + Target TCP + TLS)
	avgProc := avgTTFB - (avgProxyDNS + avgProxyTCP + avgSOCKS5 + avgDNS + avgTCP + avgTLS)
	if avgProc < 0 {
		avgProc = 0
	}

	return map[string]float64{
		"proxy_dns": avgProxyDNS,
		"proxy_tcp": avgProxyTCP,
		"socks5":    avgSOCKS5,
		"dns":       avgDNS,
		"tcp":       avgTCP,
		"tls":       avgTLS,
		"ttfb":      avgTTFB,
		"proc":      avgProc,
		"total":     avgTotal,
	}
}
