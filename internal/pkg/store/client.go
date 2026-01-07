package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	Client *redis.Client
}

// NewRedisStore 初始化 Redis 连接
func NewRedisStore(addr string) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // 本地开发通常无密码
		DB:       0,

		// 🔥🔥🔥 新增优化配置 🔥🔥🔥
		PoolSize:     200,              // 最大连接数 (设大一点，比如 200)
		MinIdleConns: 20,               // 最小空闲连接 (保持预热)
		PoolTimeout:  30 * time.Second, // 等待连接的超时时间
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("❌ Redis 连接失败: %v", err)
	}
	log.Println("✅ Redis 连接成功")
	return &RedisStore{Client: rdb}
}

// PreheatStock 库存预热：把 MySQL 库存刷入 Redis
func (r *RedisStore) PreheatStock(ctx context.Context, productID string, stock int) error {
	key := fmt.Sprintf("stock:%s", productID)
	// 设置库存
	return r.Client.Set(ctx, key, stock, 0).Err()
}

// DeductStock 原子扣减库存 (执行 Lua)
// 返回值: 1=成功, 0=库存不足, -1=未预热
func (r *RedisStore) DeductStock(ctx context.Context, productID string, amount int) (int, error) {
	key := fmt.Sprintf("stock:%s", productID)

	val, err := r.Client.Eval(ctx, AtomicDeductStock, []string{key}, amount).Result()
	if err != nil {
		return 0, err
	}

	if res, ok := val.(int64); ok {
		return int(res), nil
	}
	return 0, fmt.Errorf("redis 返回类型错误")
}

// RollbackStock 库存回滚 (补偿)
// 当 Workflow 提交失败时，把 Redis 库存加回去
func (r *RedisStore) RollbackStock(ctx context.Context, productID string, amount int) error {
	key := fmt.Sprintf("stock:%s", productID)
	return r.Client.IncrBy(ctx, key, int64(amount)).Err()
}
