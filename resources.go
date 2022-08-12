package ogame

import (
	"fmt"
	stdmath "math"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/google/gxui/math"
)

// ResourcesDetails ...
type ResourcesDetails struct {
	Metal struct {
		Available         int64
		StorageCapacity   int64
		CurrentProduction int64
		Production        int64
		// DenCapacity       int
	}
	Crystal struct {
		Available         int64
		StorageCapacity   int64
		CurrentProduction int64
		Production        int64
		// DenCapacity       int
	}
	Deuterium struct {
		Available         int64
		StorageCapacity   int64
		CurrentProduction int64
		Production        int64
		// DenCapacity       int
	}
	Energy struct {
		Available         int64
		CurrentProduction int64
		Consumption       int64
	}
	Darkmatter struct {
		Available int64
		Purchased int64
		Found     int64
	}
}

// Available returns the resources available
func (r ResourcesDetails) Available() Resources {
	return Resources{
		Metal:      r.Metal.Available,
		Crystal:    r.Crystal.Available,
		Deuterium:  r.Deuterium.Available,
		Energy:     r.Energy.Available,
		Darkmatter: r.Darkmatter.Available,
	}
}

func (r *ResourcesDetails) AvailableResIn(res Resources) (time.Duration, string, int64, int64) {

	var metalSec time.Duration
	neededMetal := r.Metal.Available - res.Metal
	metalProduction := r.Metal.CurrentProduction
	if neededMetal <= 0 && r.Metal.CurrentProduction != 0 {
		neededMetal = (-1) * neededMetal
		metalSec = time.Duration(stdmath.Ceil((float64(neededMetal)/float64(r.Metal.CurrentProduction))*float64(3600))) * time.Second
	}

	var crystalSec time.Duration
	neededCrystal := r.Crystal.Available - res.Crystal
	crystalProduction := r.Crystal.CurrentProduction
	if neededCrystal <= 0 && r.Crystal.CurrentProduction != 0 {
		neededCrystal = (-1) * neededCrystal
		crystalSec = time.Duration(stdmath.Ceil((float64(neededCrystal)/float64(r.Crystal.CurrentProduction))*float64(3600))) * time.Second
	}

	var deuteriumSec time.Duration
	neededDeuterium := r.Deuterium.Available - res.Deuterium
	deuteriumProduction := r.Deuterium.CurrentProduction
	if neededDeuterium <= 0 && r.Deuterium.CurrentProduction != 0 {
		neededDeuterium = (-1) * neededDeuterium
		deuteriumSec = time.Duration(stdmath.Ceil((float64(neededDeuterium)/float64(r.Deuterium.CurrentProduction))*float64(3600))) * time.Second
	}

	var maxDuration time.Duration
	maxDuration = metalSec
	maxResourcesName := "Metal"
	maxNeededResources := neededMetal
	maxProduction := metalProduction
	if crystalSec.Seconds() > maxDuration.Seconds() {
		maxDuration = crystalSec
		maxResourcesName = "Crystal"
		maxNeededResources = neededCrystal
		maxProduction = crystalProduction
	}
	if deuteriumSec.Seconds() > maxDuration.Seconds() {
		maxDuration = deuteriumSec
		maxResourcesName = "Deuterium"
		maxNeededResources = neededDeuterium
		maxProduction = deuteriumProduction
	}

	return maxDuration + (3 * time.Second), maxResourcesName, maxNeededResources, maxProduction
}

// AvailableIn returns the resources available
func (r ResourcesDetails) AvailableIn(secs float64) Resources {
	var res Resources

	if r.Metal.CurrentProduction != 0 {
		prodSec := float64(r.Metal.CurrentProduction) / 3600
		storageLeft := float64(r.Metal.StorageCapacity - r.Metal.Available)
		var storageFullInSec float64
		if prodSec != 0 {
			storageFullInSec = stdmath.Floor(storageLeft / prodSec)
		}
		if storageFullInSec <= float64(secs) {
			res.Metal = r.Metal.StorageCapacity
		} else {
			res.Metal += r.Metal.Available + int64(prodSec*float64(secs))
		}
	} else {
		// If Production is 0 the Storage is full
		res.Metal = r.Metal.Available
	}

	if r.Crystal.CurrentProduction != 0 {

		prodSec := float64(r.Crystal.CurrentProduction) / 3600
		storageLeft := float64(r.Crystal.StorageCapacity - r.Crystal.Available)
		var storageFullInSec float64
		if prodSec != 0 {
			storageFullInSec = stdmath.Floor(storageLeft / prodSec)
		}
		if storageFullInSec <= float64(secs) {
			res.Crystal = r.Crystal.StorageCapacity
		} else {
			res.Crystal += r.Crystal.Available + int64(prodSec*float64(secs))
		}
	} else {
		// If Production is 0 the Storage is full
		res.Crystal = r.Crystal.Available
	}

	if r.Deuterium.CurrentProduction != 0 {

		prodSec := float64(r.Deuterium.CurrentProduction) / 3600
		storageLeft := float64(r.Deuterium.StorageCapacity - r.Deuterium.Available)
		var storageFullInSec float64
		if prodSec != 0 {
			storageFullInSec = stdmath.Floor(storageLeft / prodSec)
		}
		if storageFullInSec <= float64(secs) {
			res.Deuterium = r.Deuterium.StorageCapacity
		} else {
			res.Deuterium += r.Deuterium.Available + int64(prodSec*float64(secs))
		}
	} else {
		// If Production is 0 the Storage is full
		res.Deuterium = r.Deuterium.Available
	}
	res.Energy = r.Energy.Available
	res.Darkmatter = r.Darkmatter.Available

	return res
}

// Resources represent ogame resources
type Resources struct {
	Metal      int64
	Crystal    int64
	Deuterium  int64
	Energy     int64
	Darkmatter int64
	Lifeform   int64
	Food       int64
}

func (r Resources) String() string {
	return fmt.Sprintf("[%s|%s|%s]",
		humanize.Comma(r.Metal), humanize.Comma(r.Crystal), humanize.Comma(r.Deuterium))
}

// Total returns the sum of resources
func (r Resources) Total() int64 {
	return r.Deuterium + r.Crystal + r.Metal
}

// Value returns normalized total value of all resources
func (r Resources) Value() int64 {
	return r.Deuterium*3 + r.Crystal*2 + r.Metal
}

// Sub subtract v from r
func (r Resources) Sub(v Resources) Resources {
	return Resources{
		Metal:     max64(r.Metal-v.Metal, 0),
		Crystal:   max64(r.Crystal-v.Crystal, 0),
		Deuterium: max64(r.Deuterium-v.Deuterium, 0),
	}
}

// Add adds two resources together
func (r Resources) Add(v Resources) Resources {
	return Resources{
		Metal:     r.Metal + v.Metal,
		Crystal:   r.Crystal + v.Crystal,
		Deuterium: r.Deuterium + v.Deuterium,
	}
}

// Mul multiply resources with scalar.
func (r Resources) Mul(scalar int64) Resources {
	return Resources{
		Metal:     r.Metal * scalar,
		Crystal:   r.Crystal * scalar,
		Deuterium: r.Deuterium * scalar,
	}
}

func min64(values ...int64) int64 {
	m := int64(math.MaxInt)
	for _, v := range values {
		if v < m {
			m = v
		}
	}
	return m
}

func max64(values ...int64) int64 {
	m := int64(math.MinInt)
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

// Div finds how many price a res can afford
func (r Resources) Div(price Resources) int64 {
	nb := int64(math.MaxInt)
	if price.Metal > 0 {
		nb = r.Metal / price.Metal
	}
	if price.Crystal > 0 {
		nb = min64(r.Crystal/price.Crystal, nb)
	}
	if price.Deuterium > 0 {
		nb = min64(r.Deuterium/price.Deuterium, nb)
	}
	return nb
}

// CanAfford alias to Gte
func (r Resources) CanAfford(cost Resources) bool {
	return r.Gte(cost)
}

// Gte greater than or equal
func (r Resources) Gte(val Resources) bool {
	return r.Metal >= val.Metal &&
		r.Crystal >= val.Crystal &&
		r.Deuterium >= val.Deuterium
}

// Lte less than or equal
func (r Resources) Lte(val Resources) bool {
	return r.Metal <= val.Metal &&
		r.Crystal <= val.Crystal &&
		r.Deuterium <= val.Deuterium
}

// FitsIn get the number of ships required to transport the resource
func (r Resources) FitsIn(ship Ship, techs Researches, probeRaids, isCollector, isPioneers bool) int64 {
	cargo := ship.GetCargoCapacity(techs, probeRaids, isCollector, isPioneers)
	if cargo == 0 {
		return 0
	}
	return int64(stdmath.Ceil(float64(r.Total()) / float64(cargo)))
}
