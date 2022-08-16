package ogame

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
	cookiejar "github.com/orirawlings/persistent-cookiejar"
	"golang.org/x/net/html"
)

func (b *OGame) GetUserAccounts() ([]Account, error) {
	return GetUserAccounts(b.client, b.ctx, b.lobby, b.GetBearerToken())
}

func (b *OGame) GetServers() ([]Server, error) {
	return GetServers(b.lobby, b.client, b.ctx)
}

func (b *OGame) GetPassword() string {
	return b.password
}

func (b *OGame) FindAccount(universe, lang string, playerID int64, accounts []Account, servers []Server) (Account, Server, error) {
	return findAccount(universe, lang, playerID, accounts, servers)
}

func (b *OGame) GetBearerToken() string {
	if b.bearerToken == "" {
		cookies := b.client.Jar.(*cookiejar.Jar).AllCookies()
		for _, c := range cookies {
			if c.Name == gfTokenCookieName {
				b.bearerToken = c.Value
				break
			}
		}
	}
	return b.bearerToken
}

func (b *OGame) SetQuiet(s bool) {
	b.quiet = s
}

// // Handlers.go
var lastActiveCelestialID CelestialID
var lastActiveCelestialIDMu sync.RWMutex

// HTMLCleaner ...
func HTMLCleaner(bot *OGame, method string, url1 string, params url.Values, payload url.Values, pageHTML []byte) []byte {
	extractor := bot.GetExtractor()
	tmpLastActiveCelestialID, err := extractor.ExtractPlanetID(pageHTML)
	if err != nil {

	} else {
		lastActiveCelestialIDMu.Lock()
		lastActiveCelestialID = tmpLastActiveCelestialID
		lastActiveCelestialIDMu.Unlock()
	}

	if (IsKnowFullPage(params) || len(params) == 0) && !IsAjaxPage(params) {
		doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
		node, _ := html.Parse(strings.NewReader(`<style>.cookiebanner1 {display: none;}\n.cookiebanner2 {display: none;}\n.cookiebanner3 {display: none;}</style>`))
		doc.Find("head").AppendNodes(node)
		htmlString, _ := doc.Html()
		return []byte(htmlString)
	}
	/*
		if (params.Get("page") == "ingame" || params.Get("page") == "messages" || params.Get("page") == "messages" || params.Get("page") == "shop" || params.Get("page") == "premium" || params.Get("page") == "chat" || params.Get("page") == "resourceSettings" || params.Get("page") == "rewards" || params.Get("page") == "standalone" || params.Get("page") == "standalone") &&
			params.Get("ajax") == "" && params.Get("asJson") == "" {
			doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
			node, _ := html.Parse(strings.NewReader(`<style>.cookiebanner1 {display: none;}\n.cookiebanner2 {display: none;}\n.cookiebanner3 {display: none;}</style>`))
			doc.Find("head").AppendNodes(node)
			htmlString, _ := doc.Html()
			return []byte(htmlString)
		}
	*/

	if IsAjaxPage(params) {
		switch params.Get("component") {
		case "technologydetails":
			type techDetails struct {
				Target  string `json:"target"`
				Content struct {
					Technologydetails string `json:"technologydetails"`
				} `json:"content"`
				Files struct {
					Js  []string `json:"js"`
					Css []string `json:"css"`
				} `json:"files"`
				Page struct {
					StateObj interface{} `json:"stateObj"`
					Title    string      `json:"title"`
					Url      string      `json:"url"`
				} `json:"page"`
				ServerTime   int64  `json:"serverTime"`
				NewAjaxToken string `json:"newAjaxToken"`
			}

			var Data techDetails

			err := json.Unmarshal(pageHTML, &Data)
			if err != nil {
				log.Debug(err)
				break
			}

			id, _ := strconv.ParseInt(params.Get("technology"), 10, 64)
			obj := Objs.ByID(ID(id))

			lastActiveCelestialIDMu.RLock()
			res, _ := bot.getResourcesDetails(lastActiveCelestialID)
			lastActiveCelestialIDMu.RUnlock()

			if obj.GetID().IsShip() || obj.GetID().IsDefense() {
				s := strings.ReplaceAll(``+Data.Content.Technologydetails+``, "\\n", "")
				s = strings.ReplaceAll(``+s+``, "\\", "")

				node, _ := html.Parse(bytes.NewReader([]byte(s)))
				doc := goquery.NewDocumentFromNode(node)

				max := res.Available().Div(obj.GetPrice(1))
				doc.Find("div.build_amount input").SetAttr("min", "0")
				doc.Find("div.build_amount input").SetAttr("max", strconv.FormatInt(max, 10))
				doc.Find("div.build_amount input").SetAttr("onfocus", `clearInput(this);"", "0"`)
				doc.Find("div.build_amount input").SetAttr("onkeyup", `checkIntInput(this, 1, `+strconv.FormatInt(max, 10)+`);event.stopPropagation();`)
				doc.Find("div.build_amount").AppendHtml("<button class=\"maximum\">[max. " + strconv.FormatInt(max, 10) + "]</button>")

				Data.Content.Technologydetails, err = doc.Html()
				if err != nil {
					log.Printf("Error occured %s", err.Error())
				}
				pageHTML, _ = json.Marshal(Data)
			}

			if obj.GetID().IsBuilding() {

			}
			break
		}
	}
	return pageHTML
}

var ninjaFleetToken string

// NinjaSendFleet (With Checks)...
func (b *OGame) ninjaSendFleet(celestialID CelestialID, ships []Quantifiable, speed Speed, where Coordinate,
	mission MissionID, resources Resources, holdingTime, unionID int64, ensure bool) (Fleet, error) {

	BeginTime := time.Now()
	originCoords := b.GetCachedCelestialByID(celestialID).GetCoordinate()
	// /game/index.php?page=ajax&component=fleetdispatch&ajax=1&asJson=1
	if ninjaFleetToken == "" {
		// GetToken
		nToken := url.Values{}
		nToken.Add("page", "ajax")
		nToken.Add("component", "fleetdispatch")
		nToken.Add("ajax", "1")
		nToken.Add("asJson", "1")
		tokenRsp := struct {
			NewAjaxToken string `json:"newAjaxToken"`
		}{}
		pageHTMLToken, err := b.getPageContent(nToken)
		if err != nil {
			return Fleet{}, err
		}
		err = json.Unmarshal(pageHTMLToken, &tokenRsp)
		if err != nil {
			return Fleet{}, err
		}
		ninjaFleetToken = tokenRsp.NewAjaxToken
	}

	payload := url.Values{}
	for _, s := range ships {
		if s.ID.IsFlyableShip() && s.Nbr > 0 {
			payload.Set("am"+strconv.FormatInt(int64(s.ID), 10), strconv.FormatInt(s.Nbr, 10))
		}
	}

	payload.Set("token", ninjaFleetToken)
	payload.Set("galaxy", strconv.FormatInt(where.Galaxy, 10))
	payload.Set("system", strconv.FormatInt(where.System, 10))
	payload.Set("position", strconv.FormatInt(where.Position, 10))
	if mission == RecycleDebrisField {
		where.Type = DebrisType // Send to debris field
	} else if mission == Colonize || mission == Expedition {
		where.Type = PlanetType
	}
	payload.Set("type", strconv.FormatInt(int64(where.Type), 10))
	payload.Set("union", "0")

	if unionID != 0 {
		found := false
		if !found {
			return Fleet{}, ErrUnionNotFound
		}
	}

	cargo := ShipsInfos{}.FromQuantifiables(ships).Cargo(b.getCachedResearch(), b.server.Settings.EspionageProbeRaids == 1, b.isCollector(), b.IsPioneers())
	newResources := Resources{}
	if resources.Total() > cargo {
		newResources.Deuterium = int64(math.Min(float64(resources.Deuterium), float64(cargo)))
		cargo -= newResources.Deuterium
		newResources.Crystal = int64(math.Min(float64(resources.Crystal), float64(cargo)))
		cargo -= newResources.Crystal
		newResources.Metal = int64(math.Min(float64(resources.Metal), float64(cargo)))
	} else {
		newResources = resources
	}

	newResources.Metal = MaxInt(newResources.Metal, 0)
	newResources.Crystal = MaxInt(newResources.Crystal, 0)
	newResources.Deuterium = MaxInt(newResources.Deuterium, 0)

	// Page 3 : select coord, mission, speed
	if b.IsV8() {
		payload.Set("token", ninjaFleetToken)
	}
	payload.Set("speed", strconv.FormatInt(int64(speed), 10))
	payload.Set("crystal", strconv.FormatInt(newResources.Crystal, 10))
	payload.Set("deuterium", strconv.FormatInt(newResources.Deuterium, 10))
	payload.Set("metal", strconv.FormatInt(newResources.Metal, 10))
	payload.Set("mission", strconv.FormatInt(int64(mission), 10))
	payload.Set("prioMetal", "1")
	payload.Set("prioCrystal", "2")
	payload.Set("prioDeuterium", "3")
	payload.Set("retreatAfterDefenderRetreat", "0")
	if mission == ParkInThatAlly || mission == Expedition {
		if mission == Expedition { // Expedition 1 to 18
			holdingTime = Clamp(holdingTime, 1, 18)
		} else if mission == ParkInThatAlly { // ParkInThatAlly 0, 1, 2, 4, 8, 16, 32
			holdingTime = Clamp(holdingTime, 0, 32)
		}
		payload.Set("holdingtime", strconv.FormatInt(holdingTime, 10))
	}

	// Page 4 : send the fleet
	res, _ := b.postPageContent(url.Values{"page": {"ingame"}, "component": {"fleetdispatch"}, "action": {"sendFleet"}, "ajax": {"1"}, "asJson": {"1"}, "cp": {strconv.FormatInt(int64(celestialID), 10)}}, payload)
	// {"success":true,"message":"Your fleet has been successfully sent.","redirectUrl":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=fleetdispatch","components":[]}
	// Insufficient resources. (4060)
	// {"success":false,"errors":[{"message":"Not enough cargo space!","error":4029}],"fleetSendingToken":"b4786751c6d5e64e56d8eb94807fbf88","components":[]}
	// {"success":false,"errors":[{"message":"Fleet launch failure: The fleet could not be launched. Please try again later.","error":4047}],"fleetSendingToken":"1507c7228b206b4a298dec1d34a5a207","components":[]} // bad token I think
	// {"success":false,"errors":[{"message":"Recyclers must be sent to recycle this debris field!","error":4013}],"fleetSendingToken":"b826ff8c3d4e04066c28d10399b32ab8","components":[]}
	// {"success":false,"errors":[{"message":"Error, no ships available","error":4059}],"fleetSendingToken":"b369e37ce34bb64e3a59fa26bd8d5602","components":[]}
	// {"success":false,"errors":[{"message":"You have to select a valid target.","error":4049}],"fleetSendingToken":"19218f446d0985dfd79e03c3ec008514","components":[]} // colonize debris field
	// {"success":false,"errors":[{"message":"Planet is already inhabited!","error":4053}],"fleetSendingToken":"3281f9ad5b4cba6c0c26a24d3577bd4c","components":[]}
	// {"success":false,"errors":[{"message":"Colony ships must be sent to colonise this planet!","error":4038}],"fleetSendingToken":"8700c275a055c59ca276a7f66c81b205","components":[]}
	// fetch("https://s801-en.ogame.gameforge.com/game/index.php?page=ingame&component=fleetdispatch&action=sendFleet&ajax=1&asJson=1", {"credentials":"include","headers":{"content-type":"application/x-www-form-urlencoded; charset=UTF-8","sec-fetch-mode":"cors","sec-fetch-site":"same-origin","x-requested-with":"XMLHttpRequest"},"body":"token=414847e59344881d5c71303023735ab8&am209=1&am202=10&galaxy=9&system=297&position=7&type=2&metal=0&crystal=0&deuterium=0&prioMetal=1&prioCrystal=2&prioDeuterium=3&mission=8&speed=1&retreatAfterDefenderRetreat=0&union=0&holdingtime=0","method":"POST","mode":"cors"}).then(res => res.json()).then(r => console.log(r));
	StartTime := time.Now()
	b.debug("Send Fleet: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")
	var resStruct struct {
		Success           bool          `json:"success"`
		Message           string        `json:"message"`
		FleetSendingToken string        `json:"fleetSendingToken"`
		Components        []interface{} `json:"components"`
		RedirectURL       string        `json:"redirectUrl"`
		Errors            []struct {
			Message string `json:"message"`
			Error   int64  `json:"error"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(res, &resStruct); err != nil {
		return Fleet{}, errors.New("failed to unmarshal response: " + err.Error())
	}
	ninjaFleetToken = resStruct.FleetSendingToken

	if len(resStruct.Errors) > 0 {
		return Fleet{}, errors.New(resStruct.Errors[0].Message + " (" + strconv.FormatInt(resStruct.Errors[0].Error, 10) + ")")
	}

	secs, _ := CalcFlightTime(
		b.GetCachedCelestialByID(celestialID).GetCoordinate(), where,
		b.serverData.Galaxies, b.serverData.Systems, b.serverData.DonutGalaxy, b.serverData.DonutSystem, b.serverData.GlobalDeuteriumSaveFactor,
		float64(speed)/10, GetFleetSpeedForMission(b.serverData, mission), ShipsInfos{}.FromQuantifiables(ships), b.getCachedResearch(), b.characterClass, holdingTime)

	if resStruct.Success == true {
		return Fleet{
			Mission:      mission,
			ReturnFlight: false,
			InDeepSpace:  false,
			ID:           0,
			Resources:    resources,
			Origin:       originCoords,
			Destination:  where,
			Ships:        ShipsInfos{}.FromQuantifiables(ships),
			StartTime:    StartTime,
			ArrivalTime:  StartTime.Add(time.Duration(secs) * time.Second),
			ArriveIn:     int64(StartTime.Add(time.Duration(secs) * time.Second).Sub(StartTime).Seconds()),
			BackIn:       int64(StartTime.Add(time.Duration(secs)*time.Second).Sub(StartTime).Seconds()) * 2,
		}, nil
	}
	now := time.Now().Unix()
	b.error(errors.New("could not find new fleet ID").Error()+", planetID:", celestialID, ", ts: ", now)
	return Fleet{}, errors.New("could not find new fleet ID")
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// NinjaSendFleet (With Checks)...
func (b *OGame) ninjaSendFleetWithChecks(celestialID CelestialID, ships []Quantifiable, speed Speed, where Coordinate,
	mission MissionID, resources Resources, holdingTime, unionID int64, ensure bool) (Fleet, error) {

	b.debug("Begin NinjaSendFleet")
	b.debug(ships)

	BeginTime := time.Now()
	originCoords := b.GetCachedCelestialByID(celestialID).GetCoordinate()
	// /game/index.php?page=ajax&component=fleetdispatch&ajax=1&asJson=1
	// GetToken
	nToken := url.Values{}
	nToken.Add("page", "ajax")
	nToken.Add("component", "fleetdispatch")
	nToken.Add("ajax", "1")
	nToken.Add("asJson", "1")
	tokenRsp := struct {
		NewAjaxToken string `json:"newAjaxToken"`
	}{}
	pageHTMLToken, err := b.getPageContent(nToken)
	if err != nil {
		return Fleet{}, err
	}
	err = json.Unmarshal(pageHTMLToken, &tokenRsp)
	if err != nil {
		return Fleet{}, err
	}

	b.debug("Get Token: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")

	_, _, availableShips, _, techs, err := b.getTechs(celestialID)
	if err != nil {
		return Fleet{}, err
	}
	b.debug("Get Techs: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")

	// /game/index.php?page=json&component=eventList&ajax=1
	type Events struct {
		Time   int64 `json:"time"`
		Events []struct {
			EventID   int64 `json:"eventId"`
			Timestamp int64 `json:"timestamp"`
			Type      int64 `json:"type"`
			FleetId   int64 `json:"fleetId"`
			OwnFleet  bool  `json:"ownFleet"`
			MissionId int64 `json:"missionId"`

			UnionId             int64  `json:"UnionId"`
			IsUnionOwner        bool   `json:"isUnionOwner"`
			IsUnion             bool   `json:"isUnion"`
			IsUnionMember       bool   `json:"isUnionMember"`
			OriginId            int64  `json:"originId"`
			OriginPlayerId      int64  `json:"originPlayerId"`
			OriginPlayerDeleted bool   `json:"originPlayerDeleted"`
			OriginPlayerName    string `json:"originPlayerName"`
			OriginName          string `json:"originName"` // Colony Name
			OriginGalaxy        int64  `json:"originGalaxy"`
			OriginSystem        int64  `json:"originSystem"`
			OriginPosition      int64  `json:"originPosition"`
			OriginType          int64  `json:"originType"`
			OriginCoordinates   string `json:"originCoordinates"`
			// Target
			TargetId            int64  `json:"targetId"`
			TargetPlayerDeleted bool   `json:"targetPlayerDeleted"`
			TargetPlayerId      int64  `json:"targetPlayerId"`
			TargetName          string `json:"targetName"`
			TargetGalaxy        int64  `json:"targetGalaxy"`
			TargetSystem        int64  `json:"targetSystem"`
			TargetPosition      int64  `json:"targetPosition"`
			TargetType          int64  `json:"targetType"`
			TargetCoordinates   string `json:"targetCoordinates"`
			IsReturnFlight      bool   `json:"isReturnFlight"`
			MissionType         string `json:"missionType"` // friendly
			Ships               struct {
				Num202 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"202"`
				Num203 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"203"`
				Num204 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"204"`
				Num205 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"205"`
				Num206 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"206"`
				Num207 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"207"`
				Num208 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"208"`
				Num209 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"209"`
				Num210 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"210"`
				Num211 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"211"`
				Num212 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"212"`
				Num213 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"213"`
				Num214 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"214"`
				Num215 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"215"`
				Num217 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"217"`
				Num218 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"218"`
				Num219 struct {
					ID     int64 `json:"id"`
					Number int64 `json:"number"`
				} `json:"219"`
			} `json:"ships"`
			ShipCountUncensored int64 `json:"shipCountUncensored"`
			ShipCount           int64 `json:"shipCount"`
			Cargo               []struct {
				Name   string `json:"name"`
				Amount int64  `json:"amount"`
			} `json:"cargo"`
		} `json:"events"`
	}

	eventResp := Events{}
	eventVals := url.Values{}
	eventVals.Add("page", "json")
	eventVals.Add("component", "eventList")
	eventVals.Add("ajax", "1")
	pageHTMLEventList, err := b.getPageContent(eventVals)
	if err != nil {
		return Fleet{}, err
	}
	err = json.Unmarshal(pageHTMLEventList, &eventResp)
	if err != nil {
		return Fleet{}, err
	}
	b.debug("Get EventList 1: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")

	maxInitialFleetID := FleetID(0)
	for _, f := range eventResp.Events {
		if FleetID(f.FleetId) > maxInitialFleetID {
			maxInitialFleetID = FleetID(f.FleetId)
		}
	}

	fuelCapacity := ShipsInfos{}.FromQuantifiables(ships).Cargo(Researches{}, true, false, false)

	_, fuel := CalcFlightTime(
		b.GetCachedCelestialByID(celestialID).GetCoordinate(), where,
		b.serverData.Galaxies, b.serverData.Systems, b.serverData.DonutGalaxy, b.serverData.DonutSystem, b.serverData.GlobalDeuteriumSaveFactor,
		float64(speed)/10, GetFleetSpeedForMission(b.serverData, mission), ShipsInfos{}.FromQuantifiables(ships), techs, b.characterClass, holdingTime)
	if fuelCapacity < fuel {
		return Fleet{}, fmt.Errorf("not enough fuel capacity, available " + strconv.FormatInt(fuelCapacity, 10) + " but needed " + strconv.FormatInt(fuel, 10))
	}

	// Ensure we're not trying to attack/spy ourselves
	destinationIsMyOwnPlanet := false
	myCelestials := b.getCachedCelestials()
	for _, c := range myCelestials {
		if c.GetCoordinate().Equal(where) && c.GetID() == celestialID {
			return Fleet{}, errors.New("origin and destination are the same")
		}
		if c.GetCoordinate().Equal(where) {
			destinationIsMyOwnPlanet = true
			break
		}
	}
	if destinationIsMyOwnPlanet {
		switch mission {
		case Spy:
			return Fleet{}, errors.New("you cannot spy yourself")
		case Attack:
			return Fleet{}, errors.New("you cannot attack yourself")
		}
	}

	atLeastOneShipSelected := false
	if !ensure {
		for i := range ships {
			avail := availableShips.ByID(ships[i].ID)
			ships[i].Nbr = int64(math.Min(float64(ships[i].Nbr), float64(avail)))
			if ships[i].Nbr > 0 {
				atLeastOneShipSelected = true
			}
		}
	} else {
		if ships != nil {
			for _, ship := range ships {
				if ship.Nbr > availableShips.ByID(ship.ID) {
					return Fleet{}, fmt.Errorf("not enough ships to send, %s", Objs.ByID(ship.ID).GetName())
				}
				atLeastOneShipSelected = true
			}
		}
	}
	if !atLeastOneShipSelected {
		return Fleet{}, ErrNoShipSelected
	}

	payload := url.Values{}
	for _, s := range ships {
		if s.ID.IsFlyableShip() && s.Nbr > 0 {
			payload.Set("am"+strconv.FormatInt(int64(s.ID), 10), strconv.FormatInt(s.Nbr, 10))
		}
	}

	payload.Set("token", tokenRsp.NewAjaxToken)
	payload.Set("galaxy", strconv.FormatInt(where.Galaxy, 10))
	payload.Set("system", strconv.FormatInt(where.System, 10))
	payload.Set("position", strconv.FormatInt(where.Position, 10))
	if mission == RecycleDebrisField {
		where.Type = DebrisType // Send to debris field
	} else if mission == Colonize || mission == Expedition {
		where.Type = PlanetType
	}
	payload.Set("type", strconv.FormatInt(int64(where.Type), 10))
	payload.Set("union", "0")

	if unionID != 0 {
		found := false
		if !found {
			return Fleet{}, ErrUnionNotFound
		}
	}

	// Check
	by1, err := b.postPageContent(url.Values{"page": {"ingame"}, "component": {"fleetdispatch"}, "action": {"checkTarget"}, "ajax": {"1"}, "asJson": {"1"}}, payload)
	if err != nil {
		b.error(err.Error())
		return Fleet{}, err
	}

	b.debug("Get Check: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")

	var checkRes CheckTargetResponse
	if err := json.Unmarshal(by1, &checkRes); err != nil {
		b.error(err.Error())
		return Fleet{}, err
	}

	if !checkRes.TargetOk {
		if len(checkRes.Errors) > 0 {
			return Fleet{}, errors.New(checkRes.Errors[0].Message + " (" + strconv.Itoa(checkRes.Errors[0].Error) + ")")
		}
		return Fleet{}, errors.New("target is not ok")
	}

	cargo := ShipsInfos{}.FromQuantifiables(ships).Cargo(techs, b.server.Settings.EspionageProbeRaids == 1, b.isCollector(), b.IsPioneers())
	newResources := Resources{}
	if resources.Total() > cargo {
		newResources.Deuterium = int64(math.Min(float64(resources.Deuterium), float64(cargo)))
		cargo -= newResources.Deuterium
		newResources.Crystal = int64(math.Min(float64(resources.Crystal), float64(cargo)))
		cargo -= newResources.Crystal
		newResources.Metal = int64(math.Min(float64(resources.Metal), float64(cargo)))
	} else {
		newResources = resources
	}

	newResources.Metal = MaxInt(newResources.Metal, 0)
	newResources.Crystal = MaxInt(newResources.Crystal, 0)
	newResources.Deuterium = MaxInt(newResources.Deuterium, 0)

	// Page 3 : select coord, mission, speed
	if b.IsV8() {
		payload.Set("token", checkRes.NewAjaxToken)
	}
	payload.Set("speed", strconv.FormatInt(int64(speed), 10))
	payload.Set("crystal", strconv.FormatInt(newResources.Crystal, 10))
	payload.Set("deuterium", strconv.FormatInt(newResources.Deuterium, 10))
	payload.Set("metal", strconv.FormatInt(newResources.Metal, 10))
	payload.Set("mission", strconv.FormatInt(int64(mission), 10))
	payload.Set("prioMetal", "1")
	payload.Set("prioCrystal", "2")
	payload.Set("prioDeuterium", "3")
	payload.Set("retreatAfterDefenderRetreat", "0")
	if mission == ParkInThatAlly || mission == Expedition {
		if mission == Expedition { // Expedition 1 to 18
			holdingTime = Clamp(holdingTime, 1, 18)
		} else if mission == ParkInThatAlly { // ParkInThatAlly 0, 1, 2, 4, 8, 16, 32
			holdingTime = Clamp(holdingTime, 0, 32)
		}
		payload.Set("holdingtime", strconv.FormatInt(holdingTime, 10))
	}

	// Page 4 : send the fleet
	res, _ := b.postPageContent(url.Values{"page": {"ingame"}, "component": {"fleetdispatch"}, "action": {"sendFleet"}, "ajax": {"1"}, "asJson": {"1"}}, payload)
	// {"success":true,"message":"Your fleet has been successfully sent.","redirectUrl":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=fleetdispatch","components":[]}
	// Insufficient resources. (4060)
	// {"success":false,"errors":[{"message":"Not enough cargo space!","error":4029}],"fleetSendingToken":"b4786751c6d5e64e56d8eb94807fbf88","components":[]}
	// {"success":false,"errors":[{"message":"Fleet launch failure: The fleet could not be launched. Please try again later.","error":4047}],"fleetSendingToken":"1507c7228b206b4a298dec1d34a5a207","components":[]} // bad token I think
	// {"success":false,"errors":[{"message":"Recyclers must be sent to recycle this debris field!","error":4013}],"fleetSendingToken":"b826ff8c3d4e04066c28d10399b32ab8","components":[]}
	// {"success":false,"errors":[{"message":"Error, no ships available","error":4059}],"fleetSendingToken":"b369e37ce34bb64e3a59fa26bd8d5602","components":[]}
	// {"success":false,"errors":[{"message":"You have to select a valid target.","error":4049}],"fleetSendingToken":"19218f446d0985dfd79e03c3ec008514","components":[]} // colonize debris field
	// {"success":false,"errors":[{"message":"Planet is already inhabited!","error":4053}],"fleetSendingToken":"3281f9ad5b4cba6c0c26a24d3577bd4c","components":[]}
	// {"success":false,"errors":[{"message":"Colony ships must be sent to colonise this planet!","error":4038}],"fleetSendingToken":"8700c275a055c59ca276a7f66c81b205","components":[]}
	// fetch("https://s801-en.ogame.gameforge.com/game/index.php?page=ingame&component=fleetdispatch&action=sendFleet&ajax=1&asJson=1", {"credentials":"include","headers":{"content-type":"application/x-www-form-urlencoded; charset=UTF-8","sec-fetch-mode":"cors","sec-fetch-site":"same-origin","x-requested-with":"XMLHttpRequest"},"body":"token=414847e59344881d5c71303023735ab8&am209=1&am202=10&galaxy=9&system=297&position=7&type=2&metal=0&crystal=0&deuterium=0&prioMetal=1&prioCrystal=2&prioDeuterium=3&mission=8&speed=1&retreatAfterDefenderRetreat=0&union=0&holdingtime=0","method":"POST","mode":"cors"}).then(res => res.json()).then(r => console.log(r));
	StartTime := time.Now()
	b.debug("Send Fleet: " + strconv.FormatInt(time.Now().Sub(BeginTime).Milliseconds(), 10) + " ms")
	var resStruct struct {
		Success           bool          `json:"success"`
		Message           string        `json:"message"`
		FleetSendingToken string        `json:"fleetSendingToken"`
		Components        []interface{} `json:"components"`
		RedirectURL       string        `json:"redirectUrl"`
		Errors            []struct {
			Message string `json:"message"`
			Error   int64  `json:"error"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(res, &resStruct); err != nil {
		return Fleet{}, errors.New("failed to unmarshal response: " + err.Error())
	}

	if len(resStruct.Errors) > 0 {
		return Fleet{}, errors.New(resStruct.Errors[0].Message + " (" + strconv.FormatInt(resStruct.Errors[0].Error, 10) + ")")
	}

	secs, _ := CalcFlightTime(
		b.GetCachedCelestialByID(celestialID).GetCoordinate(), where,
		b.serverData.Galaxies, b.serverData.Systems, b.serverData.DonutGalaxy, b.serverData.DonutSystem, b.serverData.GlobalDeuteriumSaveFactor,
		float64(speed)/10, GetFleetSpeedForMission(b.serverData, mission), ShipsInfos{}.FromQuantifiables(ships), techs, b.characterClass, holdingTime)

	// Check latest fleetID
	pageHTMLEventList2, err := b.getPageContent(eventVals)
	if err != nil {
		return Fleet{}, err
	}
	eventResp2 := Events{}
	err = json.Unmarshal(pageHTMLEventList2, &eventResp2)
	if err != nil {
		return Fleet{}, err
	}
	max := Fleet{}
	if len(eventResp2.Events) > 0 {
		max := Fleet{}

		for i, fleet := range eventResp2.Events {
			origin := Coordinate{fleet.OriginGalaxy, fleet.OriginSystem, fleet.OriginPosition, CelestialType(fleet.OriginType)}
			destination := Coordinate{fleet.TargetGalaxy, fleet.TargetSystem, fleet.TargetPosition, CelestialType(fleet.TargetType)}

			if FleetID(fleet.FleetId) > max.ID &&
				origin.Equal(originCoords) &&
				destination.Equal(where) &&
				MissionID(fleet.MissionId) == mission &&
				!fleet.IsReturnFlight {
				max.ID = FleetID(eventResp2.Events[i].FleetId)
			}
		}
		if max.ID > maxInitialFleetID {
			return max, nil
		}
	}

	if resStruct.Success == true {
		return Fleet{
			Mission:      mission,
			ReturnFlight: false,
			InDeepSpace:  false,
			ID:           max.ID,
			Resources:    resources,
			Origin:       originCoords,
			Destination:  where,
			Ships:        ShipsInfos{}.FromQuantifiables(ships),
			StartTime:    StartTime,
			ArrivalTime:  StartTime.Add(time.Duration(secs) * time.Second),
			ArriveIn:     int64(StartTime.Add(time.Duration(secs) * time.Second).Sub(StartTime).Seconds()),
			BackIn:       int64(StartTime.Add(time.Duration(secs)*time.Second).Sub(StartTime).Seconds()) * 2,
		}, nil
	}
	now := time.Now().Unix()
	b.error(errors.New("could not find new fleet ID").Error()+", planetID:", celestialID, ", ts: ", now)
	return Fleet{}, errors.New("could not find new fleet ID")

}

// SendFleetHandler ...
// curl 127.0.0.1:1234/bot/planets/123/send-fleet -d 'ships=203,1&ships=204,10&speed=10&galaxy=1&system=1&type=1&position=1&mission=3&metal=1&crystal=2&deuterium=3'
// curl 10.156.176.2:8080/bot/planets/35699346/ninja-send-fleet -d 'ships=210,1&speed=10&galaxy=12&system=178&type=1&position=9&mission=3&metal=0&crystal=0&deuterium=0'
func NinjaSendFleetHandler(c echo.Context) error {
	bot := c.Get("bot").(*OGame)
	planetID, err := strconv.ParseInt(c.Param("planetID"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid planet id"))
	}

	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid form"))
	}

	var ships []Quantifiable
	where := Coordinate{Type: PlanetType}
	mission := Transport
	var duration int64
	var unionID int64
	payload := Resources{}
	speed := HundredPercent
	for key, values := range c.Request().PostForm {
		switch key {
		case "ships":
			for _, s := range values {
				a := strings.Split(s, ",")
				shipID, err := strconv.ParseInt(a[0], 10, 64)
				if err != nil || !IsShipID(shipID) {
					return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid ship id "+a[0]))
				}
				nbr, err := strconv.ParseInt(a[1], 10, 64)
				if err != nil || nbr < 0 {
					return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid nbr "+a[1]))
				}
				ships = append(ships, Quantifiable{ID: ID(shipID), Nbr: nbr})
			}
		case "speed":
			speedInt, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil || speedInt < 0 || speedInt > 10 {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid speed"))
			}
			speed = Speed(speedInt)
		case "galaxy":
			galaxy, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid galaxy"))
			}
			where.Galaxy = galaxy
		case "system":
			system, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid system"))
			}
			where.System = system
		case "position":
			position, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid position"))
			}
			where.Position = position
		case "type":
			t, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid type"))
			}
			where.Type = CelestialType(t)
		case "mission":
			missionInt, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid mission"))
			}
			mission = MissionID(missionInt)
		case "duration":
			duration, err = strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid duration"))
			}
		case "union":
			unionID, err = strconv.ParseInt(values[0], 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid union id"))
			}
		case "metal":
			metal, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil || metal < 0 {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid metal"))
			}
			payload.Metal = metal
		case "crystal":
			crystal, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil || crystal < 0 {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid crystal"))
			}
			payload.Crystal = crystal
		case "deuterium":
			deuterium, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil || deuterium < 0 {
				return c.JSON(http.StatusBadRequest, ErrorResp(400, "invalid deuterium"))
			}
			payload.Deuterium = deuterium
		}
	}

	fleet, err := bot.WithPriority(Critical).NinjaSendFleet(CelestialID(planetID), ships, speed, where, mission, payload, duration, unionID, false)
	if err != nil &&
		(err == ErrInvalidPlanetID ||
			err == ErrNoShipSelected ||
			err == ErrUninhabitedPlanet ||
			err == ErrNoDebrisField ||
			err == ErrPlayerInVacationMode ||
			err == ErrAdminOrGM ||
			err == ErrNoAstrophysics ||
			err == ErrNoobProtection ||
			err == ErrPlayerTooStrong ||
			err == ErrNoMoonAvailable ||
			err == ErrNoRecyclerAvailable ||
			err == ErrNoEventsRunning ||
			err == ErrPlanetAlreadyReservedForRelocation) {
		return c.JSON(http.StatusBadRequest, ErrorResp(400, err.Error()))
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResp(500, err.Error()))
	}
	return c.JSON(http.StatusOK, SuccessResp(fleet))
}

func (b *OGame) HasEngineer() bool {
	return b.hasEngineer
}

func (b *OGame) HasCommander() bool {
	return b.hasCommander
}

func (b *OGame) HasAdmiral() bool {
	return b.hasAdmiral
}

func (b *OGame) HasGeologist() bool {
	return b.hasGeologist
}

func (b *OGame) HasTechnocrat() bool {
	return b.hasTechnocrat
}

type GiftCodePayload struct {
	GameAccountID int64      `json:"gameAccountId"`
	Server        GiftServer `json:"server"`
}

type GiftServer struct {
	Language string `json:"language"`
	Number   int64  `json:"number"`
} //`json:"server"`

func GetUserAccountsWithBearerToken(client *http.Client, lobby, token string) ([]Account, error) {
	var userAccounts []Account
	req, err := http.NewRequest("GET", "https://"+lobby+".ogame.gameforge.com/api/users/me/accounts", nil)
	if err != nil {
		return userAccounts, err
	}
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	resp, err := client.Do(req)
	if err != nil {
		return userAccounts, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Print(err)
		}
	}()
	by, err := readBody(resp)
	if err != nil {
		return userAccounts, err
	}
	if err := json.Unmarshal(by, &userAccounts); err != nil {
		if string(by) == `{"error":"not logged in"}` {
			return userAccounts, ErrNotLogged
		}
		return userAccounts, errors.New("failed to get user accounts : " + err.Error() + " : " + string(by))
	}
	return userAccounts, nil
}

func CreateGiftCodeWithBearerToken(lobby, bearerToken string, client *http.Client) string {
	var payload struct {
		Accounts []GiftCodePayload `json:"accounts"`
	}

	accounts, _ := GetUserAccountsWithBearerToken(client, lobby, bearerToken)
	for _, account := range accounts {
		payload.Accounts = append(payload.Accounts, GiftCodePayload{
			GameAccountID: account.ID,
			Server: GiftServer{
				Language: account.Server.Language,
				Number:   account.Server.Number,
			},
		})

	}
	jsonPayloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return ""
	}
	//log.Print(string(jsonPayloadBytes))
	req, err := http.NewRequest("PUT", "https://"+lobby+".ogame.gameforge.com/api/users/me/accountTrading", strings.NewReader(string(jsonPayloadBytes)))
	if err != nil {
		return ""
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("authorization", "Bearer "+bearerToken)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == 409 {
		gfChallengeID := resp.Header.Get(gfChallengeIDCookieName) // c434aa65-a064-498f-9ca4-98054bab0db8;https://challenge.gameforge.com
		if gfChallengeID != "" {
			parts := strings.Split(gfChallengeID, ";")
			challengeID := parts[0]
			return "error" + challengeID
		}
	}

	by, err := readBody(resp)
	if err != nil {
		return ""
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(by, &res); err != nil {

	}
	return res.Token
}

func (b *OGame) CreateGiftCode() string {
	client := b.GetClient()

	// var payload struct {
	// 	Accounts []struct {
	// 		GameAccountID int64 `json:"gameAccountId"`
	// 		Server        struct {
	// 			Language string `json:"language"`
	// 			Number   int64  `json:"number"`
	// 		} `json:"server"`
	// 	} `json:"accounts"`
	// }

	var payload struct {
		Accounts []GiftCodePayload `json:"accounts"`
	}

	accounts, _ := b.GetUserAccounts()
	for _, account := range accounts {
		payload.Accounts = append(payload.Accounts, GiftCodePayload{
			GameAccountID: account.ID,
			Server: GiftServer{
				Language: account.Server.Language,
				Number:   account.Server.Number,
			},
		})

	}
	jsonPayloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return ""
	}
	req, err := http.NewRequest("PUT", "https://"+b.lobby+".ogame.gameforge.com/api/users/me/accountTrading", strings.NewReader(string(jsonPayloadBytes)))
	if err != nil {
		return ""
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("authorization", "Bearer "+b.GetBearerToken())
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == 409 {
		gfChallengeID := resp.Header.Get(gfChallengeIDCookieName) // c434aa65-a064-498f-9ca4-98054bab0db8;https://challenge.gameforge.com
		if gfChallengeID != "" {
			parts := strings.Split(gfChallengeID, ";")
			challengeID := parts[0]
			return "error" + challengeID
		}
	}

	by, err := readBody(resp)
	if err != nil {
		return ""
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(by, &res); err != nil {

	}
	return res.Token
}

func (b *OGame) CreateGiftCodeSingleAccount(accountID int64, number int64, lang string) string {
	client := b.GetClient()

	var payload struct {
		Accounts []struct {
			GameAccountID int64 `json:"gameAccountId"`
			Server        struct {
				Language string `json:"language"`
				Number   int64  `json:"number"`
			} `json:"server"`
		} `json:"accounts"`
	}
	payload.Accounts = append(payload.Accounts, struct {
		GameAccountID int64 `json:"gameAccountId"`
		Server        struct {
			Language string `json:"language"`
			Number   int64  `json:"number"`
		} `json:"server"`
	}{
		GameAccountID: accountID,
		Server: struct {
			Language string `json:"language"`
			Number   int64  `json:"number"`
		}{
			Language: lang,
			Number:   number,
		},
	})

	jsonPayloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return ""
	}
	req, err := http.NewRequest("PUT", "https://"+b.lobby+".ogame.gameforge.com/api/users/me/accountTrading", strings.NewReader(string(jsonPayloadBytes)))
	if err != nil {
		return ""
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("authorization", "Bearer "+b.GetBearerToken())
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == 409 {
		gfChallengeID := resp.Header.Get(gfChallengeIDCookieName) // c434aa65-a064-498f-9ca4-98054bab0db8;https://challenge.gameforge.com
		if gfChallengeID != "" {
			parts := strings.Split(gfChallengeID, ";")
			challengeID := parts[0]
			return "error" + challengeID
		}
	}

	by, err := readBody(resp)
	if err != nil {
		return ""
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(by, &res); err != nil {

	}
	return res.Token
}

func (b *OGame) SelectCharacterClass(c CharacterClass) error {
	//{"POST":{"scheme":"https","host":"s133-cz.ogame.gameforge.com","filename":"/game/index.php","query":{"page":"ingame","component":"characterclassselection","characterClassId":"3","action":"selectClass","ajax":"1","asJson":"1"},"remote":{"Address":"0.0.0.0:443"}}}
	class := strconv.FormatInt(int64(c), 10)
	vals := url.Values{
		"page":             {"ingame"},
		"component":        {"characterclassselection"},
		"characterClassId": {class},
		"action":           {"selectClass"},
		"ajax":             {"1"},
		"asJson":           {"1"},
	}

	payload := url.Values{}
	by, err := b.PostPageContent(vals, payload)
	if err != nil {
		return err
	}
	var result struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(by, &result)
	if err != nil {
		return err
	}
	if result.Status == "success" {
		return nil
	}
	return nil
}

// CalcCargo ...
func (bot *OGame) CalcCargo(total int64) (sc, lc, rc, pf, ds int64) {
	research := bot.GetResearch()

	lc = int64(math.Ceil(float64(total) / float64(LargeCargo.GetCargoCapacity(research, bot.GetServerData().ProbeCargo != 0, bot.CharacterClass().IsCollector(), bot.IsPioneers()))))
	sc = int64(math.Ceil(float64(total) / float64(SmallCargo.GetCargoCapacity(research, bot.GetServerData().ProbeCargo != 0, bot.CharacterClass().IsCollector(), bot.IsPioneers()))))
	rc = int64(math.Ceil(float64(total) / float64(Recycler.GetCargoCapacity(research, bot.GetServerData().ProbeCargo != 0, bot.CharacterClass().IsCollector(), bot.IsPioneers()))))
	pf = int64(math.Ceil(float64(total) / float64(Pathfinder.GetCargoCapacity(research, bot.GetServerData().ProbeCargo != 0, bot.CharacterClass().IsCollector(), bot.IsPioneers()))))
	ds = int64(math.Ceil(float64(total) / float64(Deathstar.GetCargoCapacity(research, bot.GetServerData().ProbeCargo != 0, bot.CharacterClass().IsCollector(), bot.IsPioneers()))))
	return
}

// RedeemCode ...
func RedeemCodeWithBearerToken(lobby, bearerToken, token string, client *http.Client) error {
	var payload struct {
		Token string `json:"token"`
	}
	payload.Token = token
	jsonPayloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://"+lobby+".ogame.gameforge.com/api/token", strings.NewReader(string(jsonPayloadBytes)))
	if err != nil {
		return err
	}
	req.Header.Add("authorization", "Bearer "+bearerToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// {"tokenType":"accountTrading"}
	type respStruct struct {
		TokenType string `json:"tokenType"`
	}
	var respParsed respStruct
	by, err := readBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusBadRequest {
		return errors.New("invalid request, token invalid ?")
	}
	if err := json.Unmarshal(by, &respParsed); err != nil {
		return errors.New(err.Error() + " : " + string(by))
	}
	if respParsed.TokenType != "accountTrading" {
		return errors.New("tokenType is not accountTrading")
	}
	return nil
}

var cancelFleetToken = ""

func (b *OGame) njaCancelFleet(fleetID FleetID) error {
	params := url.Values{
		"page":      {"ajax"},
		"component": {MovementPage},
		"ajax":      {"1"},
	}
	pageHTML, err := b.getPageContent(params)

	token, err := b.extractor.ExtractCancelFleetToken(pageHTML, fleetID)
	if err != nil {
		return err
	}
	if pageHTML, err = b.getPageContent(url.Values{"page": {"ajax"}, "component": {"movement"}, "return": {fleetID.String()}, "token": {token}, "ajax": {"1"}}); err != nil {
		return err
	}
	fleets := b.extractor.ExtractFleets(pageHTML, b.location)
	token, err = b.extractor.ExtractCancelFleetToken(pageHTML, fleetID)
	if err == nil {
		cancelFleetToken = token
	}

	var ok bool
	for _, f := range fleets {
		if f.ID == fleetID && f.ReturnFlight {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("fleet cancel Error")
	}
	return nil
}

// GetMaxExpeditionPoints returns the max Expedition Points for Fleet and Resources finds.
func (b *OGame) GetMaxExpeditionPoints() (int64, int64) {
	var top1 int64
	h, err := b.Highscore(1, 1, 1)
	if err != nil {
		return 0, 0
	}
	for _, p := range h.Players {
		if p.Position == 1 {
			top1 = p.Score
			break
		}
	}
	//  less than 100.000
	if top1 < 100000 {
		return 1250, 2400
	}
	//  100.000–1.000.000
	if top1 >= 100000 && top1 < 1000000 {
		return 3000, 6000
	}
	//  1.000.000–5.000.000
	if top1 >= 1000000 && top1 < 5000000 {
		return 4500, 9000
	}
	// 5.000.000-25.000.000
	if top1 >= 5000000 && top1 < 25000000 {
		return 6000, 12000
	}
	// 25.000.000-50.000.000
	if top1 >= 25000000 && top1 < 50000000 {
		return 7500, 15000
	}
	// 50.000.000-75.000.000
	if top1 >= 50000000 && top1 < 75000000 {
		return 9000, 18000
	}
	// 75.000.000-100.000.000
	if top1 >= 75000000 && top1 < 100000000 {
		return 10500, 21000
	}
	// more than 100.000.000
	if top1 >= 100000000 {
		return 12500, 25000
	}
	return 0, 0
}

func (b *OGame) BuyItem(ref string, celestialID CelestialID) error {
	return b.WithPriority(Normal).BuyItem(ref, celestialID)
}

func (b *OGame) buyItem(ref string, celestialID CelestialID) error {
	params := url.Values{"page": {"shop"}, "ajax": {"1"}, "type": {ref}}
	if celestialID != 0 {
		params.Set("cp", strconv.FormatInt(int64(celestialID), 10))
	}
	darkmatter, err := b.fetchResources(celestialID)
	if err != nil {
		return err
	}
	pageHTML, err := b.getPageContent(params)
	if err != nil {
		return err
	}
	items, err := b.getItems(celestialID)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.Ref == ref {
			if item.Costs > darkmatter.Darkmatter.Available {
				costs := strconv.FormatInt(item.Costs, 10)
				dm := strconv.FormatInt(darkmatter.Darkmatter.Available, 10)
				return errors.New("not enough Darkmatter " + costs + " needed " + dm + " available")
			}
		}
	}

	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
	scriptTxt := doc.Find("script").Text()
	r := regexp.MustCompile(`var token="([^"]+)"`)
	m := r.FindStringSubmatch(scriptTxt)
	if len(m) != 2 {
		err := errors.New("failed to find buy token")
		return err
	}
	token := m[1]

	params = url.Values{"page": {"buyitem"}, "item": {ref}}
	payload := url.Values{
		"ajax":  {"1"},
		"token": {token},
	}
	var res struct {
		Message  interface{} `json:"message"`
		Error    bool        `json:"error"`
		NewToken string      `json:"newToken"`
	}
	by, err := b.postPageContent(params, payload)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(by, &res); err != nil {
		return err
	}
	if res.Error {
		if msg, ok := res.Message.(string); ok {
			return errors.New(msg)
		}
		return errors.New("unknown error")
	}
	return err
}

func (b *OGame) SetPreferences() error {
	payload := url.Values{}
	payload.Add("page", "ingame")
	payload.Add("component", PreferencesPage)

	p := b.BeginNamed("SetPreferences")
	defer p.Done()
	pageHTML, err := p.GetPageContent(payload) // Will update preferences cached values
	if err != nil {
		return err
	}
	var changeSettingsToken string
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
	if doc.Find("form#prefs input").Eq(2).AttrOr("name", "") == "token" {
		changeSettingsToken = doc.Find("form#prefs input").Eq(2).AttrOr("value", "")
	}
	if changeSettingsToken == "" {
		return errors.New("Token not found")
	}
	//#prefs > input:nth-child(3)
	//    var changeSettingsToken = "aaa71ec0484386d40100ad6a93950aa1";
	// r := regexp.MustCompile(`var changeSettingsToken = "([^"]+)"`)
	// m := r.FindStringSubmatch(string(pageHTML))
	// if len(m) != 2 {
	// 	err := errors.New("failed to find buy token")
	// 	return err
	// }
	// changeSettingsToken := m[1]

	fmt.Println("changeSettingsToken::", changeSettingsToken)

	// POST https://s180-de.ogame.gameforge.com/game/index.php?page=ingame&component=preferences

	payloadData := url.Values{}
	payloadData.Add("mode", "save")
	payloadData.Add("selectedTab", "0")
	payloadData.Add("token", changeSettingsToken)
	//payloadData.Add("db_character", "")
	payloadData.Add("spio_anz", "1")
	payloadData.Add("spySystemAutomaticQuantity", "1")
	payloadData.Add("spySystemTargetPlanetTypes", "0")
	payloadData.Add("spySystemTargetPlayerTypes", "0")
	payloadData.Add("spySystemIgnoreSpiedInLastXMinutes", "0")
	payloadData.Add("activateAutofocus", "on")
	payloadData.Add("eventsShow", "2")
	payloadData.Add("settings_sort", "0")
	payloadData.Add("settings_order", "0")
	payloadData.Add("showDetailOverlay", "on")
	//payloadData.Add("animatedSliders", "off")
	//payloadData.Add("animatedOverview", "off")
	payloadData.Add("msgResultsPerPage", "50")
	payloadData.Add("auctioneerNotifications", "on")
	payloadData.Add("showActivityMinutes", "1")

	_, err = p.PostPageContent(payload, payloadData)
	if err != nil {
		return err
	}

	return nil
}

// GetServerData gets the server data from xml api
func GetServerDataCached(client IHttpClient, ctx context.Context, serverNumber int64, serverLang string) (ServerData, error) {
	var serverData ServerData
	var filename string = "./api/s" + strconv.FormatInt(serverNumber, 10) + "-" + serverLang + "-serverData.xml"
	os.Mkdir("api", 0644)
	if _, err := os.Stat(filename); err == nil {
		by, err := os.ReadFile(filename)
		if err != nil {
			return serverData, err
		}
		if err := xml.Unmarshal(by, &serverData); err != nil {
			return serverData, err
		}
	} else if os.IsNotExist(err) {
		req, err := http.NewRequest(http.MethodGet, "https://s"+strconv.FormatInt(serverNumber, 10)+"-"+serverLang+".ogame.gameforge.com/api/serverData.xml", nil)
		if err != nil {
			return serverData, err
		}
		req.Header.Add("Accept-Encoding", "gzip, deflate, br")
		req.WithContext(ctx)
		resp, err := client.Do(req)
		if err != nil {
			return serverData, err
		}
		defer resp.Body.Close()
		by, err := readBody(resp)
		if err != nil {
			return serverData, err
		}
		if err := xml.Unmarshal(by, &serverData); err != nil {
			return serverData, err
		}
		os.WriteFile(filename, by, 0644)
	}

	return serverData, nil
}

func GetCachedServers(lobby string, client IHttpClient, ctx context.Context) ([]Server, error) {
	var servers []Server
	var filename string = "./api/servers.json"
	os.Mkdir("api", 0644)
	if _, err := os.Stat(filename); err == nil {
		by, err := os.ReadFile(filename)
		if err != nil {
			return servers, err
		}
		if err := json.Unmarshal(by, &servers); err != nil {
			return servers, errors.New("failed to get servers : " + err.Error() + " : " + string(by))
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, "https://"+lobby+".ogame.gameforge.com/api/servers", nil)
		if err != nil {
			return servers, err
		}
		req.Header.Add("Accept-Encoding", "gzip, deflate, br")
		req.WithContext(ctx)
		resp, err := client.Do(req)
		if err != nil {
			return servers, err
		}
		defer resp.Body.Close()
		by, err := readBody(resp)
		if err != nil {
			return servers, err
		}
		if err := json.Unmarshal(by, &servers); err != nil {
			return servers, errors.New("failed to get servers : " + err.Error() + " : " + string(by))
		}
		os.WriteFile(filename, by, 0644)
	}
	return servers, nil
}

func GetCachedUserAccounts(client IHttpClient, ctx context.Context, lobby, bearerToken string) ([]Account, error) {
	var userAccounts []Account
	var filename string = "./api/useraccounts.json"
	os.Mkdir("api", 0644)
	if _, err := os.Stat(filename); err == nil {
		by, err := os.ReadFile(filename)
		if err != nil {
			return userAccounts, err
		}
		if err := json.Unmarshal(by, &userAccounts); err != nil {
			return userAccounts, errors.New("failed to get user accounts : " + err.Error() + " : " + string(by))
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, "https://"+lobby+".ogame.gameforge.com/api/users/me/accounts", nil)
		if err != nil {
			return userAccounts, err
		}
		req.Header.Add("authorization", "Bearer "+bearerToken)
		req.Header.Add("Accept-Encoding", "gzip, deflate, br")
		req.WithContext(ctx)
		resp, err := client.Do(req)
		if err != nil {
			return userAccounts, err
		}
		defer resp.Body.Close()
		by, err := readBody(resp)
		if err != nil {
			return userAccounts, err
		}
		if err := json.Unmarshal(by, &userAccounts); err != nil {
			return userAccounts, errors.New("failed to get user accounts : " + err.Error() + " : " + string(by))
		}
		os.WriteFile(filename, by, 0644)
	}

	return userAccounts, nil
}

func (b *OGame) OfferOfTheDayAvailable() error {
	pageHTML, err := b.postPageContent(url.Values{"page": {"ajax"}, "component": {"traderimportexport"}}, url.Values{"show": {"importexport"}, "ajax": {"1"}})
	if err != nil {
		return err
	}

	price, importToken, planetResources, multiplier, err := b.extractor.ExtractOfferOfTheDay(pageHTML)
	if err != nil {
		return err
	}
	payload := calcResources(price, planetResources, multiplier)
	payload.Add("action", "trade")
	payload.Add("bid[honor]", "0")
	payload.Add("token", importToken)
	payload.Add("ajax", "1")
	pageHTML1, err := b.postPageContent(url.Values{"page": {"ajax"}, "component": {"traderimportexport"}, "ajax": {"1"}, "action": {"trade"}, "asJson": {"1"}}, payload)
	if err != nil {
		return err
	}
	// {"message":"You have bought a container.","error":false,"item":{"uuid":"40f6c78e11be01ad3389b7dccd6ab8efa9347f3c","itemText":"You have purchased 1 KRAKEN Bronze.","bargainText":"The contents of the container not appeal to you? For 500 Dark Matter you can exchange the container for another random container of the same quality. You can only carry out this exchange 2 times per daily offer.","bargainCost":500,"bargainCostText":"Costs: 500 Dark Matter","tooltip":"KRAKEN Bronze|Reduces the building time of buildings currently under construction by <b>30m<\/b>.<br \/><br \/>\nDuration: now<br \/><br \/>\nPrice: --- <br \/>\nIn Inventory: 1","image":"98629d11293c9f2703592ed0314d99f320f45845","amount":1,"rarity":"common"},"newToken":"07eefc14105db0f30cb331a8b7af0bfe"}
	var tmp struct {
		Message      string
		Error        bool
		NewAjaxToken string
	}
	if err := json.Unmarshal(pageHTML1, &tmp); err != nil {
		return err
	}
	if tmp.Error {
		return errors.New(tmp.Message)
	}

	payload2 := url.Values{"action": {"takeItem"}, "token": {tmp.NewAjaxToken}, "ajax": {"1"}}
	pageHTML2, _ := b.postPageContent(url.Values{"page": {"ajax"}, "component": {"traderimportexport"}, "ajax": {"1"}, "action": {"takeItem"}, "asJson": {"1"}}, payload2)
	var tmp2 struct {
		Message      string
		Error        bool
		NewAjaxToken string
	}
	if err := json.Unmarshal(pageHTML2, &tmp2); err != nil {
		return err
	}
	if tmp2.Error {
		return errors.New(tmp2.Message)
	}
	// {"error":false,"message":"You have accepted the offer and put the item in your inventory.","item":{"name":"Bronze Deuterium Booster","image":"f0e514af79d0808e334e9b6b695bf864b861bdfa","imageLarge":"c7c2837a0b341d37383d6a9d8f8986f500db7bf9","title":"Bronze Deuterium Booster|+10% more Deuterium Synthesizer harvest on one planet<br \/><br \/>\nDuration: 1w<br \/><br \/>\nPrice: --- <br \/>\nIn Inventory: 134","effect":"+10% more Deuterium Synthesizer harvest on one planet","ref":"d9fa5f359e80ff4f4c97545d07c66dbadab1d1be","rarity":"common","amount":134,"amount_free":134,"amount_bought":0,"category":["d8d49c315fa620d9c7f1f19963970dea59a0e3be","e71139e15ee5b6f472e2c68a97aa4bae9c80e9da"],"currency":"dm","costs":"2500","isReduced":false,"buyable":false,"canBeActivated":true,"canBeBoughtAndActivated":false,"isAnUpgrade":false,"isCharacterClassItem":false,"hasEnoughCurrency":true,"cooldown":0,"duration":604800,"durationExtension":null,"totalTime":null,"timeLeft":null,"status":null,"extendable":false,"firstStatus":"effecting","toolTip":"Bronze Deuterium Booster|+10% more Deuterium Synthesizer harvest on one planet&lt;br \/&gt;&lt;br \/&gt;\nDuration: 1w&lt;br \/&gt;&lt;br \/&gt;\nPrice: --- &lt;br \/&gt;\nIn Inventory: 134","buyTitle":"This item is currently unavailable for purchase.","activationTitle":"Activate","moonOnlyItem":false,"newOffer":false,"noOfferMessage":"There are no further offers today. Please come again tomorrow."},"newToken":"dec779714b893be9b39c0bedf5738450","components":[],"newAjaxToken":"e20cf0a6ca0e9b43a81ccb8fe7e7e2e3"}

	return nil
}

func (b *OGame) CreateAlliance(tag, name string) error {
	bot := b.BeginNamed("Create Alliance")
	defer bot.Done()

	params := url.Values{"page": {"ingame"}, "component": {"alliance"}}

	pageHTML, err := bot.GetPageContent(params)
	if err != nil {
		return err
	}
	//doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
	rgx := regexp.MustCompile(`var token = "([^"]+)";`)
	m := rgx.FindSubmatch(pageHTML)
	if len(m) != 2 {
		return errors.New("unable to find form token")
	}
	token := string(m[1])

	//  var urlCreateAlliance = "http:\/\/127.0.0.1:8080\/195\/game\/index.php?page=ingame&component=alliance&tab=createNewAlliance&action=createAlliance&asJson=1";
	//  var urlSendApplication = "http:\/\/127.0.0.1:8080\/195\/game\/index.php?page=ingame&component=alliance&tab=handleApplication&action=createApplication&asJson=1";
	//	var urlCancelApplication = "http:\/\/127.0.0.1:8080\/195\/game\/index.php?page=ingame&component=alliance&tab=handleApplication&action=cancelApplication&asJson=1";
	params = url.Values{"page": {"ingame"}, "component": {"alliance"}, "tab": {"createNewAlliance"}, "action": {"createAlliance"}, "asJson": {"1"}}
	payload := url.Values{"createTag": {tag}, "createName": {name}, "token": {token}}
	//createTag=Nuclear&createName=Nuclear&token=043a9f44af42e0da6da9b7f4c72fdb75
	pageHTML, err = bot.PostPageContent(params, payload)
	log.Printf("Payload: %s", payload.Encode())

	type resultJson struct {
		Status string `json:"status"`
	}
	var result resultJson
	err = json.Unmarshal(pageHTML, &result)
	if err != nil {
		return err
	}
	if result.Status == "success" {
		return nil
	}
	return errors.New("Status " + result.Status)
}

func (b *OGame) SetAllianceClass(class AllianceClass) error {
	bot := b.BeginNamed("Set Alliance Class")
	defer bot.Done()
	//https://s801-en.ogame.gameforge.com/game/index.php?page=ingame&component=alliance&tab=classselection&action=fetchClasses&ajax=1&token=917c290ab03a9e989ce851a207d27dd3
	// Warriors
	// Traders
	// Researchers
	//ogame.Warrior
	//ogame.Trader
	//ogame.Researcher

	//https://s801-en.ogame.gameforge.com/game/index.php?page=ingame&component=allianceclassselection&action=fetchDataAboutCurrentAllianceClass&ajax=1&asJson=1
	/*
		Result:
		{"currentAllianceClass":"-","dateOfLastAllianceClassChange":"16.08.2022 13:52:15","status":"success","message":"OK","components":[],"allianceClasses":[{"id":1,"name":"Warriors","price":500000,"icon":"warrior","isActive":true,"boni":[{"name":"Faster Ships","icon":"allymembershipspeed","longDescription":"+10% speed for ships flying between alliance members","shortDescription":"+10%"},{"name":"More Combat Research Levels","icon":"allycombatresearch","longDescription":"+1 combat research levels","shortDescription":"1 additional combat research levels"},{"name":"More Espionage Research Levels","icon":"allyespionageresearch","longDescription":"+1 espionage research levels","shortDescription":"1 additional espionage research levels"},{"name":"Espionage System","icon":"usespysystem","longDescription":"The espionage system can be used to scan whole systems.","shortDescription":"Use espionage system"}],"titleText":"Alliance Class: Warriors|+10% speed for ships flying between alliance members<br>+1 combat research levels<br>+1 espionage research levels<br>The espionage system can be used to scan whole systems.","isSelected":true,"infoLink":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=allianceclassinfo&ajax=1&allianceClassId=1","button":{"type":"freeselect","darkmatter":500000,"label":"Free Activation","link":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=classselection&allianceClassId=1&action=selectClass&ajax=1&asJson=1&type=freeselect","disabled":false,"changeTitle":""}},{"id":2,"name":"Traders","price":500000,"icon":"trader","isActive":false,"boni":[{"name":"Rapid Transporters","icon":"allyshipspeed","longDescription":"+10% speed for transporters","shortDescription":"+10%"},{"name":"Increased Production","icon":"allyresource","longDescription":"+5% mine production","shortDescription":"+5%"},{"name":"Increased Energy Production","icon":"allyenergy","longDescription":"+5% energy production","shortDescription":"+5%"},{"name":"Increased Planet Storage Capacity","icon":"allyresource","longDescription":"+10% planet storage capacity","shortDescription":"+10%"},{"name":"Increased Moon Storage Capacity","icon":"allyresource","longDescription":"+10% moon storage capacity","shortDescription":"+10%"}],"titleText":"Alliance Class: Traders|+10% speed for transporters<br>+5% mine production<br>+5% energy production<br>+10% planet storage capacity<br>+10% moon storage capacity","isSelected":false,"infoLink":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=allianceclassinfo&ajax=1&allianceClassId=2","button":{"type":"freeselect","darkmatter":500000,"label":"Free Activation","link":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=classselection&allianceClassId=2&action=selectClass&ajax=1&asJson=1&type=freeselect","disabled":false,"changeTitle":""}},{"id":3,"name":"Researchers","price":500000,"icon":"explorer","isActive":false,"boni":[{"name":"Planet Size","icon":"allycolonization","longDescription":"+5% larger planets on colonisation","shortDescription":"+5%"},{"name":"Faster Expeditions","icon":"allyexpeditionspeed","longDescription":"+10% speed to expedition destination","shortDescription":"+10%"},{"name":"System Phalanx","icon":"usephalanxsystem","longDescription":"The system phalanx can be used to scan fleet movements in whole systems.","shortDescription":"Use system phalanx"}],"titleText":"Alliance Class: Researchers|+5% larger planets on colonisation<br>+10% speed to expedition destination<br>The system phalanx can be used to scan fleet movements in whole systems.","isSelected":false,"infoLink":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=allianceclassinfo&ajax=1&allianceClassId=3","button":{"type":"freeselect","darkmatter":500000,"label":"Free Activation","link":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=classselection&allianceClassId=3&action=selectClass&ajax=1&asJson=1&type=freeselect","disabled":false,"changeTitle":""}}],"premiumLink":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=premium&showDarkMatter=1","allianceClassId":1,"token":"d5e705f2d350435c1e3d17cca9f9dc0b","newAjaxToken":"d5e705f2d350435c1e3d17cca9f9dc0b"}
	*/
	params := url.Values{}
	pageHTML, err := bot.GetPageContent(params)
	if err != nil {
		return err
	}

	type jsonResult struct {
		NewAjaxToken string `json:"newAjaxToken"`
	}
	var result jsonResult
	err = json.Unmarshal(pageHTML, &result)
	if err != nil {
		return err
	}

	params = url.Values{
		"page":            {"ingame"},
		"component":       {"alliance"},
		"tab":             {"classselection"},
		"allianceClassId": {strconv.FormatInt(int64(class), 10)},
		"action":          {"selectClass"},
		"ajax":            {"1"},
		"asJson":          {"1"},
		"type":            {"freeselect"},
	}
	payload := url.Values{
		"token": {result.NewAjaxToken},
	}
	pageHTML, err = bot.PostPageContent(params, payload)
	if err != nil {
		return err
	}

	type result2Json struct {
		Success string `json:"success"`
	}
	var result2 result2Json
	err = json.Unmarshal(pageHTML, &result2)
	if err != nil {
		return err
	}

	if result2.Success == "success" {
		return nil
	}

	// POST
	//https://s801-en.ogame.gameforge.com/game/index.php?page=ingame&component=alliance&tab=classselection&allianceClassId=3&action=selectClass&ajax=1&asJson=1&type=freeselect
	// Payload: {token: ""abcd}

	/*
		Result:{"tab":"classselection","allianceClassEnabled":true,"tabs":{"overview":{"name":"Overview","cssClass":"navi overview","enabled":true,"url":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=overview&action=fetchOverview&ajax=1","active":false,"tab":"overview"},"management":{"name":"Management","cssClass":"navi management","enabled":true,"url":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=management&action=fetchManagement&ajax=1","active":false,"tab":"management"},"broadcast":{"name":"Communication","cssClass":"navi broadcast","enabled":true,"url":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=broadcast&action=fetchBroadcast&ajax=1","active":false,"tab":"broadcast"},"applications":{"name":"Applications","cssClass":"navi applications","enabled":true,"applicationCount":0,"url":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=applications&action=fetchApplications&ajax=1","active":false,"tab":"applications"},"classselection":{"name":"Alliance Classes","cssClass":"navi classselection","enabled":true,"url":"https:\/\/s801-en.ogame.gameforge.com\/game\/index.php?page=ingame&component=alliance&tab=classselection&action=fetchClasses&ajax=1","active":true,"tab":"classselection"}},"status":"failure","errors":[{"message":"A previously unknown error has occurred. Unfortunately your last action couldn`t be executed!","error":100001}],"components":[],"newAjaxToken":"f7c9ea655d2cffa7aae7abf8eea45e02"}
	*/
	return errors.New("Error Selecting Alliance Class Success: " + result2.Success)
}
