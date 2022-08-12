package ogame

import (
	"bytes"
	"math"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

func (b *OGame) ChangeUsername(newUsername string) {
	// POST
	// https://s182-de.ogame.gameforge.com/game/index.php?page=ingame&component=preferences

	//mode=save&selectedTab=0&token=0ca38d65c41dd963494865a3a2f6c619&db_character=Legend&db_character_password=6M4duzUcja2CU0Q4AZne&urlaubs_modus=on

	/*
		mode "save"
		selectedTab	"0"
		token	"0ca38d65c41dd963494865a3a2f6c619"
		db_character	"Legend"
		db_character_password	"6M4duzUcja2CU0Q4AZne"
		urlaubs_modus	"on"
	*/
	prio := b.BeginNamed("Change Username")
	defer prio.Done()
	pageHTML, _ := prio.GetPageContent(url.Values{"page": {"ingame"}, "component": {PreferencesPage}})
	doc, _ := goquery.NewDocumentFromReader(bytes.NewReader(pageHTML))
	token := doc.Find("form#prefs input").Eq(2).AttrOr("value", "")

	vals := url.Values{}
	vals.Add("mode", "save")
	vals.Add("selectedTab", "0")
	vals.Add("token", token)
	vals.Add("db_character", newUsername)
	vals.Add("db_character_password", b.password)
	//vals.Add("urlaubs_modus", "off")
	//resultJSON, _ := prio.PostPageContent(url.Values{"page": {"ingame"}, "component": {PreferencesPage}}, vals)
	//log.Println(string(resultJSON))
	prio.PostPageContent(url.Values{"page": {"ingame"}, "component": {PreferencesPage}}, vals)
}

func (s ShipsInfos) Div(d int64) ShipsInfos {
	var res = ShipsInfos{}
	if d > 0 {
		res.Battlecruiser = int64(math.Floor(float64(s.Battlecruiser) / float64(d)))
		res.Battleship = int64(math.Floor(float64(s.Battleship) / float64(d)))
		res.Bomber = int64(math.Floor(float64(s.Bomber) / float64(d)))
		res.ColonyShip = int64(math.Floor(float64(s.ColonyShip) / float64(d)))
		res.Cruiser = int64(math.Floor(float64(s.Cruiser) / float64(d)))
		res.Deathstar = int64(math.Floor(float64(s.Deathstar) / float64(d)))
		res.Destroyer = int64(math.Floor(float64(s.Destroyer) / float64(d)))
		res.EspionageProbe = int64(math.Floor(float64(s.EspionageProbe) / float64(d)))
		res.HeavyFighter = int64(math.Floor(float64(s.HeavyFighter) / float64(d)))
		res.LargeCargo = int64(math.Floor(float64(s.LargeCargo) / float64(d)))
		res.SmallCargo = int64(math.Floor(float64(s.SmallCargo) / float64(d)))
		res.LightFighter = int64(math.Floor(float64(s.LightFighter) / float64(d)))
		res.Pathfinder = int64(math.Floor(float64(s.Pathfinder) / float64(d)))
		res.Reaper = int64(math.Floor(float64(s.Reaper) / float64(d)))
		res.Recycler = int64(math.Floor(float64(s.Recycler) / float64(d)))

		res.SolarSatellite = int64(math.Floor(float64(s.SolarSatellite) / float64(d)))
		res.Crawler = int64(math.Floor(float64(s.Crawler) / float64(d)))
	}
	return res
}
