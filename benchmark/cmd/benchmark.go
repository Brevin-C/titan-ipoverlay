package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"titan-ipoverlay/benchmark/internal/config"
	"titan-ipoverlay/benchmark/internal/exporter"
	"titan-ipoverlay/benchmark/internal/reporter"
	"titan-ipoverlay/benchmark/internal/tester"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "ip-proxy-benchmark",
		Usage: "IP代理性能测试工具",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "configs/bench_config.yaml",
				Usage:   "配置文件路径",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "reports/benchmark_report.xlsx",
				Usage:   "输出Excel报告路径",
			},
			&cli.StringFlag{
				Name:  "proxy",
				Value: "titan",
				Usage: "要测试的代理名称（在配置文件中定义）",
			},
			&cli.BoolFlag{
				Name:  "test-all-proxies",
				Value: false,
				Usage: "测试配置文件中的所有代理（批量模式）",
			},
			&cli.StringFlag{
				Name:  "target",
				Value: "",
				Usage: "要测试的目标URL（覆盖配置文件中的第一个目标）",
			},
			&cli.StringFlag{
				Name:  "mode",
				Value: "all",
				Usage: "测试模式: single, concurrent, all",
			},
			&cli.IntFlag{
				Name:  "count",
				Value: 0,
				Usage: "请求数量（覆盖配置文件）",
			},
			&cli.IntFlag{
				Name:  "concurrency",
				Value: 0,
				Usage: "并发数（覆盖配置文件）",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Value:   false,
				Usage:   "显示详细日志",
			},
			&cli.StringSliceFlag{
				Name:    "export-formats",
				Aliases: []string{"e"},
				Value:   cli.NewStringSlice("csv", "json", "html"),
				Usage:   "导出格式: csv, json, html (可以多选，用逗号分隔)",
			},
			&cli.StringFlag{
				Name:  "export-dir",
				Value: "reports",
				Usage: "导出目录路径",
			},
		},
		Action: runBenchmark,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func runBenchmark(c *cli.Context) error {
	// Load configuration
	cfg, err := config.LoadConfig(c.String("config"))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine target URL
	targetURL := c.String("target")
	if targetURL != "" {
		// Check if it's a target name from config
		for _, t := range cfg.Targets {
			if t.Name == targetURL {
				targetURL = t.URL
				break
			}
		}
	} else {
		if len(cfg.Targets) == 0 {
			return fmt.Errorf("no targets defined in configuration")
		}
		targetURL = cfg.Targets[0].URL
	}

	// Parse timeout
	timeout, err := time.ParseDuration(cfg.Settings.RequestTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Parse request interval
	interval, err := time.ParseDuration(cfg.Settings.RequestInterval)
	if err != nil {
		return fmt.Errorf("invalid request interval: %w", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n收到中断信号，正在停止测试...")
		cancel()
	}()

	// Determine which proxies to test
	var proxyNames []string
	if c.Bool("test-all-proxies") {
		// Test all proxies in config
		for name := range cfg.Proxies {
			proxyNames = append(proxyNames, name)
		}
		fmt.Printf("\n========================================\n")
		fmt.Printf("🚀 批量代理测试模式\n")
		fmt.Printf("========================================\n")
		fmt.Printf("将测试 %d 个代理节点\n", len(proxyNames))
		fmt.Printf("目标: %s\n", targetURL)
		fmt.Printf("========================================\n\n")
	} else {
		// Test single proxy
		proxyName := c.String("proxy")
		if _, ok := cfg.Proxies[proxyName]; !ok {
			return fmt.Errorf("proxy '%s' not found in configuration", proxyName)
		}
		proxyNames = []string{proxyName}
	}

	// Collect results from all proxies
	var allResults []*tester.TestResult

	// Test each proxy
	for proxyIndex, proxyName := range proxyNames {
		proxyConfig := cfg.Proxies[proxyName]

		fmt.Printf("\n========================================\n")
		if c.Bool("test-all-proxies") {
			fmt.Printf("正在测试代理 [%d/%d]: %s\n", proxyIndex+1, len(proxyNames), proxyConfig.Name)
		} else {
			fmt.Printf("IP代理性能测试工具\n")
		}
		fmt.Printf("========================================\n")
		fmt.Printf("代理: %s (%s)\n", proxyConfig.Name, proxyConfig.Socks5)
		fmt.Printf("目标: %s\n", targetURL)
		fmt.Printf("========================================\n\n")

		// Create HTTP client for this proxy
		httpClient, err := tester.NewHTTPClient(
			proxyConfig.Socks5,
			proxyConfig.Name,
			proxyConfig.Username,
			proxyConfig.Password,
			timeout,
		)
		if err != nil {
			fmt.Printf("⚠️  跳过代理 %s: 创建客户端失败: %v\n\n", proxyConfig.Name, err)
			continue
		}

		// Test scenarios for this proxy
		mode := c.String("mode")
		scenarios := cfg.GetEnabledScenarios()

		for _, scenario := range scenarios {
			// Skip if mode doesn't match
			if mode != "all" {
				if mode == "single" && scenario.Type != "single" {
					continue
				}
				if mode == "concurrent" && scenario.Type != "concurrent" {
					continue
				}
			}

			// Override count if specified in CLI
			count := scenario.Count
			if c.Int("count") > 0 {
				count = c.Int("count")
			}

			// Override concurrency if specified in CLI
			concurrency := scenario.Concurrency
			if c.Int("concurrency") > 0 {
				concurrency = c.Int("concurrency")
			}

			var result *tester.TestResult

			if scenario.Type == "single" {
				// Run single request test
				singleTester := tester.NewSingleTester(httpClient, interval)
				result, err = singleTester.RunTest(ctx, scenario.Name, targetURL, count)
			} else if scenario.Type == "concurrent" {
				// Run concurrent test
				concurrentTester := tester.NewConcurrentTester(httpClient, concurrency)
				result, err = concurrentTester.RunTest(ctx, scenario.Name, targetURL, count)
			}

			if err != nil {
				if err == context.Canceled {
					fmt.Println("测试被用户取消")
					goto GENERATE_REPORT
				}
				fmt.Printf("⚠️  测试失败: %v\n", err)
				continue
			}

			if result != nil {
				allResults = append(allResults, result)
			}

			// Small delay between tests
			time.Sleep(1 * time.Second)
		}

		// Delay between different proxies
		if proxyIndex < len(proxyNames)-1 {
			fmt.Printf("\n⏳ 等待2秒后测试下一个代理...\n")
			time.Sleep(2 * time.Second)
		}
	}

GENERATE_REPORT:
	if len(allResults) == 0 {
		return fmt.Errorf("no test results collected")
	}

	// Generate Excel report
	fmt.Printf("\n========================================\n")
	fmt.Printf("📊 生成Excel报告...\n")
	fmt.Printf("========================================\n")

	excelReporter := reporter.NewExcelReporter()
	outputPath := c.String("output")

	// Ensure output directory exists
	exportDir := c.String("export-dir")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := excelReporter.GenerateReport(allResults, outputPath); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	fmt.Printf("✓ 报告已生成: %s\n", outputPath)

	// Export to additional formats if requested
	exportFormatsRaw := c.StringSlice("export-formats")
	if len(exportFormatsRaw) > 0 {
		fmt.Printf("\n========================================\n")
		fmt.Printf("📤 导出测试结果...\n")
		fmt.Printf("========================================\n")

		// Parse export formats
		var exportFormats []exporter.ExportFormat
		for _, format := range exportFormatsRaw {
			format = strings.ToLower(strings.TrimSpace(format))
			switch format {
			case "csv":
				exportFormats = append(exportFormats, exporter.FormatCSV)
			case "json":
				exportFormats = append(exportFormats, exporter.FormatJSON)
			case "html":
				exportFormats = append(exportFormats, exporter.FormatHTML)
			}
		}

		exp := exporter.NewExporter(exportDir)
		if c.Bool("test-all-proxies") {
			// Export batch results
			if err := exp.ExportBatch(allResults, exportFormats); err != nil {
				fmt.Printf("⚠️  导出失败: %v\n", err)
			}
		} else {
			// Export individual results
			for _, result := range allResults {
				if err := exp.Export(result, exportFormats); err != nil {
					fmt.Printf("⚠️  导出 %s 失败: %v\n", result.ProxyName, err)
				}
			}
		}
	}
	if c.Bool("test-all-proxies") {
		fmt.Printf("\n🎉 批量测试完成! 共测试 %d 个代理，执行 %d 个测试场景\n\n", len(proxyNames), len(allResults))
	} else {
		fmt.Printf("\n测试完成! 共执行 %d 个测试场景\n\n", len(allResults))
	}

	return nil
}
