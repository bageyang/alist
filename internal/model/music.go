package model

type Music struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	SourcesId string `json:"sources_id" gorm:"type:varchar(64)"`
	Name      string `json:"name" binding:"required" gorm:"type:varchar(64)"`
	Artist    string `json:"artist" gorm:"type:varchar(64)"`
	Size      int64  `json:"size"`
	Album     string `json:"album" gorm:"type:varchar(64)"`
	Duration  string `json:"duration" gorm:"type:varchar(16)"`
}
