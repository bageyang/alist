package kuwo

import (
	"context"
	"github.com/alist-org/alist/v3/server/common"
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
		startTask()
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

}
