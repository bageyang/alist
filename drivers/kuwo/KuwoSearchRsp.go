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
}
