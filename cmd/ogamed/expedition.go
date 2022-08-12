package main

import (
	"log"
	"math"
	"time"

	"github.com/0xE232FE/ogame.mod"
)

func expeditionLoop(bot *ogame.OGame) {
	for {
		b := bot.BeginNamed("Expedition Bot")
		fleets, slots := b.GetFleets()
		expeditionHandler(b, bot, fleets, slots)
		b.Done()

		if !slots.IsExpeditionPossible() {
			// Wait
			var wait int64
			for _, f := range fleets {
				if f.Mission == ogame.Expedition {
					if wait == 0 {
						if f.ReturnFlight {
							wait = int64(time.Until(f.ArrivalTime).Seconds())
						} else {
							wait = int64(time.Until(f.BackTime).Seconds())
						}
						//log.Printf("%d) Arrival Time: %s (%f Seconds) Backtime: %s (Return flight: %t) ", f.ID, f.ArrivalTime, time.Until(f.ArrivalTime).Seconds(), f.BackTime, f.ReturnFlight)
					}
					if f.ReturnFlight {
						wait = int64(math.Min(float64(wait), float64(time.Until(f.ArrivalTime).Seconds())))
					} else {
						wait = int64(math.Min(float64(wait), float64(time.Until(f.BackTime).Seconds())))
					}
				}
			}
			log.Printf("Waiting for next Expedition: %s", time.Duration(wait)*time.Second)
			time.Sleep(time.Duration(wait) * time.Second)
		}

		wait := time.Duration(random(300, 900)) * time.Second
		log.Printf("Waiting for next Expedition Loop: %s", wait)
		time.Sleep(wait)
	}
}

func expeditionHandler(b ogame.Prioritizable, bot *ogame.OGame, fleets []ogame.Fleet, slots ogame.Slots) {
	if slots.IsExpeditionPossible() {
		// Send Expedition
		celestials := bot.GetCachedCelestials()

		for _, celestial := range celestials {

			resb, fac, ships, _, techs, err := b.GetTechs(celestial.GetID())
			if err != nil {
				log.Println(err)
				return
			}

			resd, err := b.GetResourcesDetails(celestial.GetID())
			if err != nil {
				log.Println(err)
				return
			}

			ships = ships.Div(slots.GetExpeditionPossible())

			minExpedtionFleet := ogame.ShipsInfos{
				SmallCargo:     1,
				EspionageProbe: 1,
			}

			if !ships.Has(minExpedtionFleet) {
				_, wait, err := b.GetProduction(celestial.GetID())
				if err != nil {
					log.Println(err)
					return
				}
				if wait > 0 {
					log.Printf("Fleet/Defense Production already in Progres %s", time.Duration(wait)*time.Second)
					return
				}

				sc := ogame.SmallCargo
				probe := ogame.EspionageProbe

				price := sc.GetPrice(slots.GetExpeditionPossible())
				price = price.Add(probe.GetPrice(slots.GetExpeditionPossible()))

				if resd.Available().CanAfford(price) && sc.IsAvailable(celestial.GetType(), resb.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) && probe.IsAvailable(celestial.GetType(), resb.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) {
					err := b.Build(celestial.GetID(), sc.ID, slots.GetExpeditionPossible())
					if err != nil {
						log.Println(err)
						return
					}

					err = b.Build(celestial.GetID(), probe.ID, slots.GetExpeditionPossible())
					if err != nil {
						log.Println(err)
						return
					}
					_, wait, err := b.GetProduction(celestial.GetID())
					if err != nil {
						log.Println(err)
						return
					}
					log.Printf("Build Needed Ships for minimum Expeditions and wait %s", time.Duration(wait)*time.Second)
					//time.Sleep(time.Duration(wait+1) * time.Second)
					return
				}
				return
			}

			if !ships.HasFlyableShips() {
				return
			}

			var bestShip ogame.Ship

			for _, s := range ogame.Ships {
				if s.GetID() == ogame.DeathstarID || s.GetID() == ogame.RecyclerID || s.GetID() == ogame.ColonyShipID || s.GetID() == ogame.CrawlerID || s.GetID() == ogame.SolarSatelliteID || s.GetID() == ogame.LargeCargoID || s.GetID() == ogame.SmallCargoID || s.GetID() == ogame.PathfinderID {
					continue
				}

				if s.GetID().IsFlyableShip() && ships.ByID(s.GetID()) > slots.GetExpeditionPossible() {
					ships.Set(s.GetID(), 0)
					if bestShip == nil {
						bestShip = s
						continue
					}
					if s.GetPrice(1).Value() > bestShip.GetPrice(1).Value() {
						bestShip = s
					}
				}
			}

			if bestShip != nil {
				if bestShip.GetID() > 0 {
					ships.Set(bestShip.GetID(), 1)
				}

			}

			if ships.ByID(ogame.PathfinderID) > slots.GetExpeditionPossible() {
				ships.Set(ogame.PathfinderID, 1)
			}

			ships.Add(minExpedtionFleet)

			log.Printf("Possible Expeditions %d", slots.GetExpeditionPossible())
			for i := slots.GetExpeditionPossible(); i > 0; i-- {
				//origin ogame.Coordinate, destination ogame.Coordinate, speed ogame.Speed, ships ogame.ShipsInfos, mission ogame.MissionID

				dest := celestial.GetCoordinate()
				dest.Position = 16
				dest.Type = ogame.PlanetType
				_, fuel := b.FlightTime(celestial.GetCoordinate(), dest, ogame.HundredPercent, ships, ogame.Expedition, 1)
				if fuel*2 > resd.Available().Deuterium {
					log.Printf("Nicht genügend Treibstoff (140026)")
					b.Done()
					time.Sleep(time.Duration(random(1800, 3600)) * time.Second)
					return
				}
				fleet, err := b.SendFleet(celestial.GetID(), ships.ToQuantifiables(), ogame.HundredPercent, dest, ogame.Expedition, ogame.Resources{}, 1, 0)
				if err != nil {
					log.Println(err)
					return
				}

				//func (ogame.Prioritizable).SendFleet(celestialID ogame.CelestialID, ships []ogame.Quantifiable, speed ogame.Speed, where ogame.Coordinate, mission ogame.MissionID, resources ogame.Resources, holdingTime int64, unionID int64

				// fb := ogame.NewFleetBuilder(bot)
				// fb.SetShips(ships)
				// fb.SetOrigin(celestial.GetCoordinate())
				// dest := celestial.GetCoordinate()
				// dest.Position = 16
				// dest.Type = ogame.PlanetType
				// fb.SetDestination(dest)
				// fb.SetMission(ogame.Expedition)
				// fb.SetSpeed(ogame.HundredPercent)
				// fb.SetDuration(1)
				// if slots.GetExpeditionPossible() == 1 {
				// 	fb.SetAllShips()
				// }
				// fleet, err := fb.SendNow()
				// if err != nil {
				// 	log.Println(err)
				// 	return
				// }
				log.Printf("Send Expedition Succeed %d", fleet.ID)
				time.Sleep(time.Duration(random(3, 6)) * time.Second)
			}
		}
	}
}

func expedition(bot *ogame.OGame) error {
	celestials := bot.GetCachedCelestials()
	for _, celestial := range celestials {
		//(ogame.ResourcesBuildings, ogame.Facilities, ogame.ShipsInfos, ogame.DefensesInfos, ogame.Researches, error)
		resd, err := celestial.GetResourcesDetails()
		if err != nil {
			return err
		}
		resb, fac, ships, _, techs, err := bot.GetTechs(celestial.GetID())
		if err != nil {
			return err
		}

		if !ships.HasFlyableShips() {
			continue
		}

		if ships.SmallCargo < 3 && ships.EspionageProbe < 3 {
			sc := ogame.SmallCargo
			probe := ogame.EspionageProbe

			price := sc.GetPrice(3)
			price = price.Add(probe.GetPrice(3))

			if resd.Available().CanAfford(price) && sc.IsAvailable(celestial.GetType(), resb.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) && probe.IsAvailable(celestial.GetType(), resb.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) {
				err := celestial.Build(sc.ID, 3)
				if err != nil {
					return err
				}
				err = celestial.Build(probe.ID, 3)
				if err != nil {
					return err
				}
				_, wait, err := celestial.GetProduction()
				if err != nil {
					return err
				}
				log.Printf("Build Needed Ships for minimum Expeditions and wait %s", time.Duration(wait)*time.Second)
				time.Sleep(time.Duration(wait) * time.Second)
			}
			return nil
		} else {
			// var expoShips = ogame.ShipsInfos{}

			// objShips := ogame.Ships
			// for _, o := range objShips {
			// 	if o.GetID() == ogame.SolarSatelliteID || o.GetID() == ogame.CrawlerID || o.GetID() == ogame.ColonyShipID || o.GetID() == ogame.DeathstarID || o.GetID() == ogame.RecyclerID {
			// 		continue
			// 	} else {
			// 		if expoShips.ByID(o.GetID()) >= NbrExposPossible {
			// 			expoShips.Set(o.GetID(), 1)
			// 		}
			// 	}
			// }

			// expoShips.LargeCargo = expoShipsDiv.LargeCargo
			// expoShips.SmallCargo = expoShipsDiv.SmallCargo

			for i := 0; i > 0; i-- {
				fb := ogame.NewFleetBuilder(bot)
				fb.SetShips(ships)
				fb.SetOrigin(celestial.GetCoordinate())
				dest := celestial.GetCoordinate()
				dest.Position = 16
				dest.Type = ogame.PlanetType
				fb.SetDestination(dest)
				fb.SetMission(ogame.Expedition)
				fb.SetSpeed(ogame.HundredPercent)
				fb.SetDuration(1)
				fleet, err := fb.SendNow()
				if err != nil {
					return err
				}
				log.Printf("Send Expedition Succeed %d", fleet.ID)
				time.Sleep(time.Duration(random(3, 6)) * time.Second)
			}
			return nil
		}
	}
	return nil
}
