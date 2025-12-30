package tester

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// SingleTester performs "sequential" sampling but with low concurrency for speed
type SingleTester struct {
	client   *HTTPClient
	interval time.Duration
	workers  int
}

// NewSingleTester creates a new single request tester
func NewSingleTester(client *HTTPClient, interval time.Duration) *SingleTester {
	return &SingleTester{
		client:   client,
		interval: interval,
		workers:  10, // Increased default workers to speed up "sequential" sampling
	}
}

// RunTest executes N requests using a small worker pool to speed up collection
func (st *SingleTester) RunTest(ctx context.Context, testName, targetURL string, count int) (*TestResult, error) {
	result := &TestResult{
		TestName:   testName,
		ProxyName:  st.client.proxyName,
		TargetURL:  targetURL,
		TotalCount: count,
		Metrics:    make([]LatencyMetrics, count),
		StartTime:  time.Now(),
	}

	fmt.Printf("开始单次请求测试: %s\n", testName)
	fmt.Printf("  目标URL: %s\n", targetURL)
	fmt.Printf("  请求次数: %d (并发池大小: %d)\n", count, st.workers)
	fmt.Printf("  代理: %s\n\n", st.client.proxyName)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		semaphore = make(chan struct{}, st.workers)
	)

	successCount := 0
	failedCount := 0

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			metrics, err := st.client.MakeRequest(ctx, targetURL)

			mu.Lock()
			result.Metrics[index] = *metrics
			if err == nil && metrics.Success {
				successCount++
			} else {
				failedCount++
				if failedCount <= 5 {
					errMsg := metrics.Error
					if errMsg == "" && err != nil {
						errMsg = err.Error()
					}
					fmt.Fprintf(os.Stderr, "  [详细错误] 请求 #%d 失败: %s\n", index+1, errMsg)
				}
			}

			completed := successCount + failedCount
			// Report progress more frequently (every 20 or 5%, whichever is smaller)
			reportFreq := count / 20
			if reportFreq < 10 {
				reportFreq = 10
			}
			if completed%reportFreq == 0 || completed == count {
				fmt.Printf("  进度: %d/%d (成功: %d, 失败: %d)\n",
					completed, count, successCount, failedCount)
			}
			mu.Unlock()

			if st.interval > 0 {
				time.Sleep(st.interval)
			}
		}(i)
	}

	wg.Wait()

	result.SuccessCount = successCount
	result.FailedCount = failedCount
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	fmt.Printf("\n测试完成!\n")
	fmt.Printf("  总耗时: %v\n", result.Duration)
	fmt.Printf("  成功率: %.2f%%\n", CalculateSuccessRate(result))
	fmt.Printf("  吞吐量: %.2f req/s\n\n", float64(count)/result.Duration.Seconds())

	return result, nil
}

// ConcurrentTester performs concurrent request testing
type ConcurrentTester struct {
	client      *HTTPClient
	concurrency int
}

// NewConcurrentTester creates a new concurrent tester
func NewConcurrentTester(client *HTTPClient, concurrency int) *ConcurrentTester {
	return &ConcurrentTester{
		client:      client,
		concurrency: concurrency,
	}
}

// RunTest executes concurrent requests and collects metrics
func (ct *ConcurrentTester) RunTest(ctx context.Context, testName, targetURL string, count int) (*TestResult, error) {
	result := &TestResult{
		TestName:   testName,
		ProxyName:  ct.client.proxyName,
		TargetURL:  targetURL,
		TotalCount: count,
		Metrics:    make([]LatencyMetrics, count),
		StartTime:  time.Now(),
	}

	fmt.Printf("开始并发测试: %s\n", testName)
	fmt.Printf("  目标URL: %s\n", targetURL)
	fmt.Printf("  并发数: %d\n", ct.concurrency)
	fmt.Printf("  总请求数: %d\n", count)
	fmt.Printf("  代理: %s\n\n", ct.client.proxyName)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		semaphore = make(chan struct{}, ct.concurrency)
	)

	successCount := 0
	failedCount := 0

	// Launch concurrent requests
	for i := 0; i < count; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Make request
			metrics, err := ct.client.MakeRequest(ctx, targetURL)

			// Store results with mutex protection
			mu.Lock()
			result.Metrics[index] = *metrics
			if err == nil {
				successCount++
			} else {
				failedCount++
			}

			// Progress reporting
			completed := successCount + failedCount
			if completed%50 == 0 || completed == count {
				fmt.Printf("  进度: %d/%d (成功: %d, 失败: %d)\n",
					completed, count, successCount, failedCount)
			}
			mu.Unlock()
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()

	result.SuccessCount = successCount
	result.FailedCount = failedCount
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Calculate throughput
	throughput := float64(count) / result.Duration.Seconds()

	fmt.Printf("\n测试完成!\n")
	fmt.Printf("  总耗时: %v\n", result.Duration)
	fmt.Printf("  成功率: %.2f%%\n", CalculateSuccessRate(result))
	fmt.Printf("  吞吐量: %.2f req/s\n\n", throughput)

	return result, nil
}
