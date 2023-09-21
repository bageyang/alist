package kuwo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alist-org/alist/v3/drivers/baidu_netdisk"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/bogem/id3v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MusicTask = "music:task:kuwo-queue"
const MusicErrTask = "music:task:error-queue"

var CacheDir = "/opt/cache/"
var IsRuning = false

var clientMu sync.Mutex

var uploadPah = "/"

type FileDeleteCloser struct {
	deletePath *os.File
	modelMusic *model.Music
}

func (f FileDeleteCloser) Close() error {
	err := db.CreateMusicRecord(f.modelMusic)
	if err != nil {
		log.Errorf("新增歌曲记录异常: %+v", err)
	}
	log.Infof("删除文件: %+v", f.deletePath)
	return f.deletePath.Close()
}

func init() {
	go watchUploadConfig()
}

func watchUploadConfig() {
	time.Sleep(time.Second * 10)
	for {
		groups, err := op.GetSettingItemsByGroup(model.MUSIC)
		if err != nil {
			log.Errorf("配置查询失败: %+v", err)
			continue
		}
		for _, group := range groups {
			key := group.Key
			value := group.Value
			if key == conf.UploadPath && len(value) > 0 {
				if uploadPah != value {
					uploadPah = value
					log.Infof("更新上传目录: %+v", uploadPah)
				}
				continue
			}
			if key == conf.MusicCachePath && len(value) > 0 {
				if CacheDir != value {
					CacheDir = value
					log.Infof("更新临时目录: %+v", CacheDir)
				}
				continue
			}
		}
		time.Sleep(time.Second * 30)
	}
}

func HandTask(musicList []MusicInfo) bool {
	redisClient := common.GetRedisClient()
	var ctx = context.Background()
	for _, info := range musicList {
		jsonData, err := json.Marshal(info)
		if err != nil {
			log.Errorf("序列化任务异常: %+v", err)
			continue
		}
		_, err = redisClient.LPush(ctx, MusicTask, jsonData).Result()
		if err != nil {
			log.Errorf("添加任务队列异常: %+v", err)
			continue
		}
	}
	checkTask()
	return true
}

func CleanFailTask() {
	ctx := context.Background()
	redisClient := common.GetRedisClient()
	_, err := redisClient.Del(ctx, MusicErrTask).Result()
	if err != nil {
		log.Errorf("清除失败队列异常: %+v", err)
	}
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
		musicStr, err := redisClient.LPop(ctx, MusicTask).Result()
		if err == redis.Nil {
			log.Info("暂时无任务,结束。。。")
			break
		} else if err != nil {
			log.Errorf("redis 获取数据错误 %+v", err)
			log.Error("redis 获取数据错误 结束本次任务")
			break
		} else {
			music := new(MusicInfo)
			err := json.Unmarshal([]byte(musicStr), music)
			if err != nil {
				log.Infof("解析任务失败: %+v", musicStr)
				continue
			}
			name := music.Name
			artist := music.Artist
			log.Infof("开始任务: %+v", music.Name)
			exist, modelMusic := db.ExistMusicByNameAndArtist(name, artist)
			if exist {
				log.Infof("音乐已存在: %+v", modelMusic)
				continue
			}
			log.Infof("新曲目,开始下载")
			err = downloadAndUploadMusic(music)
			if err != nil {
				s := err.Error()
				music.Extra = s
				jsonData, err := json.Marshal(music)
				if err != nil {
					log.Errorf("写入异常信息失败: %+v", err)
					continue
				}
				_, err = redisClient.LPush(ctx, MusicErrTask, jsonData).Result()
				if err != nil {
					log.Errorf("添加异常队列异常: %+v", err)
					continue
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
	log.Info("队列暂无下载任务,结束。。。")
	clientMu.Lock()
	IsRuning = false
	clientMu.Unlock()
}

func downloadAndUploadMusic(music *MusicInfo) error {
	orgName := music.Name
	musicOutput, fileSize, err := downloadMusic(music)
	if err != nil {
		return err
	}
	//fileInfo, err := os.Stat(musicOutput)
	//file, err := os.Open(musicOutput)
	//if err != nil {
	//	log.Infof("打开文件失败: %+v", err)
	//	return errors.New("打开文件失败")
	//}
	//closers := utils.Closers{}
	//m := new(model.Music)
	//m.Name = orgName
	//m.SourcesId = music.Rid
	//m.Album = music.Album
	//m.Artist = music.Artist
	//m.Size = fileSize
	//m.Duration = music.DURATION
	//closer := FileDeleteCloser{file, m}
	//closers.Add(closer)
	//s := &stream.FileStream{
	//	Obj: &model.Object{
	//		Name:     music.Name,
	//		Size:     fileInfo.Size(),
	//		Modified: time.Now(),
	//	},
	//	//Reader:       file,
	//	WebPutAsTask: true,
	//	Closers:      closers,
	//}
	//s.SetTmpFile(file)

	//err = fs.PutAsTask(uploadPah, s)
	//if err != nil {
	//	log.Infof("添加上传任务失败: %+v", err)
	//	return errors.New("打开文件失败")
	//}
	err = uploadFile(music.Name, musicOutput)
	if err != nil {
		return err
	}
	m := new(model.Music)
	m.Name = orgName
	m.SourcesId = music.Rid
	m.Album = music.Album
	m.Artist = music.Artist
	m.Size = fileSize
	m.Duration = music.DURATION
	err = db.CreateMusicRecord(m)
	if err != nil {
		log.Errorf("新增歌曲记录异常: %+v", err)
	}
	err = os.Remove(musicOutput)
	if err != nil {
		log.Errorf("新增歌曲文件异常: %+v", err)
	}
	return nil
}

func uploadFile(fileName string, musicPath string) error {
	log.Infof("开始上传%+v", musicPath)
	driver, err := op.GetStorageByMountPath(uploadPah)
	if err != nil {
		log.Errorf("获取驱动失败:%+v", err)
		return err
	}
	addition := driver.GetAddition()
	baiduAddition, ok := addition.(*baidu_netdisk.Addition)
	if !ok {
		log.Errorf("无法找到路径对应baidu授权信息:%+v", reflect.TypeOf(addition))
		return errors.New("无法获取baidu授权信息")
	}
	accessToken := baiduAddition.AccessToken
	diskPath := "/apps/Alist/"
	if !strings.HasSuffix(diskPath, "/") {
		diskPath = diskPath + "/"
	}
	diskPath = diskPath + fileName
	url := fmt.Sprintf("https://d.pcs.baidu.com/rest/2.0/pcs/file?method=upload&ondup=overwrite&access_token=%s&path=%s", accessToken, diskPath)
	res, err := client.R().SetFile("file", musicPath).Post(url)
	if err != nil {
		log.Errorf("上传失败:%+v", err)
		return err
	}
	errCode := utils.Json.Get(res.Body(), "error_code").ToInt()
	errNo := utils.Json.Get(res.Body(), "errno").ToInt()
	if errCode != 0 || errNo != 0 {
		log.Errorf("上传失败:%+v", res.String())
		return errs.NewErr(errs.StreamIncomplete, "上传失败:", res.String())
	}
	log.Infof("上传成功")
	moveMusic(baiduAddition, diskPath, uploadPah, fileName)
	return nil
}

type FileManager struct {
	Path    string `json:"path"`
	Dest    string `json:"dest"`
	Newname string `json:"newname"`
	Ondup   string `json:"ondup"`
}

func moveMusic(addition *baidu_netdisk.Addition, srcPath string, distPath string, fileName string) {
	if !strings.HasSuffix(distPath, "/") {
		distPath = distPath + "/"
	}
	url := "http://pan.baidu.com/rest/2.0/xpan/file"
	params := map[string]string{
		"method":       "filemanager",
		"access_token": addition.AccessToken,
		"opera":        "mover",
	}
	fileManager := FileManager{
		Path:    srcPath,
		Dest:    distPath,
		Newname: fileName,
		Ondup:   "overwrite",
	}
	fileManagerArr := []FileManager{fileManager}
	log.Infof("请求移动文件from: %s  => %s%s", srcPath, distPath, fileName)
	// 在请求中设置需要提交的form数据
	rsp, err := client.R().
		SetFormData(params).
		SetBody(map[string]interface{}{
			"async":    2,
			"filelist": fileManagerArr,
		}).
		Post(url)
	if err != nil {
		log.Errorf("移动文件失败:%+v", err)
	}
	errNo := utils.Json.Get(rsp.Body(), "errno").ToInt()
	if errNo != 0 {
		log.Errorf("移动文件失败:%+v", rsp.String())
	}
	log.Infof("移动文件成功%s%s", distPath, fileName)
}

func downloadMusic(music *MusicInfo) (string, int64, error) {
	rid := music.Rid
	musicRid := music.MusicRid
	format := GetMusicFormat(musicRid)
	if format == nil {
		return "", 0, errors.New("下载歌曲失败")
	}
	kuwoPlay := GetDownloadUrl(rid, format.Format, format.Bitrate)
	if kuwoPlay == nil {
		return "", 0, errors.New("下载歌曲失败")
	}
	url := kuwoPlay.Url
	fileName := music.Artist + "-" + music.Name + "." + format.Format
	music.Name = fileName
	musicOutput := CacheDir + fileName
	_, err := client.R().SetOutput(CacheDir + music.Name).Get(url)
	if err != nil {
		log.Errorf("下载歌曲失败: %+v", err)
		return "", 0, errors.New("下载歌曲失败")
	} else {
		log.Infof("下载歌曲成功: %+v", musicOutput)
	}
	picList := SearchMusicPic(rid)
	picOutput := CacheDir + music.Rid + ".jpg"
	if picList == nil {
		picOutput = ""
	} else {
		_, picErr := client.R().SetOutput(picOutput).Get(picList[0])
		if picErr != nil {
			log.Errorf("下载图片失败: %+v", picErr)
			picOutput = ""
		} else {
			log.Infof("下载图片成功: %+v", picOutput)
		}
	}
	lrc := GetMusicLrc(rid)
	FillMusicId3Tag(musicOutput, picOutput, lrc, *music)
	err = os.Remove(picOutput)
	if err != nil {
		log.Infof("删除图片失败: %+v", picOutput)
	}
	return musicOutput, format.Size, nil
}

func FillMusicId3Tag(musicPath string, imagePath string, lyrics string, musicInfo MusicInfo) {
	tag, err := id3v2.Open(musicPath, id3v2.Options{Parse: true})
	if err != nil {
		log.Errorf("打开文件错误: %+v", err)
		return
	}

	tag.SetTitle(musicInfo.Name)
	tag.SetArtist(musicInfo.Artist)
	tag.SetAlbum(musicInfo.Album)

	if len(imagePath) > 0 {
		imgFile, _ := os.Open(imagePath)
		defer func(imgFile *os.File) {
			err := imgFile.Close()
			if err != nil {

			}
		}(imgFile)
		img, _, _ := image.Decode(imgFile)
		buf := new(bytes.Buffer)
		_ = jpeg.Encode(buf, img, nil)
		artwork := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    "image/jpeg",
			PictureType: id3v2.PTFrontCover,
			Description: "Front cover",
			Picture:     buf.Bytes(),
		}
		tag.AddAttachedPicture(artwork)
	}

	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
		Encoding: id3v2.EncodingUTF8,
		Language: "chi",
		Lyrics:   lyrics,
	})

	if err = tag.Save(); err != nil {
		log.Errorf("写入标签错误: %+v", err)
	}

	err = tag.Close()
	if err != nil {
		log.Errorf("写入标签错误: %+v", err)
		return
	}
}

func ListUnHandTask() TaskRet {
	ctx := context.Background()
	redisClient := common.GetRedisClient()
	result, err := redisClient.LRange(ctx, MusicTask, 0, 99).Result()
	if err != nil {
		log.Errorf("获取任务队列错误: %+v", err)
		return *new(TaskRet)
	}
	errResult, err := redisClient.LRange(ctx, MusicErrTask, 0, 99).Result()
	if err != nil {
		log.Errorf("获取任务队列错误: %+v", err)
		return *new(TaskRet)
	}
	size := len(result)
	errorSize := len(errResult)
	if size == 0 && errorSize == 0 {
		return *new(TaskRet)
	}
	ret := new(TaskRet)
	musicList := make([]MusicInfo, 0)
	for _, s := range result {
		music := new(MusicInfo)
		err := json.Unmarshal([]byte(s), music)
		if err != nil {
			log.Error("json 反序列化错误: %+v", err)
			continue
		}
		musicList = append(musicList, *music)
	}
	errorList := make([]MusicInfo, 0)
	for _, s := range errResult {
		music := new(MusicInfo)
		err := json.Unmarshal([]byte(s), music)
		if err != nil {
			log.Error("json 反序列化错误: %+v", err)
			continue
		}
		errorList = append(errorList, *music)
	}
	ret.ErrorList = errorList
	ret.ProcessList = musicList
	return *ret
}
