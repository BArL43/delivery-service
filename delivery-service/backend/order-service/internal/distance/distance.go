package distance

import (
	"math"
)

const earthRadiusKm = 6371.0

func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180.0
}

func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := degreesToRadians(lat1)
	lon1Rad := degreesToRadians(lon1)
	lat2Rad := degreesToRadians(lat2)
	lon2Rad := degreesToRadians(lon2)

	diffLat := lat2Rad - lat1Rad
	diffLon := lon2Rad - lon1Rad

	a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Pow(math.Sin(diffLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
