package config

type Config struct {
	GribFile   string
	ColorMap   string
	OutputFile string
	MinZoom    int
	MaxZoom    int
	NumWorkers int
	Bounds     [4]float64 // [minLat, minLon, maxLat, maxLon]
	Quality    int
	Verbose    bool
}

const (
	TileSize    = 256
	WorldSizeWM = 40075016.685578488
	OffsetWM    = 20037508.342789244
	EarthRadius = 6378137.0
)
