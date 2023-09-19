package handles

import (
	"fmt"
	"github.com/alist-org/alist/v3/drivers/kuwo"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/gin-gonic/gin"
)

type MusicSearchReq struct {
	Keywords string `json:"keywords"`
	PageSize int    `json:"pageSize"`
	PageNo   int    `json:"pageNo"`
}

type MusicList struct {
	Musics []kuwo.MusicInfo `json:"musicList"`
}

func (p *MusicSearchReq) Validate() error {
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageNo < 1 {
		p.PageNo = 1
	}
	return nil
}

func MusicSearch(c *gin.Context) {
	var (
		req MusicSearchReq
		err error
	)
	if err = c.ShouldBind(&req); err != nil {
		fail(c, "参数错误")
		return
	}
	err = req.Validate()
	if err != nil {
		fail(c, "参数错误")
		return
	}
	music, err := kuwo.SearchKuwoMusic(req.Keywords, req.PageNo, req.PageSize)
	if err != nil {
		fail(c, "搜索异常")
		return
	}
	common.SuccessResp(c, music)
}

func AddDownLoadTask(c *gin.Context) {
	var (
		req MusicList
		err error
	)
	if err = c.ShouldBind(&req); err != nil {
		fail(c, "参数错误")
		return
	}
	fmt.Printf("%+v", req)
	kuwo.HandTask(req.Musics)
	common.SuccessResp(c)
}

func ListDownLoadTask(c *gin.Context) {
	tasks := kuwo.ListUnHandTask()
	common.SuccessResp(c, tasks)
}

func fail(c *gin.Context, msg string) {
	common.ErrorStrResp(c, msg, 500)
}
