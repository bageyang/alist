package db

import (
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/pkg/errors"
)

func ExistMusicByNameAndArtist(name string, artist string) (bool, *model.Music) {
	music := model.Music{Name: name, Artist: artist}
	if err := db.Where(music).First(&music).Error; err != nil {
		return false, nil
	} else {
		return true, &music
	}
}

func CreateMusicRecord(music *model.Music) error {
	return errors.WithStack(db.Create(music).Error)
}
