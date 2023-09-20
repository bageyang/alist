package kuwo

type SearchRsp struct {
	Total   string      `json:"TOTAL"`
	AbsList []MusicInfo `json:"abslist"`
}

type MusicInfo struct {
	Name     string `json:"NAME"`
	Album    string `json:"ALBUM"`
	Artist   string `json:"ARTIST"`
	Rid      string `json:"DC_TARGETID"`
	DURATION string `json:"DURATION"`
	Thumb    string `json:"hts_MVPIC"`
	MusicRid string `json:"MUSICRID"`
	Formats  string `json:"FORMATS"`
	Extra    string `json:"extra"`
}

type KuwoPlay struct {
	Format  string `json:"format"`
	Bitrate string `json:"bitrate"`
	Url     string `json:"url"`
	Sig     string `json:"sig"`
	Rid     string `json:"rid"`
}

type FormatInfo struct {
	Format  string `json:"format"`
	Bitrate string `json:"bitrate"`
	Size    int64  `json:"size"`
}

type LrcRsp struct {
	Status  int     `json:"status"`
	LrcData LrcWarp `json:"data"`
}

type LrcWarp struct {
	LrcList []KuwoLrc `json:"lrclist"`
}

type KuwoLrc struct {
	LineLyric string `json:"lineLyric"`
	Time      string `json:"time"`
}

type KuwoPicRsp struct {
	PicList []map[string]string `json:"array"`
}

type Ret[T any] struct {
	Code int    `json:"code"`
	Data T      `json:"data"`
	Msg  string `json:"msg"`
}

type TaskRet struct {
	ProcessList []MusicInfo `json:"processList"`
	ErrorList   []MusicInfo `json:"errorList"`
}
