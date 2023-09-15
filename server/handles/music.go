package handles

import (
	"github.com/alist-org/alist/v3/drivers/kuwo"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/gin-gonic/gin"
)

type MusicSearchReq struct {
	Keywords string `json:"keywords"`
	PageSize int    `json:"pageSize"`
	PageNo   int    `json:"pageNo"`
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
		common.ErrorResp(c, err, 400)
		return
	}
	err = req.Validate()
	if err != nil {
		return
	}
	kuwo.SearchKuwoMusic(req.Keywords, req.PageNo, req.PageSize)

}
