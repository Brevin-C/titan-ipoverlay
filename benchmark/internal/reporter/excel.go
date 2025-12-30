package reporter

import (
	"fmt"
	"time"
	"titan-ipoverlay/benchmark/internal/tester"

	"github.com/xuri/excelize/v2"
)

// ExcelReporter generates Excel reports from test results
type ExcelReporter struct {
	file *excelize.File
}

// NewExcelReporter creates a new Excel reporter
func NewExcelReporter() *ExcelReporter {
	return &ExcelReporter{
		file: excelize.NewFile(),
	}
}

// GenerateReport creates a comprehensive Excel report
func (r *ExcelReporter) GenerateReport(results []*tester.TestResult, outputPath string) error {
	// Delete default Sheet1
	r.file.DeleteSheet("Sheet1")

	// Create summary sheet
	if err := r.createSummarySheet(results); err != nil {
		return fmt.Errorf("failed to create summary sheet: %w", err)
	}

	// Create individual test sheets
	for i, result := range results {
		sheetName := fmt.Sprintf("测试%d_%s", i+1, result.ProxyName)
		if len(sheetName) > 31 { // Excel sheet name limit
			sheetName = sheetName[:31]
		}
		if err := r.createDetailSheet(sheetName, *result); err != nil {
			return fmt.Errorf("failed to create detail sheet: %w", err)
		}
	}

	// Create comparison sheet if we have multiple results
	if len(results) >= 2 {
		if err := r.createComparisonSheet(results); err != nil {
			return fmt.Errorf("failed to create comparison sheet: %w", err)
		}
	}

	// Save file
	if err := r.file.SaveAs(outputPath); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	return nil
}

// createSummarySheet creates the summary overview sheet
func (r *ExcelReporter) createSummarySheet(results []*tester.TestResult) error {
	sheetName := "测试概览"
	index, err := r.file.NewSheet(sheetName)
	if err != nil {
		return err
	}
	r.file.SetActiveSheet(index)

	// Set column widths
	r.file.SetColWidth(sheetName, "A", "A", 20)
	r.file.SetColWidth(sheetName, "B", "F", 15)

	// Header
	headers := []string{"测试名称", "代理名称", "总请求数", "成功数", "成功率(%)", "平均延迟(ms)"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		r.file.SetCellValue(sheetName, cell, header)
	}

	// Style header
	headerStyle, _ := r.file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	r.file.SetCellStyle(sheetName, "A1", fmt.Sprintf("%c1", 'A'+len(headers)-1), headerStyle)

	// Data rows
	for i, result := range results {
		row := i + 2
		stats := tester.CalculateAllStats(result)
		avgLatency := float64(stats["total"].Mean.Milliseconds())
		successRate := tester.CalculateSuccessRate(result)

		r.file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), result.TestName)
		r.file.SetCellValue(sheetName, fmt.Sprintf("B%d", row), result.ProxyName)
		r.file.SetCellValue(sheetName, fmt.Sprintf("C%d", row), result.TotalCount)
		r.file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), result.SuccessCount)
		r.file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%.2f", successRate))
		r.file.SetCellValue(sheetName, fmt.Sprintf("F%d", row), fmt.Sprintf("%.2f", avgLatency))
	}

	return nil
}

// createDetailSheet creates a detailed sheet for a single test result
func (r *ExcelReporter) createDetailSheet(sheetName string, result tester.TestResult) error {
	_, err := r.file.NewSheet(sheetName)
	if err != nil {
		return err
	}

	// Statistics section
	stats := tester.CalculateAllStats(&result)

	headers := []string{"指标", "平均值(ms)", "中位数/P50(ms)", "P95(ms)", "P99(ms)", "最小值(ms)", "最大值(ms)"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		r.file.SetCellValue(sheetName, cell, header)
	}

	metricNames := map[string]string{
		"dns":    "DNS解析",
		"tcp":    "TCP连接",
		"socks5": "SOCKS5握手",
		"tls":    "TLS握手",
		"ttfb":   "首字节时间",
		"total":  "总延迟",
	}

	row := 2
	for _, metricKey := range []string{"dns", "tcp", "socks5", "tls", "ttfb", "total"} {
		stat := stats[metricKey]
		r.file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), metricNames[metricKey])
		r.file.SetCellValue(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("%.2f", float64(stat.Mean.Microseconds())/1000.0))
		r.file.SetCellValue(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("%.2f", float64(stat.Median.Microseconds())/1000.0))
		r.file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), fmt.Sprintf("%.2f", float64(stat.P95.Microseconds())/1000.0))
		r.file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%.2f", float64(stat.P99.Microseconds())/1000.0))
		r.file.SetCellValue(sheetName, fmt.Sprintf("F%d", row), fmt.Sprintf("%.2f", float64(stat.Min.Microseconds())/1000.0))
		r.file.SetCellValue(sheetName, fmt.Sprintf("G%d", row), fmt.Sprintf("%.2f", float64(stat.Max.Microseconds())/1000.0))
		row++
	}

	// Test info
	r.file.SetCellValue(sheetName, "A9", "测试信息")
	r.file.SetCellValue(sheetName, "A10", "目标URL:")
	r.file.SetCellValue(sheetName, "B10", result.TargetURL)
	r.file.SetCellValue(sheetName, "A11", "总请求数:")
	r.file.SetCellValue(sheetName, "B11", result.TotalCount)
	r.file.SetCellValue(sheetName, "A12", "成功数:")
	r.file.SetCellValue(sheetName, "B12", result.SuccessCount)
	r.file.SetCellValue(sheetName, "A13", "失败数:")
	r.file.SetCellValue(sheetName, "B13", result.FailedCount)
	r.file.SetCellValue(sheetName, "A14", "成功率:")
	r.file.SetCellValue(sheetName, "B14", fmt.Sprintf("%.2f%%", tester.CalculateSuccessRate(&result)))

	return nil
}

// createComparisonSheet creates a comparison sheet between different proxy results
func (r *ExcelReporter) createComparisonSheet(results []*tester.TestResult) error {
	sheetName := "对比分析"
	_, err := r.file.NewSheet(sheetName)
	if err != nil {
		return err
	}

	// Set column widths
	r.file.SetColWidth(sheetName, "A", "A", 20)
	r.file.SetColWidth(sheetName, "B", "F", 15)

	// Title
	r.file.SetCellValue(sheetName, "A1", "性能对比分析")
	titleStyle, _ := r.file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})
	r.file.SetCellStyle(sheetName, "A1", "A1", titleStyle)

	// Headers
	row := 3
	r.file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "指标")
	for i, result := range results {
		col := string(rune('B' + i))
		r.file.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, row), result.ProxyName)
	}

	// Add difference columns if comparing two proxies
	if len(results) == 2 {
		r.file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), "差异(ms)")
		r.file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), "差异比(%)")
	}

	// Metric rows
	metricNames := map[string]string{
		"dns":    "DNS解析(ms)",
		"tcp":    "TCP连接(ms)",
		"socks5": "SOCKS5握手(ms)",
		"tls":    "TLS握手(ms)",
		"ttfb":   "首字节时间(ms)",
		"total":  "总延迟(ms)",
	}

	row = 4
	for _, metricKey := range []string{"dns", "tcp", "socks5", "tls", "ttfb", "total"} {
		r.file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), metricNames[metricKey])

		var values []float64
		for i, result := range results {
			stats := tester.CalculateAllStats(result)
			value := float64(stats[metricKey].Mean.Microseconds()) / 1000.0
			values = append(values, value)

			col := string(rune('B' + i))
			r.file.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%.2f", value))
		}

		// Calculate difference if comparing two proxies
		if len(results) == 2 && len(values) == 2 {
			diff := values[0] - values[1]
			diffPct := 0.0
			if values[1] != 0 {
				diffPct = (diff / values[1]) * 100.0
			}

			r.file.SetCellValue(sheetName, fmt.Sprintf("D%d", row), fmt.Sprintf("%.2f", diff))
			r.file.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%.2f", diffPct))

			// Color code the difference
			if diff > 0 {
				// Slower - red background
				style, _ := r.file.NewStyle(&excelize.Style{
					Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFcccc"}, Pattern: 1},
				})
				r.file.SetCellStyle(sheetName, fmt.Sprintf("D%d", row), fmt.Sprintf("E%d", row), style)
			} else {
				// Faster - green background
				style, _ := r.file.NewStyle(&excelize.Style{
					Fill: excelize.Fill{Type: "pattern", Color: []string{"#ccFFcc"}, Pattern: 1},
				})
				r.file.SetCellStyle(sheetName, fmt.Sprintf("D%d", row), fmt.Sprintf("E%d", row), style)
			}
		}

		row++
	}

	// Success rate comparison
	row++
	r.file.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "成功率(%)")
	for i, result := range results {
		col := string(rune('B' + i))
		successRate := tester.CalculateSuccessRate(result)
		r.file.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%.2f", successRate))
	}

	return nil
}

// FormatDuration formats a duration as milliseconds with 2 decimal places
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2f", float64(d.Microseconds())/1000.0)
}
