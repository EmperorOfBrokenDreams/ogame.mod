package ogame

import "time"

// Fleet represent a player fleet information
type Fleet struct {
	Mission        MissionID
	ReturnFlight   bool
	InDeepSpace    bool
	ID             FleetID
	Resources      Resources  `gorm:"embedded"`
	Origin         Coordinate `gorm:"embedded"`
	Destination    Coordinate `gorm:"embedded"`
	Ships          ShipsInfos `gorm:"embedded"`
	StartTime      time.Time
	ArrivalTime    time.Time
	BackTime       time.Time
	ArriveIn       int64
	BackIn         int64
	UnionID        int64
	TargetPlanetID int64
}
