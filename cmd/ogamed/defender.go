package main

import (
	"log"
	"time"

	"github.com/alaingilbert/ogame"
)

func defender(bot *ogame.OGame) {
	type defenderAttackTimes struct {
		Nbr        int64
		Celestial  ogame.Celestial
		earliestAt time.Time
		latestAt   time.Time
		attack     ogame.AttackEvent
	}
	var SavedAttacks map[int64]ogame.AttackEvent = map[int64]ogame.AttackEvent{}
	var ignoreUser map[int64]bool = map[int64]bool{}

	ignoreUser[136981] = true

	for {
		attacks, err := bot.GetAttacks()

		if err != nil {
			time.Sleep(time.Duration(random(300, 600)))
			continue
		}
		if len(attacks) > 0 {

			var defenderAttackTimes defenderAttackTimes = defenderAttackTimes{}
			for _, a := range attacks {

				if _, exists := ignoreUser[a.AttackerID]; exists {
					log.Printf("Ignore Player ID: %d", a.AttackerID)
					continue
				}
				for _, cel := range bot.GetCachedCelestials() {
					if a.Destination.Equal(cel.GetCoordinate()) {
						defenderAttackTimes.Celestial = cel
					}
				}
			}

			for _, a := range attacks {
				if _, ok := SavedAttacks[a.ID]; !ok {
					cel := bot.GetCachedCelestialByCoord(a.Destination)

					ships, err := cel.GetShips()
					if err != nil {
						continue
					}

					if !ships.HasFlyableShips() {
						continue
					}

					fn := func() {
						fb := ogame.NewFleetBuilder(bot)
						fb.SetOrigin(a.Destination)
						ships, err := cel.GetShips()
						if err != nil {
							delete(SavedAttacks, a.ID)
							return
						}
						fb.SetShips(ships)
						resources, err := cel.GetResources()
						if err != nil {
							delete(SavedAttacks, a.ID)
							return
						}

						destination := ogame.Coordinate{}
						moons := bot.GetCachedMoons()
						if len(moons) > 0 {
							for _, m := range moons {
								if !m.Coordinate.Equal(a.Destination) {
									destination = m.Coordinate
								}
							}
						}
						if destination.Equal(ogame.Coordinate{}) {
							planets := bot.GetCachedPlanets()
							for _, p := range planets {
								if !p.Coordinate.Equal(a.Destination) {
									destination = p.Coordinate
								}
							}
						}
						fb.SetDestination(destination)
						fb.SetAllResources()
						fb.SetSpeed(ogame.TenPercent)

						_, fuel := fb.FlightTime()

						if resources.Deuterium < fuel {
							delete(SavedAttacks, a.ID)
						}
						fb.SetMission(ogame.Park)

						if !destination.Equal(ogame.Coordinate{}) {
							escapedFleet, err := fb.SendNow()
							if escapedFleet.ID == 0 && err != nil {
								return
							}
						}
						delete(SavedAttacks, a.ID)
					}
					SavedAttacks[a.ID] = a
					wait := time.Until(a.ArrivalTime) - (time.Duration(random(30, 60)) * time.Second)
					log.Printf("Attack incoming detected, saving Fleet in %s ", wait)
					time.AfterFunc(wait, fn)
				}
			}
		}

		nextCheck := time.Duration(random(300, 600) * int64(time.Second))
		log.Printf("Next Defender Check in %s at %s", nextCheck, time.Now().Add(nextCheck))
		time.Sleep(nextCheck)
	}
}
