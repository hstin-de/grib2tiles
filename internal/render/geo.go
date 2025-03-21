package render

import (
	"math"

	"hstin/grib2tiles/internal/config"
)

func MercatorToLatLon(mercX, mercY float64) (float64, float64) {
	lon := (mercX / config.EarthRadius) * 180.0 / math.Pi
	lat := (math.Asin(math.Tanh(mercY / config.EarthRadius))) * 180.0 / math.Pi
	return lat, lon
}

func LatLonToTile(lat, lon float64, zoom int) (int, int) {
	if lat < -85.05112878 {
		lat = -85.05112878
	} else if lat > 85.05112878 {
		lat = 85.05112878
	}

	if lon < -180 {
		lon = -180
	} else if lon > 180 {
		lon = 180
	}

	n := math.Pow(2.0, float64(zoom))
	x := int(math.Floor((lon + 180.0) / 360.0 * n))

	if x >= int(n) {
		x = int(n) - 1
	}

	latRad := lat * math.Pi / 180.0
	y := int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))

	if y < 0 {
		y = 0
	} else if y >= int(n) {
		y = int(n) - 1
	}

	return x, y
}
