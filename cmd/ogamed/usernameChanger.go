package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/0xE232FE/ogame.mod"
)

func UserNameChanger(bot *ogame.OGame) {
	for {
		if !bot.IsLoggedIn() {
			time.Sleep(5 * time.Second)
			continue
		}
		by, err := os.ReadFile("usernamestealer.json")
		if err != nil {
			log.Println(err)
			return
		}

		userStealer := struct {
			Username   string
			Coordinate ogame.Coordinate
		}{}

		err = json.Unmarshal(by, &userStealer)
		if err != nil {
			log.Println(err)
			return
		}

		if userStealer.Username == bot.Player.PlayerName {
			return
		}

		if userStealer.Coordinate.Galaxy > bot.GetServer().Settings.UniverseSize {
			log.Println("Galaxy out of Range")
		}

		galaxy, err := bot.GalaxyInfos(userStealer.Coordinate.Galaxy, userStealer.Coordinate.System)
		if galaxy.Position(userStealer.Coordinate.Position) == nil && err == nil {
			log.Printf("Player %s deleted change username now", galaxy.Position(userStealer.Coordinate.Position).Player.Name)
			bot.ChangeUsername(userStealer.Username)
			return
		}

		if err == nil {
			log.Printf("Checking for Player %s (%d) at %s", galaxy.Position(userStealer.Coordinate.Position).Player.Name, galaxy.Position(userStealer.Coordinate.Position).ID, galaxy.Position(userStealer.Coordinate.Position).Coordinate)
		} else {
			log.Printf("%s", err)
		}

		dur := time.Duration(random(900, 1800)) * time.Second
		log.Printf("Wait %s", dur)
		time.Sleep(dur)
	}
}
