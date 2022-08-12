package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/0xE232FE/ogame.mod"
	"github.com/0xE232FE/ogame.mod/cmd/ogamed/ogb"
	"github.com/labstack/echo"
)

var botPlanets map[ogame.CelestialID]time.Time = map[ogame.CelestialID]time.Time{}

func GetBuildingQueueHandler(c echo.Context) error {
	bot := c.Get("bot").(*ogame.OGame)
	database := c.Get("database").(*ogb.Ogb)
	cache, _ := json.Marshal(database)
	db := ogb.New()
	json.Unmarshal(cache, &db)

	bot.GetCachedCelestials()
	return c.JSON(http.StatusOK, ogame.SuccessResp(botPlanets))
}

func builder(bot *ogame.OGame) {
	// bot.RegisterHTMLInterceptor(func(method string, url string, params url.Values, payload url.Values, pageHTML []byte) {
	// 	e := bot.GetExtractor()

	// 	ConstructionDuration := map[string]time.Duration{}
	// 	ReasearchDuration := map[string]time.Duration{}
	// 	ProductionDuration := map[string]time.Duration{}

	// 	if !ogame.IsAjaxPage(params) && isLogged(pageHTML) {
	// 		page := params.Get("page")
	// 		component := params.Get("component")
	// 		if page != "standalone" && component != "empire" {
	// 			if page == "ingame" {
	// 				page = component
	// 			}

	// 			if ogame.IsKnowFullPage(params) {
	// 				e.ExtractPlanetID(pageHTML)
	// 				e.ExtractPlanetType(pageHTML)
	// 				e.ExtractPlanetCoordinate(pageHTML)
	// 			}

	// 			if page == ogame.SuppliesPage {
	// 				e.ExtractResourcesBuildings(pageHTML)
	// 			}
	// 			if page == ogame.FacilitiesPage {
	// 				e.ExtractFacilities(pageHTML)
	// 			}
	// 		}
	// 	}
	// })

	for {
		for nextRunKey, nextRun := range botPlanets {
			if time.Now().After(nextRun) {
				log.Printf("Delete Next Run at %s for Planet %d", nextRun.String(), nextRunKey)
				delete(botPlanets, nextRunKey)
			}
		}
		for _, c := range bot.GetCachedCelestials() {
			if _, ok := botPlanets[c.GetID()]; ok {
				// Skip if Planet already have timer
				continue
			}

			if c.GetCoordinate().IsMoon() {
				continue
			}

			constID, constSec, researchingID, _ := c.ConstructionsBeingBuilt()
			if constSec > 0 {
				waitConstSec := time.Duration(constSec) * time.Second
				botPlanets[c.GetID()] = time.Now().Add(waitConstSec)
				log.Printf("%s waiting %s", constID, waitConstSec)
				continue
			}

			res, fac, _, _, techs, err := bot.GetTechs(c.GetID())
			if err != nil {
				log.Println(err)
				break
			}
			resd, err := c.GetResourcesDetails()
			if err != nil {
				log.Println(err)
				break
			}

			constructionID, constructionSec, _, _ := c.ConstructionsBeingBuilt()
			waitConstruction := time.Duration(constructionSec) * time.Second
			if waitConstruction.Seconds() > 0 {
				log.Printf("%s waiting %s", constructionID, waitConstruction)
				//time.Sleep(waitConstruction)
				// break
				botPlanets[c.GetID()] = time.Now().Add(waitConstruction)
				continue
			}

			_, shipyardConstructionSec, err := c.GetProduction()
			if err != nil {
				log.Println(err)
				break
			}

			if shipyardConstructionSec > 0 {
				log.Printf("Shipyard is used waiting for %d", shipyardConstructionSec)
				//time.Sleep(time.Duration(shipyardConstructionSec) * time.Second)
				//break
				botPlanets[c.GetID()] = time.Now().Add(time.Duration(shipyardConstructionSec) * time.Second)
				continue
			}

			var buildingQuantifiable ogame.Quantifiable
			var minPrice ogame.Resources

			for _, b := range ogame.PlanetBuildings {
				switch b.GetID() {
				case ogame.ShieldedMetalDenID, ogame.UndergroundCrystalDenID, ogame.SeabedDeuteriumDenID, ogame.SolarSatelliteID:
					continue
				}

				switch b.GetID() {
				case ogame.ShipyardID, ogame.NaniteFactoryID:
					if shipyardConstructionSec > 0 {
						continue
					}
				}

				if b.GetID() == ogame.ResearchLabID && researchingID != 0 {
					log.Println("Research currently in progress can not build reserch Lab")
					continue
				}

				var level int64
				if b.GetID().IsResourceBuilding() {
					level = res.ByID(b.GetID())
				}

				if b.GetID().IsFacility() {
					level = fac.ByID(b.GetID())
				}

				maxResourcesBuildings := ogame.ResourcesBuildings{
					MetalMine:            23,
					CrystalMine:          22,
					DeuteriumSynthesizer: 21,
					MetalStorage:         10,
					CrystalStorage:       10,
					DeuteriumTank:        10,
					SolarPlant:           20,
					FusionReactor:        0,
				}

				maxFacilities := ogame.Facilities{
					RoboticsFactory: 10,
					Shipyard:        12,
					ResearchLab:     12,
					MissileSilo:     6,
					NaniteFactory:   3,
				}

				maxResearches := ogame.Researches{
					EnergyTechnology:             12,
					LaserTechnology:              12,
					IonTechnology:                5,
					HyperspaceTechnology:         7,
					CombustionDrive:              6,
					ImpulseDrive:                 6,
					HyperspaceDrive:              6,
					EspionageTechnology:          8,
					ComputerTechnology:           10,
					Astrophysics:                 9,
					IntergalacticResearchNetwork: 0,
					GravitonTechnology:           0,
					WeaponsTechnology:            6,
					ShieldingTechnology:          6,
					ArmourTechnology:             6,
				}

				var maxLevel int64
				if b.GetID().IsResourceBuilding() {
					maxLevel = maxResourcesBuildings.ByID(b.GetID())
				}

				if b.GetID().IsFacility() {
					maxLevel = maxFacilities.ByID(b.GetID())
				}

				if b.GetID().IsTech() {
					maxLevel = maxResearches.ByID(b.GetID())
				}

				price := b.GetPrice(level + 1)

				if minPrice.Total() == 0 {
					minPrice = price
				}

				// Low Energy Case
				if resd.Energy.Available < 0 && shipyardConstructionSec <= 0 {
					log.Printf("Energy Available %d", resd.Energy.Available)
					if res.SolarPlant < maxResourcesBuildings.SolarPlant {
						buildingQuantifiable.ID = ogame.SolarPlantID
						minPrice = ogame.SolarPlant.GetPrice(res.SolarPlant + 1)
					} else {
						buildingQuantifiable.ID = ogame.SolarSatelliteID
						ss := ogame.SolarSatellite
						price = ss.GetPrice(10)
						minPrice = price
						if b.IsAvailable(c.GetType(), res.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) && resd.Available().CanAfford(price) {
							buildingQuantifiable.ID = ogame.SolarSatelliteID
							buildingQuantifiable.Nbr = 10
							s := ogame.SolarSatellite
							minPrice = s.GetPrice(10)
						}
					}
					break
				}

				//if level <= maxLevel && price.Lte(minPrice) && b.IsAvailable(c.GetType(), res.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) && resd.Available().CanAfford(price) {
				//if level <= maxLevel && price.Lte(minPrice) && b.IsAvailable(c.GetType(), res.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) {
				if level < maxLevel && price.Value() <= minPrice.Value() && b.IsAvailable(c.GetType(), res.Lazy(), fac.Lazy(), techs.Lazy(), resd.Energy.CurrentProduction, bot.CharacterClass()) {
					buildingQuantifiable.ID = b.GetID()
					minPrice = price
					log.Printf("Construction %s %s", buildingQuantifiable.ID, minPrice)
				}
			}
			if !resd.Available().CanAfford(minPrice) {
				var currentProductionMetal float64 = float64(resd.Metal.CurrentProduction)
				var currentProductionCrystal float64 = float64(resd.Crystal.CurrentProduction)
				var currentProductionDeuterium float64 = float64(resd.Deuterium.CurrentProduction)

				// var currentPriceMetal float64 = float64(minPrice.Metal)
				// var currentPriceCrystal float64 = float64(minPrice.Crystal)
				// var currentPriceDeuterium float64 = float64(minPrice.Deuterium)

				var metalNeeded float64
				var crystalNeeded float64
				var deuteriumNeeded float64

				var metalTime time.Duration
				var crystalTime time.Duration
				var deuteriumTime time.Duration

				resNeeded := minPrice.Sub(resd.Available())
				if resNeeded.Metal > 0 {
					metalNeeded = float64(resNeeded.Metal)
					if currentProductionMetal != 0 {
						secs := math.Ceil((metalNeeded / currentProductionMetal) * 3600)
						metalTime = time.Duration(secs) * time.Second
					}
				}
				if resNeeded.Crystal > 0 {
					crystalNeeded = float64(resNeeded.Crystal)
					if currentProductionCrystal != 0 {
						secs := math.Ceil((crystalNeeded / currentProductionCrystal) * 3600)
						crystalTime = time.Duration(secs) * time.Second
					}
				}
				if resNeeded.Deuterium > 0 {
					deuteriumNeeded = float64(resNeeded.Deuterium)
					if currentProductionDeuterium != 0 {
						secs := math.Ceil((deuteriumNeeded / currentProductionDeuterium) * 3600)
						deuteriumTime = time.Duration(secs) * time.Second
					}
				}

				maxTime := math.Max(metalTime.Seconds(), float64(crystalTime.Seconds()))
				maxTime = math.Max(maxTime, float64(deuteriumTime.Seconds()))

				log.Printf("%s Resources needed %s wait %s - Production: [%d|%d|%d]", buildingQuantifiable.ID, resNeeded, time.Duration(maxTime)*time.Second, resd.Metal.CurrentProduction, resd.Crystal.CurrentProduction, resd.Deuterium.CurrentProduction)
				//time.Sleep(time.Duration(maxTime) * time.Second)
				botPlanets[c.GetID()] = time.Now().Add(time.Duration(maxTime) * time.Second)
				continue
			}
			if buildingQuantifiable.ID != 0 {
				log.Printf("Try to build %s", buildingQuantifiable.ID)
				err := c.Build(buildingQuantifiable.ID, buildingQuantifiable.Nbr)
				if err != nil {
					log.Printf("BuildBuilding error: %s", err)
				} else {
					constructionID, constructionSec, _, _ := c.ConstructionsBeingBuilt()
					waitConstruction := time.Duration(constructionSec) * time.Second
					log.Printf("%s waiting %s", constructionID, waitConstruction)
					botPlanets[c.GetID()] = time.Now().Add(waitConstruction)
					continue
				}
			} else {
				log.Println("Nothing to build")
			}
		}
		wait := time.Duration(random(60, 90)) * time.Second
		var waitInt float64
		for _, v := range botPlanets {
			//log.Printf("%d - %s", k, time.Until(v))
			if waitInt == 0 {
				waitInt = time.Until(v).Seconds()
			}
			waitInt = math.Min(waitInt, time.Until(v).Seconds())
		}
		if waitInt <= 0 {
			wait = time.Duration(random(60, 90)) * time.Second
		} else {
			waitInt += 3
			wait = time.Duration(time.Duration(waitInt) * time.Second)
		}

		log.Printf("Construction Waiting for next loop %s", wait)
		time.Sleep(wait)
	}
}
