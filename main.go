package main

import (
	"fmt"
	"github.com/alist-org/alist/v3/drivers/kuwo"
)

func main() {
	//cmd.Execute()
	//kuwo.GetDownloadUrl("108414", "mp3", "")
	format := kuwo.SearchMusicPic("108414")
	fmt.Printf("%+v", format)
}
