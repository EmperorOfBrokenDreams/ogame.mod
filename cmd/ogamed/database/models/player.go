package models

import (
	"github.com/0xE232FE/ogame.mod"
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	ogame.UserInfos
}
