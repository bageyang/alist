package kuwo

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/alist-org/alist/v3/internal/conf"
	"github.com/alist-org/alist/v3/internal/fs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/internal/stream"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/alist-org/alist/v3/server/common"
	"github.com/bogem/id3v2"
	"github.com/redis/go-redis/v9"
	"image"
	"image/jpeg"
	"os"
	"sync"
	"time"
)

const MusicTask = "music:task:kuwo-queue"

var CacheDir = "/opt/cache/"
var IsRuning = false

var clientMu sync.Mutex

var uploadPah = "/"

type FileDeleteCloser struct {
	deletePath *os.File
}

func (f FileDeleteCloser) Close() error {
	//err := os.Remove(f.deletePath)
	utils.Log.Infof("删除文件: %+v", f.deletePath)
	return f.deletePath.Close()
}

func init() {
	go watchUploadConfig()
}

func watchUploadConfig() {
	time.Sleep(time.Second * 30)
	for {
		time.Sleep(time.Second * 30)
		groups, err := op.GetSettingItemsByGroup(model.MUSIC)
		if err != nil {
			utils.Log.Errorf("配置查询失败: %+v", err)
			continue
		}
		for _, group := range groups {
			key := group.Key
			value := group.Value
			if key == conf.UploadPath && len(value) > 0 {
				if uploadPah != value {
					uploadPah = value
					utils.Log.Infof("更新上传目录: %+v", uploadPah)
				}
				continue
			}
			if key == conf.MusicCachePath && len(value) > 0 {
				if CacheDir != value {
					CacheDir = value
					utils.Log.Infof("更新临时目录: %+v", CacheDir)
				}
				continue
			}
		}
		time.Sleep(time.Second * 30)
	}
}

func HandTask(musicList []MusicInfo) bool {
	client := common.GetRedisClient()
	var ctx = context.Background()
	for _, info := range musicList {
		jsonData, err := json.Marshal(info)
		if err != nil {
			utils.Log.Errorf("添加任务队列异常: %+v", err)
		}
		_, err = client.LPush(ctx, MusicTask, jsonData).Result()
		if err != nil {
			utils.Log.Errorf("添加任务队列异常: %+v", err)
		}
	}
	checkTask()
	return true
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
			utils.Log.Info("暂时无任务,结束。。。")
			break
		} else if err != nil {
			panic(err)
		} else {
			music := new(MusicInfo)
			json.Unmarshal([]byte(musicStr), music)
			utils.Log.Infof("开始任务: %+v", music.Name)
			downloadAndUploadMusic(music)
		}
	}
	clientMu.Lock()
	IsRuning = false
	clientMu.Unlock()
}

func downloadAndUploadMusic(music *MusicInfo) {
	musicOutput := downloadMusic(music)
	fileInfo, err := os.Stat(musicOutput)
	file, err := os.Open(musicOutput)
	if err != nil {
		utils.Log.Infof("打开文件失败: %+v", err)
	}
	closers := utils.Closers{}
	closer := FileDeleteCloser{file}
	closers.Add(closer)
	s := &stream.FileStream{
		Obj: &model.Object{
			Name:     music.Name,
			Size:     fileInfo.Size(),
			Modified: time.Now(),
		},
		//Reader:       file,
		WebPutAsTask: true,
		Closers:      closers,
	}
	s.SetTmpFile(file)
	fs.PutAsTask(uploadPah, s)
}

func downloadMusic(music *MusicInfo) string {
	rid := music.Rid
	musicRid := music.MusicRid
	format := GetMusicFormat(musicRid)
	kuwoPlay := GetDownloadUrl(rid, format.Format, format.Bitrate)
	url := kuwoPlay.Url
	fileName := music.Artist + "-" + music.Name + "." + format.Format
	music.Name = fileName
	musicOutput := CacheDir + fileName
	_, err := client.R().SetOutput(CacheDir + music.Name).Get(url)
	if err != nil {
		utils.Log.Errorf("下载歌曲失败: %+v", err)
	} else {
		utils.Log.Infof("下载歌曲成功: %+v", musicOutput)
	}
	picList := SearchMusicPic(rid)
	picOutput := CacheDir + music.Rid + ".jpg"
	_, picErr := client.R().SetOutput(picOutput).Get(picList[0])
	if picErr != nil {
		utils.Log.Errorf("下载图片失败: %+v", picErr)
	} else {
		utils.Log.Infof("下载图片成功: %+v", picOutput)
	}
	lrc := GetMusicLrc(rid)
	FillMusicId3Tag(musicOutput, picOutput, lrc, *music)
	err = os.Remove(picOutput)
	if err != nil {
		utils.Log.Infof("删除图片失败: %+v", picOutput)
	}
	return musicOutput
}

func FillMusicId3Tag(musicPath string, imagePath string, lyrics string, musicInfo MusicInfo) {
	tag, err := id3v2.Open(musicPath, id3v2.Options{Parse: true})
	if err != nil {
		utils.Log.Errorf("打开文件错误: %+v", err)
		return
	}

	tag.SetTitle(musicInfo.Name)
	tag.SetArtist(musicInfo.Artist)
	tag.SetAlbum(musicInfo.Album)

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

	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
		Encoding: id3v2.EncodingUTF8,
		Language: "chi",
		Lyrics:   lyrics,
	})

	if err = tag.Save(); err != nil {
		utils.Log.Errorf("写入标签错误: %+v", err)
	}

	err = tag.Close()
	if err != nil {
		return
	}
}

func ListUnHandTask() []MusicInfo {
	ctx := context.Background()
	redisClient := common.GetRedisClient()
	result, err := redisClient.LRange(ctx, MusicTask, 0, 9).Result()
	if err == redis.Nil {
		utils.Log.Errorf("获取任务队列错误: %+v", err)
	}
	size := len(result)
	if size == 0 {
		return make([]MusicInfo, 0)
	}
	musicList := make([]MusicInfo, size)
	for _, s := range result {
		music := new(MusicInfo)
		json.Unmarshal([]byte(s), music)
		musicList = append(musicList, *music)
	}
	return musicList
}
