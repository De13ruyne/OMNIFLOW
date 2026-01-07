package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func main() {
	// 🔥 1. 加大请求量，以便测出更稳定的数值
	totalRequests := 2000
	concurrency := 500 // 并发协程数

	apiURL := "http://localhost:8000/api/v1/orders"
	jsonBody := []byte(`{"amount": 100, "items": ["iPhone15"]}`)

	// 🔥 2. 必须优化 Client，消除客户端瓶颈
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000, // 关键：允许保持 1000 个长连接
			IdleConnTimeout:     30 * time.Second,
		},
		Timeout: 5 * time.Second,
	}

	var wg sync.WaitGroup
	wg.Add(totalRequests) // 注意这里 WaitGroup 等待的是总请求数

	// 限制并发数的管道 (Semaphore pattern)
	// 如果直接开 2000 个 goroutine 可能会太重，控制同时只有 500 个在跑
	sem := make(chan struct{}, concurrency)

	var success, failure, other int
	var mu sync.Mutex

	fmt.Printf("🚀 开始压测: 总请求 %d, 并发 %d...\n", totalRequests, concurrency)
	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		sem <- struct{}{} // 获取令牌
		go func(id int) {
			defer func() {
				<-sem // 释放令牌
				wg.Done()
			}()

			resp, err := httpClient.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
			if err != nil {
				fmt.Printf("请求失败: %v\n", err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == 200 {
				success++
			} else if resp.StatusCode == 429 {
				failure++
			} else {
				other++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 🔥 3. 核心计算公式
	qps := float64(totalRequests) / duration.Seconds()

	fmt.Println("\n====== 📊 性能报告 ======")
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("总请求: %d\n", totalRequests)
	fmt.Printf("🔥 真实 QPS: %.2f (Requests/Sec)\n", qps) // 这里就是你要的数字！
	fmt.Printf("---------------------------\n")
	fmt.Printf("成功 (200): %d\n", success)
	fmt.Printf("拦截 (429): %d\n", failure)
	fmt.Printf("错误 (Other): %d\n", other)
}
