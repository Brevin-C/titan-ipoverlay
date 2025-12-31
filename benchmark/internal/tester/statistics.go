package tester

import (
	"sort"
	"time"
)

// CalculateStats computes statistical metrics from latency data
func CalculateStats(durations []time.Duration) *Stats {
	if len(durations) == 0 {
		return &Stats{}
	}

	// Sort durations for percentile calculation
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate statistics
	stats := &Stats{
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Median: percentile(sorted, 50),
		P95:    percentile(sorted, 95),
		P99:    percentile(sorted, 99),
	}

	// Calculate mean
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	stats.Mean = time.Duration(int64(sum) / int64(len(durations)))

	return stats
}

// percentile calculates the nth percentile from sorted durations
func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	// Use linear interpolation
	rank := float64(p) / 100.0 * float64(len(sorted)-1)
	lowerIndex := int(rank)
	upperIndex := lowerIndex + 1

	if upperIndex >= len(sorted) {
		return sorted[lowerIndex]
	}

	// Interpolate between two values
	fraction := rank - float64(lowerIndex)
	lower := float64(sorted[lowerIndex])
	upper := float64(sorted[upperIndex])
	result := lower + fraction*(upper-lower)

	return time.Duration(result)
}

// CalculateSuccessRate returns the success rate as a percentage
func CalculateSuccessRate(result *TestResult) float64 {
	if result.TotalCount == 0 {
		return 0.0
	}
	return float64(result.SuccessCount) / float64(result.TotalCount) * 100.0
}

// ExtractMetricDurations extracts a specific metric from all results
func ExtractMetricDurations(metrics []LatencyMetrics, metricType string) []time.Duration {
	durations := make([]time.Duration, 0, len(metrics))

	for _, m := range metrics {
		if !m.Success {
			continue // Skip failed requests
		}

		var duration time.Duration
		switch metricType {
		case "proxy_dns":
			duration = m.ProxyDNS
		case "proxy_tcp":
			duration = m.ProxyTCP
		case "socks5":
			duration = m.SOCKS5Handshake
		case "dns":
			duration = m.DNSLookup
		case "tcp":
			duration = m.TCPConnect
		case "tls":
			duration = m.TLSHandshake
		case "ttfb":
			duration = m.TTFB
		case "total":
			duration = m.TotalTime
		default:
			continue
		}

		durations = append(durations, duration)
	}

	return durations
}

// CalculateAllStats calculates statistics for all metric types
func CalculateAllStats(result *TestResult) map[string]*Stats {
	statsMap := make(map[string]*Stats)

	metricTypes := []string{"proxy_dns", "proxy_tcp", "socks5", "dns", "tcp", "tls", "ttfb", "total"}

	for _, metricType := range metricTypes {
		durations := ExtractMetricDurations(result.Metrics, metricType)
		statsMap[metricType] = CalculateStats(durations)
	}

	return statsMap
}

// CompareTwoResults creates a comparison between Titan and competitor results
func CompareTwoResults(titanResult, competitorResult *TestResult) *ComparisonResult {
	comparison := &ComparisonResult{
		TitanResult:      titanResult,
		CompetitorResult: competitorResult,
		TitanStats:       CalculateAllStats(titanResult),
		CompetitorStats:  CalculateAllStats(competitorResult),
		Differences:      make(map[string]Difference),
	}

	// Calculate differences for each metric
	for metricType := range comparison.TitanStats {
		titanMean := comparison.TitanStats[metricType].Mean
		compMean := comparison.CompetitorStats[metricType].Mean

		difference := Difference{
			Absolute: titanMean - compMean,
		}

		if compMean > 0 {
			difference.Percentage = float64(titanMean-compMean) / float64(compMean) * 100.0
		}

		comparison.Differences[metricType] = difference
	}

	return comparison
}
