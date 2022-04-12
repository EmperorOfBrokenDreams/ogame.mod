package ogb

import "github.com/0xE232FE/ogame.mod"

type SentFleet struct {
	OriginID          ogame.CelestialID
	OriginCoords      ogame.Coordinate
	DestinationCoords ogame.Coordinate
	Mission           ogame.MissionID
	Speed             ogame.Speed
	HoldingTime       int64
	Ships             ogame.ShipsInfos
	Resources         ogame.Resources
	Token             string
}
