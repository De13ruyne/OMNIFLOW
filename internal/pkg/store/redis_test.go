package store

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

func TestFlashSale_Concurrency(t *testing.T) {
	// 1. 启动一个内存 Redis (模拟真实环境)
	s := miniredis.RunT(t)

	// 2. 初始化我们的 RedisStore
	store := NewRedisStore(s.Addr())
	ctx := context.Background()

	// 3. 预热库存：iPhone15 只有 10 个
	product := "iPhone15"
	initialStock := 10
	err := store.PreheatStock(ctx, product, initialStock)
	assert.NoError(t, err)

	// 4. 模拟 100 个人同时抢购 (高并发)
	concurrentNum := 100
	var wg sync.WaitGroup
	wg.Add(concurrentNum)

	successCount := 0
	failCount := 0
	var mu sync.Mutex // 计数锁

	for i := 0; i < concurrentNum; i++ {
		go func() {
			defer wg.Done()

			// 尝试扣减 1 个库存
			// 注意：这里是直接测 Lua 脚本逻辑，不经过 HTTP
			res, _ := store.DeductStock(ctx, product, 1)

			mu.Lock()
			if res == 1 {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 5. 验证结果 (核心断言)
	// 应该只有 10 个人抢到，90 个人失败
	assert.Equal(t, 10, successCount, "抢购成功人数应该等于库存数")
	assert.Equal(t, 90, failCount, "剩下的人应该全部失败")

	// 验证 Redis 里剩下的库存应该是 0，而不是负数
	s.CheckGet(t, "stock:iPhone15", "0")
}
