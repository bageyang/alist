package main

import (
	"github.com/alist-org/alist/v3/drivers/kuwo"
)

func main() {
	//cmd.Execute()
	music, err := kuwo.SearchKuwoMusic("周杰伦", 1, 10)
	if err != nil {
		print("错误")
		return
	}
	print("=================================")
	print(music.Total)
}
