package models

import (
	"github.com/0xE232FE/ogame.mod"
	"gorm.io/gorm"
)

type Fleet struct {
	gorm.Model
	ogame.Fleet `gorm:"embedded"`
}
