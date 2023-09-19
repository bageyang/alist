package common

import (
	"context"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/redis/go-redis/v9"
	"strconv"
	"sync"
	"time"
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
	go watchRedisConfig()
}

func watchRedisConfig() {
	time.Sleep(time.Second * 30)
	for {
		groups, err := op.GetSettingItemsByGroup(model.MUSIC)
		if err != nil {
			utils.Log.Errorf("配置查询失败: %+v", err)
			continue
		}
		changed := false
		newOptions := &redis.Options{}
		for _, group := range groups {
			key := group.Key
			value := group.Value
			if key == conf.RedisAddr && len(value) > 0 {
				newOptions.Addr = value
				if clientOptions.Addr != value {
					changed = true
					break
				}
			}
			if key == conf.RedisPassword && len(value) > 0 {
				newOptions.Password = value
				if clientOptions.Password != value {
					changed = true
					break
				}
			}
			if key == conf.RedisDB && len(value) > 0 {
				num, _ := strconv.Atoi(value)
				newOptions.DB = num
				if clientOptions.DB != num {
					changed = true
					break
				}
			}
		}
		if changed {
			ResetRedisClient(newOptions)
			clientOptions = newOptions
			utils.Log.Infof("Redis 加载新配置: %+v", clientOptions)
		}
		time.Sleep(time.Second * 30)
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
