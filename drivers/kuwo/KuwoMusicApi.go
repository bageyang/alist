package kuwo

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
	"io"
	"math/big"
	"strconv"
	"strings"
)

func init() {
	two := big.NewInt(2)
	for i := range ArrayMask {
		ArrayMask[i] = new(big.Int).Exp(two, big.NewInt(int64(i)), nil)
	}
	ArrayMask[len(ArrayMask)-1] = new(big.Int).Exp(two, big.NewInt(63), nil)
	ArrayMask[len(ArrayMask)-1].Mul(ArrayMask[len(ArrayMask)-1], big.NewInt(-1))
	_, _ = csrfGet("https://www.kuwo.cn/search/list?key=%E5%91%A8")
}

var client = resty.New()

const SearchUrl = "https://search.kuwo.cn/r.s?all=%s&ft=music&newsearch=1&alflac=1&itemset=web_2013&client=kt&cluster=0&pn=%d&rn=%d&vermerge=1&rformat=json&encoding=utf8&show_copyright_off=1&pcmp4=1&ver=mbox&vipver=MUSIC_8.7.7.1_BDS4&plat=pc&devid=76039576"

func SearchKuwoMusic(text string, pageNo int, pageSize int) (*SearchRsp, error) {
	if pageNo < 1 {
		pageNo = 1
	}
	pageNo = pageNo - 1
	url := fmt.Sprintf(SearchUrl, text, pageNo, pageSize)
	searchDto := new(SearchRsp)
	rsp, err := client.R().Get(url)
	if err != nil {
		log.Errorf("失败：%+v", err)
		return nil, err
	}
	validJSON := strings.Replace(string(rsp.Body()), "'", "\"", -1)
	err = json.Unmarshal([]byte(validJSON), searchDto)
	if err != nil {
		return nil, err
	}
	for i := range searchDto.AbsList {
		searchDto.AbsList[i].Album = strings.Replace(searchDto.AbsList[i].Album, "&nbsp;", "-", -1)
		searchDto.AbsList[i].Artist = strings.Replace(searchDto.AbsList[i].Artist, "&nbsp;", "-", -1)
		searchDto.AbsList[i].Name = strings.Replace(searchDto.AbsList[i].Name, "&nbsp;", "-", -1)
	}
	return searchDto, nil
}

func GetDownloadUrl(rid string, format string, bitrate string) *KuwoPlay {
	if len(format) == 0 {
		format = "mp3"
	}
	var builder strings.Builder
	if len(bitrate) == 0 {
		_, err := fmt.Fprintf(&builder, "corp=kuwo&p2p=1&type=convert_url2&format=%s&rid=%s", format, rid)
		if err != nil {
			log.Errorf("格式化url错误:%+v", err)
			return nil
		}
	} else {
		_, err := fmt.Fprintf(&builder, "corp=kuwo&p2p=1&type=convert_url2&format=%s&rid=%s&br=%s", format, rid, bitrate)
		if err != nil {
			log.Errorf("格式化url错误:%+v", err)
			return nil
		}
	}
	willEnc := builder.String()
	s := base64Encrypt(willEnc)
	url := "http://nmobi.kuwo.cn/mobi.s?f=kuwo&q=" + s
	rsp, err := csrfGet(url)
	if err != nil {
		log.Errorf("获取下载地址失败：%+v", err)
		return nil
	}
	body := string(rsp.Body())
	if body == "" {
		log.Errorf("获取下载地址失败：%+v", body)
		return nil
	}
	split := strings.Split(body, "\r\n")
	if len(split) < 2 {
		log.Errorf("获取下载地址失败：%+v", body)
		return nil
	}
	mapKeyValue := make(map[string]string)
	for _, s := range split {
		keyAndValue := strings.Split(s, "=")
		if len(keyAndValue) < 2 {
			continue
		}
		mapKeyValue[keyAndValue[0]] = keyAndValue[1]
	}
	kuwoPlay := &KuwoPlay{
		Format:  mapKeyValue["format"],
		Bitrate: mapKeyValue["bitrate"],
		Url:     mapKeyValue["url"],
		Rid:     mapKeyValue["rid"],
	}
	return kuwoPlay
}

const InfoUrl = "https://search.kuwo.cn/r.s?stype=musicinfo&itemset=music_2014&alflac=1&pcmp4=1&ids=%s"

func GetMusicFormat(musicRid string) *FormatInfo {
	if len(musicRid) < 1 || strings.Index(musicRid, "MUSIC_") < 0 {
		log.Errorf("音乐id有误：%+v", musicRid)
		return nil
	}
	url := fmt.Sprintf(InfoUrl, musicRid)
	rsp, err := client.R().Get(url)
	if err != nil {
		log.Errorf("查询歌曲信息失败: %+v", err)
		return nil
	}
	data := deflateData(rsp.Body())
	if len(data) < 1 {
		log.Errorf("解密format失败: %+v", err)
		return nil
	}
	split := strings.Split(data, "\r\n")
	if len(split) < 1 {
		log.Errorf("解密format失败: %+v", data)
		return nil
	}
	tmp := int64(0)
	var tmpFormat FormatInfo
	for _, row := range split {
		format := getFormatAndSize(row)
		if format != nil && tmp < format.Size {
			tmpFormat = *format
		}
	}
	return &tmpFormat
}

func getFormatAndSize(row string) *FormatInfo {
	format := ""
	if strings.Contains(row, "WMA") {
		format = "wma"
	} else if strings.Contains(row, "MP3") {
		format = "mp3"
	} else if strings.Contains(row, "FLAC") {
		format = "flac"
	} else {
		return nil
	}
	sizeStr := subByStr(row, "SIZE:", "|")
	num, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return nil
	}
	bitrate := subByStr(row, "BT:", "")
	if bitrate != "" {
		bitrate = bitrate + "k" + format
	}
	f := &FormatInfo{
		Format:  format,
		Size:    num,
		Bitrate: bitrate,
	}
	return f
}

const MusicPicUrl = "https://artistpicserver.kuwo.cn/pic.web?type=big_artist_pic&pictype=url&content=list&&id=0&rid=%s&from=pc&json=1&version=1&width=1920&height=1080"

func SearchMusicPic(rid string) []string {
	url := fmt.Sprintf(MusicPicUrl, rid)
	rsp, err := client.R().Get(url)
	if err != nil {
		log.Errorf("搜索图片请求错误:%+v", err)
		return nil
	}
	k := new(KuwoPicRsp)
	err = json.Unmarshal(rsp.Body(), &k)
	if err != nil {
		log.Errorf("搜索图片解析错误:%+v", err)
		return nil
	}
	list := k.PicList
	if len(list) < 1 {
		return nil
	}
	size := 5
	if len(list) < 5 {
		size = len(list)
	}
	var ret []string

	for i := 0; i < size; i++ {
		pic := list[i]
		v, ok := pic["bkurl"]
		if ok {
			ret = append(ret, v)
			continue
		}
		v, ok = pic["wpurl"]
		if ok {
			ret = append(ret, v)
			continue
		}
	}
	return ret
}

const MusicLrcUrl = "https://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=%s"

func GetMusicLrc(rid string) string {
	url := fmt.Sprintf(MusicLrcUrl, rid)
	rsp, err := client.R().
		SetHeader("Accept", " application/json, text/plain, */*").
		SetHeader("Accept-Encoding", "gzip, deflate").
		SetHeader("Accept-Language", " zh,zh-CN;q=0.9,en;q=0.8").
		SetHeader("Cookie",
			"_ga=GA1.2.857113574.1642845552; Hm_lvt_cdb524f42f0ce19b169a8071123a4797=1657840598,1658149818,1658150983,1658364956; _gid=GA1.2.1639017380.1658364956; uname3=qq1616934321; t3kwid=131286315; userid=131286315; websid=931938875; pic3=\"\"; t3=qq; Hm_lpvt_cdb524f42f0ce19b169a8071123a4797=1658370991; _gat=1; kw_token=KVURCEC25G").
		SetHeader("Host", "www.kuwo.cn").
		SetHeader("Proxy-Connection", "keep-alive").
		SetHeader("Referer", "http://www.kuwo.cn/search/list?key=%E5%91%A8%E6%9D%B0%E4%BC%A6").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36").
		Get(url)
	if err != nil {
		log.Errorf("获取歌词请求错误:%+v", err)
		return ""
	}
	k := new(LrcRsp)
	err = json.Unmarshal(rsp.Body(), &k)
	if err != nil {
		log.Errorf("获取歌词请求解析错误:%+v", err)
		return ""
	}
	if k.Status != 200 {
		log.Errorf("获取歌词请求错误:%+v", k)
		return ""
	}

	if len(k.LrcData.LrcList) == 0 {
		log.Errorf("获取歌词内容为空:%+v", k)
		return ""
	}
	var builder strings.Builder
	for _, lrc := range k.LrcData.LrcList {
		lyric := lrc.LineLyric
		time := lrc.Time
		f, err := strconv.ParseFloat(time, 64)
		if err != nil {
			log.Errorf("歌词解析错误: %+v", err)
			continue
		}
		duration := formatDuration(f)
		builder.WriteString("[" + duration + "]" + lyric)
		builder.WriteString("\n")
	}
	return builder.String()
}

func formatDuration(totalSeconds float64) string {
	minutes := int(totalSeconds / 60)
	seconds := totalSeconds - float64(minutes*60)
	return fmt.Sprintf("%02d:%05.2f", minutes, seconds)
}

func subByStr(src string, left string, right string) string {
	i := strings.Index(src, left)
	if i < 0 {
		return ""
	}
	l := i + len(left)

	var r int
	if right == "" {
		r = len(src)
	} else {
		ri := strings.Index(src[l:], right)
		if ri == -1 {
			r = len(src)
		} else {
			r = l + ri
		}
	}
	return src[l:r]
}

func deflateData(data []byte) string {
	newBytes := findBytes(data)
	if newBytes == nil {
		log.Error("没有找到满足条件的字节序列")
		return ""
	}

	r, err := zlib.NewReader(bytes.NewReader(newBytes))
	if err != nil {
		log.Errorf("无法使用zlib解压，错误原因: %+v", err)
		return ""
	}
	defer func(r io.ReadCloser) {
		err := r.Close()
		if err != nil {
			log.Errorf("关闭流失败: %+v", err)
		}
	}(r)

	decompress, err := io.ReadAll(r)
	if err != nil {
		log.Errorf("无法读取解压数据，错误原因: %+v", err)
		return ""
	}
	return string(decompress)
}

func findBytes(b []byte) []byte {
	needle := []byte{0x00, 0x00, 0x78, 0x9c}
	index := bytes.Index(b, needle)
	if index == -1 {
		// 如果未找到，返回nil
		return nil
	}
	return b[index+2:] // 将0x78, 0x9c后的数据截取出来
}

var csrf = ""

func csrfGet(url string) (*resty.Response, error) {
	rsp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36 Edg/110.0.1587.50").
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7").
		SetHeader("Referer", "https://www.kuwo.cn/search/list?key=").
		SetHeader("csrf", csrf).
		Get(url)
	if err != nil {
		return rsp, err
	} else {
		header := rsp.Header()
		ck := header.Get("Set-Cookie")
		if len(ck) != 0 {
			l := strings.Index(ck, "kw_token=") + 9
			r := strings.Index(ck[l:], ";")
			if r > l {
				csrf = ck[l:(l + r)]
			}
		}
		return rsp, nil
	}
}

func base64Encrypt(msg string) string {
	b1 := encrypt(msg)
	arr2 := make([]byte, len(b1))
	for i, v := range b1 {
		arr2[i] = byte(v)
	}
	s := base64.StdEncoding.EncodeToString(arr2)
	s = strings.Replace(s, "\n", "", -1)
	return s
}

var SecretKey = []byte("ylzsxkwm")
var ArrayPc1 = []int{56, 48, 40, 32, 24, 16, 8, 0, 57, 49, 41, 33, 25, 17, 9, 1, 58, 50, 42, 34, 26, 18, 10, 2, 59, 51, 43, 35, 62, 54, 46, 38, 30, 22, 14, 6, 61, 53, 45, 37, 29, 21, 13, 5, 60, 52, 44, 36, 28, 20, 12, 4, 27, 19, 11, 3}
var ArrayLsMask = []int64{0, 0x100001, 0x300003}
var ArrayLs = []int{1, 1, 2, 2, 2, 2, 2, 2, 1, 2, 2, 2, 2, 2, 2, 1}
var ArrayPc2 = []int{13, 16, 10, 23, 0, 4, -1, -1, 2, 27, 14, 5, 20, 9, -1, -1, 22, 18, 11, 3, 25, 7, -1, -1, 15, 6, 26, 19, 12, 1, -1, -1, 40, 51, 30, 36, 46, 54, -1, -1, 29, 39, 50, 44, 32, 47, -1, -1, 43, 48, 38, 55, 33, 52, -1, -1, 45, 41, 49, 35, 28, 31, -1, -1}
var ArrayIp = []int{57, 49, 41, 33, 25, 17, 9, DesModeDecrypt, 59, 51, 43, 35, 27, 19, 11, 3, 61, 53, 45, 37, 29, 21, 13, 5, 63, 55, 47, 39, 31, 23, 15, 7, 56, 48, 40, 32, 24, 16, 8, 0, 58, 50, 42, 34, 26, 18, 10, 2, 60, 52, 44, 36, 28, 20, 12, 4, 62, 54, 46, 38, 30, 22, 14, 6}
var DesModeDecrypt = 1
var ArrayE = []int{31, 0, DesModeDecrypt, 2, 3, 4, -1, -1, 3, 4, 5, 6, 7, 8, -1, -1, 7, 8, 9, 10, 11, 12, -1, -1, 11, 12, 13, 14, 15, 16, -1, -1, 15, 16, 17, 18, 19, 20, -1, -1, 19, 20, 21, 22, 23, 24, -1, -1, 23, 24, 25, 26, 27, 28, -1, -1, 27, 28, 29, 30, 31, 30, -1, -1}
var ArrayP = []int{15, 6, 19, 20, 28, 11, 27, 16, 0, 14, 22, 25, 4, 17, 30, 9, 1, 7, 23, 13, 31, 26, 2, 8, 18, 12, 29, 5, 21, 10, 3, 24}
var ArrayIP1 = []int{39, 7, 47, 15, 55, 23, 63, 31, 38, 6, 46, 14, 54, 22, 62, 30, 37, 5, 45, 13, 53, 21, 61, 29, 36, 4, 44, 12, 52, 20, 60, 28, 35, 3, 43, 11, 51, 19, 59, 27, 34, 2, 42, 10, 50, 18, 58, 26, 33, DesModeDecrypt, 41, 9, 49, 17, 57, 25, 32, 0, 40, 8, 48, 16, 56, 24}
var MatrixNSBox = [][]int{
	{14, 4, 3, 15, 2, 13, 5, 3, 13, 14, 6, 9, 11, 2, 0, 5, 4, 1, 10, 12, 15, 6, 9, 10, 1, 8, 12, 7, 8, 11, 7, 0, 0, 15, 10, 5, 14, 4, 9, 10, 7, 8, 12, 3, 13, 1, 3, 6, 15, 12, 6, 11, 2, 9, 5, 0, 4, 2, 11, 14, 1, 7, 8, 13},
	{15, 0, 9, 5, 6, 10, 12, 9, 8, 7, 2, 12, 3, 13, 5, 2, 1, 14, 7, 8, 11, 4, 0, 3, 14, 11, 13, 6, 4, 1, 10, 15, 3, 13, 12, 11, 15, 3, 6, 0, 4, 10, 1, 7, 8, 4, 11, 14, 13, 8, 0, 6, 2, 15, 9, 5, 7, 1, 10, 12, 14, 2, 5, 9},
	{10, 13, 1, 11, 6, 8, 11, 5, 9, 4, 12, 2, 15, 3, 2, 14, 0, 6, 13, 1, 3, 15, 4, 10, 14, 9, 7, 12, 5, 0, 8, 7, 13, 1, 2, 4, 3, 6, 12, 11, 0, 13, 5, 14, 6, 8, 15, 2, 7, 10, 8, 15, 4, 9, 11, 5, 9, 0, 14, 3, 10, 7, 1, 12},
	{7, 10, 1, 15, 0, 12, 11, 5, 14, 9, 8, 3, 9, 7, 4, 8, 13, 6, 2, 1, 6, 11, 12, 2, 3, 0, 5, 14, 10, 13, 15, 4, 13, 3, 4, 9, 6, 10, 1, 12, 11, 0, 2, 5, 0, 13, 14, 2, 8, 15, 7, 4, 15, 1, 10, 7, 5, 6, 12, 11, 3, 8, 9, 14},
	{2, 4, 8, 15, 7, 10, 13, 6, 4, 1, 3, 12, 11, 7, 14, 0, 12, 2, 5, 9, 10, 13, 0, 3, 1, 11, 15, 5, 6, 8, 9, 14, 14, 11, 5, 6, 4, 1, 3, 10, 2, 12, 15, 0, 13, 2, 8, 5, 11, 8, 0, 15, 7, 14, 9, 4, 12, 7, 10, 9, 1, 13, 6, 3},
	{12, 9, 0, 7, 9, 2, 14, 1, 10, 15, 3, 4, 6, 12, 5, 11, 1, 14, 13, 0, 2, 8, 7, 13, 15, 5, 4, 10, 8, 3, 11, 6, 10, 4, 6, 11, 7, 9, 0, 6, 4, 2, 13, 1, 9, 15, 3, 8, 15, 3, 1, 14, 12, 5, 11, 0, 2, 12, 14, 7, 5, 10, 8, 13},
	{4, 1, 3, 10, 15, 12, 5, 0, 2, 11, 9, 6, 8, 7, 6, 9, 11, 4, 12, 15, 0, 3, 10, 5, 14, 13, 7, 8, 13, 14, 1, 2, 13, 6, 14, 9, 4, 1, 2, 14, 11, 13, 5, 0, 1, 10, 8, 3, 0, 11, 3, 5, 9, 4, 15, 2, 7, 8, 12, 15, 10, 7, 6, 12},
	{13, 7, 10, 0, 6, 9, 5, 15, 8, 4, 3, 10, 11, 14, 12, 5, 2, 11, 9, 6, 15, 12, 0, 3, 4, 1, 14, 13, 1, 2, 7, 8, 1, 2, 12, 15, 10, 4, 0, 3, 13, 14, 6, 9, 7, 8, 9, 6, 15, 1, 5, 12, 3, 10, 14, 5, 8, 7, 11, 0, 4, 13, 2, 11},
}
var ArrayMask [64]*big.Int

func encrypt(msg string) []int8 {
	msgBytes := []byte(msg)
	key := SecretKey
	l := int64(0)
	for i := 0; i < 8; i++ {
		l = l | (int64(key[i]) & 0xFF << (i * 8))
	}
	j := len(msgBytes) / 8
	arrLong1 := make([]int64, 16)
	for i := range arrLong1 {
		arrLong1[i] = 0
	}
	subKeys(l, arrLong1, 0)
	arrLong2 := make([]int64, j)
	for m := 0; m < j; m++ {
		for n := 0; n < 8; n++ {
			arrLong2[m] |= int64(msgBytes[n+m*8]) & 0xFF << (n * 8)
		}
	}
	newLength := (1 + 8*(j+1)) / 8
	arrLong3 := make([]int64, newLength)
	for i := 0; i < j; i++ {
		arrLong3[i] = des64(arrLong1, arrLong2[i])
	}
	arrByte1 := msgBytes[j*8:]
	var l2 int64 = 0
	for i1 := 0; i1 < len(msgBytes)%8; i1++ {
		l2 |= int64(arrByte1[i1]) << (i1 * 8)
	}
	arrLong3[j] = des64(arrLong1, l2)
	arrByte2 := make([]int8, 8*len(arrLong3))
	i4 := 0
	for _, l3 := range arrLong3 {
		for i6 := 0; i6 < 8; i6++ {
			arrByte2[i4] = int8(255 & (l3 >> (i6 * 8)))
			i4 += 1
		}
	}
	return arrByte2
}

func des64(arrLong []int64, l int64) int64 {
	pR := make([]int, 8)
	pSource := make([]int64, 2)
	var SOut int64 = 0
	var L int64 = 0
	var R int64 = 0
	out := bitTransform(ArrayIp, 64, l)
	pSource[0] = 0xFFFFFFFF & out
	pSource[1] = (-4294967296 & out) >> 32
	for i := 0; i < 16; i++ {
		R = pSource[1]
		R = bitTransform(ArrayE, 64, R)
		R ^= arrLong[i]
		for j := 0; j < 8; j++ {
			pR[j] = int(int64(255) & (R >> (j * 8)))
		}
		SOut = 0
		for sbi := 7; sbi >= 0; sbi-- {
			SOut <<= 4
			SOut |= int64(MatrixNSBox[sbi][pR[sbi]])
		}

		R = bitTransform(ArrayP, 32, SOut)
		L = pSource[0]
		pSource[0] = pSource[1]
		pSource[1] = L ^ R
	}
	reverseArray(pSource)
	out = pSource[1]<<32 | int64(uint32(pSource[0]))
	out = bitTransform(ArrayIP1, 64, out)
	return out
}

func reverseArray(array []int64) {
	for i := 0; i < len(array)/2; i++ {
		temp := array[i]
		array[i] = array[len(array)-i-1]
		array[len(array)-i-1] = temp
	}
}

func subKeys(l int64, longs []int64, n int) {
	l2 := bitTransform(ArrayPc1, 56, l)
	for i := 0; i < 16; i++ {
		l2 = ((l2 & ArrayLsMask[ArrayLs[i]]) << (28 - ArrayLs[i])) | ((l2 & ^ArrayLsMask[ArrayLs[i]]) >> uint(ArrayLs[i]))
		longs[i] = bitTransform(ArrayPc2, 64, l2)
	}
	j := 0
	for n == 1 && j < 8 {
		longs[j], longs[15-j] = longs[15-j], longs[j]
		j += 1
	}
}

func bitTransform(arrInt []int, n int, l int64) int64 {
	l2 := new(big.Int)
	for i := 0; i < n; i++ {
		lBigInt := big.NewInt(l)
		if arrInt[i] < 0 || (lBigInt.And(ArrayMask[arrInt[i]], lBigInt).Cmp(big.NewInt(0)) == 0) {
			continue
		}
		l2.Or(ArrayMask[i], l2)
	}
	return l2.Int64()
}
