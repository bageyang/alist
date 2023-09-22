package kuwo

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

const panPath = "/apps/Alist/"
const normalSize = 4194304
const vipSize = 16777216
const sVipSize = 33554432
const sliceSize = sVipSize

func Upload(accessToken string, srcPath string, name string) error {
	tmpDir := getTmpDir(srcPath)
	destPath := panPath + name
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Errorf("删除临时目录异常:%+v", err)
		}
	}(tmpDir)

	// slice and preCreate
	sliceMd5List, fileSize, err := getSliceMd5(srcPath, tmpDir)
	if err != nil {
		log.Errorf("上传失败,分片异常: %+v", srcPath)
		return err
	}
	arg := NewPrecreateArg(destPath, fileSize, sliceMd5List)
	preCreateRet, err := PreCreate(accessToken, arg)
	if err != nil {
		log.Errorf("预上传异常,请求参数：%+v", arg)
		log.Errorf("预上传异常,case: %+v,返回值: %+v", err, preCreateRet)
		return err
	}
	uploadId := preCreateRet.UploadId
	sliceNum := len(sliceMd5List)

	// upload slice
	for i := 0; i < sliceNum; i++ {
		sliceFilePath := tmpDir + strconv.Itoa(i)
		uploadArg := NewUploadArg(uploadId, destPath, sliceFilePath, i)
		uploadRet, err := sliceUpload(accessToken, uploadArg)
		if err != nil {
			log.Errorf("分片上传异常: %+v,返回值: %+v", err, uploadRet)
			return err
		}
	}

	// createFile
	createArg := NewCreateArg(uploadId, destPath, fileSize, sliceMd5List)
	createRet, err := CreateFile(accessToken, createArg)
	if err != nil {
		log.Errorf("网盘创建文件异常: %+v,返回值: %+v", err, createRet)
		return err
	}
	return nil
}

func getSliceMd5(filePath string, tmpDir string) ([]string, uint64, error) {
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		log.Errorf("创建临时目录异常:  %+v", err)
		return nil, 0, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("打开文件异常:  %+v", err)
		return nil, 0, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		log.Errorf("文件状态异常: %+v", err)
	}

	fileSize := fileInfo.Size()
	sliceNum := int(math.Ceil(float64(fileSize) / float64(sliceSize)))
	var md5List []string
	for i := 0; i < sliceNum; i++ {
		buffer := make([]byte, sliceSize)
		n, err := file.Read(buffer[:])
		if err != nil && err != io.EOF {
			log.Errorf("读取文件异常: %+v", err)
			return nil, 0, err
		}
		if n == 0 {
			break
		}
		sliceFile, err := os.Create(tmpDir + strconv.Itoa(i))
		if err != nil {
			log.Errorf("创建分片文件异常: %+v", err)
			break
		}
		_, err = sliceFile.Write(buffer[:n])
		if err != nil {
			log.Errorf("写入分片文件异常: %+v", err)
			return nil, 0, err
		}
		hash := md5.New()
		hash.Write(buffer[:n])
		sliceMd5 := hex.EncodeToString(hash.Sum(nil))
		md5List = append(md5List, sliceMd5)
		err = sliceFile.Close()
		if err != nil {
			log.Errorf("关闭文件异常: %+v", err)
			return nil, 0, err
		}
	}
	return md5List, uint64(fileSize), err
}

func getTmpDir(srcPath string) string {
	srcDir := filepath.Dir(srcPath)
	milliseconds := time.Now().UnixNano() / int64(time.Millisecond)
	return srcDir + "/" + strconv.FormatInt(milliseconds, 10) + "/"
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

func PreCreate(accessToken string, arg *PrecreateArg) (PrecreateReturn, error) {
	ret := PrecreateReturn{}

	protocal := "https"
	host := "pan.baidu.com"
	router := "/rest/2.0/xpan/file?method=precreate&"
	uri := protocal + "://" + host + router

	params := url.Values{}
	params.Set("access_token", accessToken)
	uri += params.Encode()

	headers := map[string]string{
		"Host":         host,
		"Content-Type": "application/x-www-form-urlencoded",
	}

	postBody := url.Values{}
	postBody.Add("path", arg.Path)
	postBody.Add("size", strconv.FormatUint(arg.Size, 10))
	blockListJson, _ := json.Marshal(arg.BlockList)
	postBody.Add("block_list", string(blockListJson))
	postBody.Add("isdir", "0")
	postBody.Add("autoinit", "1")
	postBody.Add("rtype", "3")

	body, _, err := DoHTTPRequest(uri, strings.NewReader(postBody.Encode()), headers)
	if err != nil {
		return ret, err
	}
	if err = json.Unmarshal([]byte(body), &ret); err != nil {
		return ret, errors.New("unmarshal precreate body failed,body")
	}
	if ret.Errno != 0 {
		log.Errorf("响应:%+v", body)
		return ret, errors.New("call precreate failed")
	}
	return ret, nil
}

func sliceUpload(accessToken string, arg *UploadArg) (UploadReturn, error) {
	ret := UploadReturn{}

	//打开文件句柄操作
	fileHandle, err := os.Open(arg.LocalFile)
	if err != nil {
		return ret, errors.New("superfile2 open file failed")
	}
	defer fileHandle.Close()

	// 获取文件当前信息
	fileInfo, err := fileHandle.Stat()
	if err != nil {
		return ret, err
	}

	// 读取文件块
	buf := make([]byte, fileInfo.Size())
	n, err := fileHandle.Read(buf)
	if err != nil {
		if err != io.EOF {
			return ret, err
		}
	}

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileWriter, err := bodyWriter.CreateFormFile("file", "file")
	if err != nil {
		return ret, err
	}

	//iocopy
	_, err = io.Copy(fileWriter, bytes.NewReader(buf[0:n]))
	if err != nil {
		return ret, err
	}

	bodyWriter.Close()

	protocal := "https"
	host := "d.pcs.baidu.com"
	router := "/rest/2.0/pcs/superfile2?method=upload&"
	uri := protocal + "://" + host + router

	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("path", arg.Path)
	params.Set("uploadid", arg.UploadId)
	params.Set("partseq", strconv.Itoa(arg.Partseq))
	uri += params.Encode()

	contentType := bodyWriter.FormDataContentType()
	headers := map[string]string{
		"Host":         host,
		"Content-Type": contentType,
	}

	body, _, err := SendHTTPRequest(uri, bodyBuf, headers)
	if err != nil {
		return ret, err
	}
	if err := json.Unmarshal([]byte(body), &ret); err != nil {
		return ret, errors.New("unmarshal body failed")

	}
	if ret.Md5 == "" {
		return ret, errors.New("md5 is empty")

	}
	return ret, nil
}

func CreateFile(accessToken string, arg *CreateArg) (CreateReturn, error) {
	ret := CreateReturn{}

	protocal := "https"
	host := "pan.baidu.com"
	router := "/rest/2.0/xpan/file?method=create&"
	uri := protocal + "://" + host + router

	headers := map[string]string{
		"Host":         host,
		"Content-Type": "application/x-www-form-urlencoded",
	}

	params := url.Values{}
	params.Set("access_token", accessToken)
	uri += params.Encode()

	postBody := url.Values{}
	postBody.Add("rtype", "2")
	postBody.Add("path", arg.Path)
	postBody.Add("size", strconv.FormatUint(arg.Size, 10))
	postBody.Add("isdir", "0")
	js, _ := json.Marshal(arg.BlockList)
	postBody.Add("block_list", string(js))
	postBody.Add("uploadid", arg.UploadId)

	var body string
	var err error

	body, _, err = DoHTTPRequest(uri, strings.NewReader(postBody.Encode()), headers)
	if err != nil {
		return ret, err
	}
	if err = json.Unmarshal([]byte(body), &ret); err != nil {
		return ret, errors.New("unmarshal create body failed")
	}
	if ret.Errno != 0 {
		return ret, errors.New("call create failed")
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
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(respBody), resp.StatusCode, nil
}

func SendHTTPRequest(url string, body io.Reader, headers map[string]string) (string, int, error) {
	timeout := 60 * time.Second
	retryTimes := 3
	postData, _ := io.ReadAll(body)
	var resp *http.Response
	for i := 1; i <= retryTimes; i++ {
		tr := &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost: -1,
		}
		httpClient := &http.Client{Transport: tr}
		httpClient.Timeout = timeout
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
		if err != nil {
			return "", 0, err
		}
		// request header
		for k, v := range headers {
			req.Header.Add(k, v)
		}
		resp, err = httpClient.Do(req)
		if err == nil {
			break
		}
		if i == retryTimes {
			return "", 0, err
		}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
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

type PrecreateArg struct {
	Path      string   `json:"path"`
	Size      uint64   `json:"size"`
	BlockList []string `json:"block_list"`
}

func NewPrecreateArg(path string, size uint64, blockList []string) *PrecreateArg {
	s := new(PrecreateArg)
	s.Path = path
	s.Size = size
	s.BlockList = blockList
	return s
}

type PrecreateReturn struct {
	Errno      int    `json:"errno"`
	ReturnType int    `json:"return_type"`
	BlockList  []int  `json:"block_list"`
	UploadId   string `json:"uploadid"`
	RequestId  int    `json:"request_id"`
}

type UploadArg struct {
	UploadId  string `json:"uploadid"`
	Path      string `json:"path"`
	LocalFile string `json:"local_file"`
	Partseq   int    `json:"partseq"`
}

func NewUploadArg(uploadId string, path string, localFile string, partseq int) *UploadArg {
	s := new(UploadArg)
	s.UploadId = uploadId
	s.Path = path
	s.LocalFile = localFile
	s.Partseq = partseq
	return s
}

type UploadReturn struct {
	Md5       string `json:"md5"`
	RequestId int    `json:"request_id"`
}

type CreateArg struct {
	UploadId  string   `json:"uploadid"`
	Path      string   `json:"path"`
	Size      uint64   `json:"size"`
	BlockList []string `json:"block_list"`
}

func NewCreateArg(uploadId string, path string, size uint64, blockList []string) *CreateArg {
	s := new(CreateArg)
	s.UploadId = uploadId
	s.Path = path
	s.Size = size
	s.BlockList = blockList
	return s
}

type CreateReturn struct {
	Errno int    `json:"errno"`
	Path  string `json:"path"`
}
