package kuwo

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func MoveFile(accessToken string, srcPath string, destPath string, name string) error {
	opera := "move"
	async := "0"
	var files []FileListInfo
	fileListInfo := FileListInfo{
		Path:    srcPath,
		Dest:    destPath,
		Ondup:   "overwrite",
		Newname: name,
	}
	files = append(files, fileListInfo)
	arg := NewFileManagerArg(opera, async, files)
	_, err := FileManager(accessToken, arg)
	return err

}

func FileManager(accessToken string, arg *FileManagerArg) (FileManagerReturn, error) {
	ret := FileManagerReturn{}

	protocal := "http"
	host := "pan.baidu.com"
	router := "/rest/2.0/xpan/file?method=filemanager&"
	uri := protocal + "://" + host + router

	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("opera", arg.Opera)
	uri += params.Encode()

	headers := map[string]string{
		"Host":         host,
		"Content-Type": "application/x-www-form-urlencoded",
	}

	postBody := url.Values{}
	postBody.Add("async", arg.Async)
	fileListJson, err := json.Marshal(arg.FileList)
	if err != nil {
		return ret, err
	}
	postBody.Add("filelist", string(fileListJson))

	body, _, err := DoHTTPRequest(uri, strings.NewReader(postBody.Encode()), headers)
	if err != nil {
		return ret, err
	}
	if err = json.Unmarshal([]byte(body), &ret); err != nil {
		return ret, errors.New("unmarshal filemanager body failed,body")
	}
	if ret.Errno != 0 {
		log.Errorf("ret:%+v", body)
		return ret, errors.New("call filemanager failed")
	}
	return ret, nil
}

func DoHTTPRequest(url string, body io.Reader, headers map[string]string) (string, int, error) {
	timeout := 5 * time.Second
	retryTimes := 3
	tr := &http.Transport{
		MaxIdleConnsPerHost: -1,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: tr}
	httpClient.Timeout = timeout
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", 0, err
	}
	// request header
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	var resp *http.Response
	for i := 1; i <= retryTimes; i++ {
		resp, err = httpClient.Do(req)
		if err == nil {
			break
		}
		if i == retryTimes {
			return "", 0, err
		}
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(respBody), resp.StatusCode, nil
}

type FileManagerReturn struct {
	Errno     int                      `json:"errno"`
	Info      []map[string]interface{} `json:"info"`
	Taskid    int                      `json:"taskid"` // 异步下返回此参数
	RequestId int                      `json:"request_id"`
}

func NewFileManagerArg(opera string, async string, fileList []FileListInfo) *FileManagerArg {
	s := new(FileManagerArg)
	s.Opera = opera
	s.Async = async
	s.FileList = fileList
	return s
}

type FileManagerArg struct {
	Opera    string         `json:"opera"`
	Async    string         `json:"async"`
	FileList []FileListInfo `json:"filelist"`
}

type FileListInfo struct {
	Path    string `json:"path"`
	Newname string `json:"newname"`
	Dest    string `json:"dest"`
	Ondup   string `json:"ondup"`
}
