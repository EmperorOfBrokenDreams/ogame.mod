package database

import (
	"log"

	//"github.com/0xE232FE/ogame.mod/cmd/ogamed/database/models"

	"github.com/0xE232FE/ogame.mod/cmd/ogamed/database/models"
	//"gorm.io/driver/sqlite"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func InitDatabase() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	if err != nil {
		log.Println(err)
		return nil
	}

	if db != nil {
		db.AutoMigrate(
			&models.User{},
			&models.Session{},
			&models.Server{},
			&models.Bot{},
			&models.UserBot{},
			&models.BotPlanet{},
			&models.Test{},
		)
	}

	return db
}
