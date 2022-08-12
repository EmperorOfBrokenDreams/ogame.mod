package models

import "github.com/0xE232FE/ogame.mod"

type Test struct {
	ID             uint
	MyType         `gorm:"-"`
	ogame.PlanetID `gorm:"type:uint"`
}

type MyType int64
