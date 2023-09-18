package kuwo

import (
	"context"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"sync"
)

const MusicTask = "music:task:kuwo-queue"

var IsRuning = false

var clientMu sync.Mutex

func HandTask(musicIds []string) bool {
	client := common.GetRedisClient()
	var ctx = context.Background()
	client.LPush(ctx, MusicTask, musicIds)
	checkTask()
	return true
}

func checkTask() {
	clientMu.Lock()
	defer clientMu.Unlock()
	if !IsRuning {
		go startTask()
	}
}

func startTask() {
	clientMu.Lock()
	if IsRuning {
		clientMu.Unlock()
		return
	} else {
		IsRuning = true
		clientMu.Unlock()
	}
	ctx := context.Background()
	redisClient := common.GetRedisClient()
	for {
		musicId, err := redisClient.LPop(ctx, MusicTask).Result()
		if err == redis.Nil {
			log.Info("暂时无任务,结束。。。")
			break
		} else if err != nil {
			panic(err)
		} else {
			downloadMusic(musicId)
		}
	}

}

func downloadMusic(id string) {
}
