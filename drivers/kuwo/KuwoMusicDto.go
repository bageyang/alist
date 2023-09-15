package kuwo

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

var client = resty.New()

const SearchUrl = "https://search.kuwo.cn/r.s?all=%s&ft=music&newsearch=1&alflac=1&itemset=web_2013&client=kt&cluster=0&pn=%d&rn=%d&vermerge=1&rformat=json&encoding=utf8&show_copyright_off=1&pcmp4=1&ver=mbox&vipver=MUSIC_8.7.7.1_BDS4&plat=pc&devid=76039576"

func SearchKuwoMusic(text string, pageNo int, pageSize int) (*SearchRsp, error) {
	url := fmt.Sprintf(SearchUrl, text, pageNo, pageSize)
	searchDto := new(SearchRsp)
	rsp, err := client.R().Get(url)
	if err != nil {
		log.Fatal("失败：", err)
		return nil, err
	}
	jsonParsedObj, _ := gabs.ParseJSON([]byte(`{
		"outer":{
			"values":{
				"first":10,
				"second":11
			}
		},
		"outer2":"hello world"
	}`))
	jsonOutput := jsonParsedObj.String()
	fmt.Println(string(rsp.Body()))
	fmt.Println(searchDto.Total)
	return searchDto, nil
}

func main() {

}
