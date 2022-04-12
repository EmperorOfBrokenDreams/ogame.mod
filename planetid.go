package ogame

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
)

// PlanetID represent a planet id
type PlanetID CelestialID

func (p PlanetID) String() string {
	return strconv.FormatInt(int64(p), 10)
}

// Celestial convert a PlanetID to a CelestialID
func (p PlanetID) Celestial() CelestialID {
	return CelestialID(p)
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (p *PlanetID) Scan(value interface{}) error {
	bytes, ok := value.(int64)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}
	*p = PlanetID(bytes)
	return nil
}

// Value return json value, implement driver.Valuer interface
func (p PlanetID) Value() (driver.Value, error) {
	return p, nil
}
