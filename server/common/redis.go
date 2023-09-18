package common

import (
	"context"
	"github.com/redis/go-redis/v9"
	"sync"
)

var (
	clientInstance *redis.Client
	once           sync.Once
	ctx            = context.Background()
	clientOptions  *redis.Options
	clientMu       sync.Mutex
)

func init() {
	// 默认连接参数
	clientOptions = &redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}
}

func GetRedisClient() *redis.Client {
	clientMu.Lock()
	defer clientMu.Unlock()

	once.Do(func() {
		clientInstance = redis.NewClient(clientOptions)
	})

	return clientInstance
}

func ResetRedisClient(options *redis.Options) {
	clientMu.Lock()
	defer clientMu.Unlock()

	// 关闭旧的 Client 连接
	if clientInstance != nil {
		_ = clientInstance.Close()
	}
	// 重置 once，使其在下次调用 GetClient 时能够重新初始化 Client
	once = sync.Once{}
	// 设置新的连接参数
	clientOptions = options
}
