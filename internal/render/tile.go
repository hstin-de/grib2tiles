package render

import (
	"bytes"
	"hstin/grib2tiles/internal/colormap"
	"hstin/grib2tiles/internal/config"
	"hstin/grib2tiles/parser"
	"image"

	"github.com/chai2010/webp"
)

type TileJob struct {
	Z uint8
	X uint32
	Y uint32
}

type TileResult struct {
	Z    uint8
	X    uint32
	Y    uint32
	Data []byte
}

func RenderTile(gribFile *parser.GRIBFile, z, x, y int, cfg *config.Config) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))

	s := config.WorldSizeWM / (float64(config.TileSize) * float64(uint32(1)<<z))
	baseX := x * config.TileSize
	baseY := y * config.TileSize

	for py := 0; py < config.TileSize; py++ {
		rowOffset := py * img.Stride
		worldY := float64(baseY + py)
		for px := 0; px < config.TileSize; px++ {
			worldX := float64(baseX + px)
			mercX := worldX*s - config.OffsetWM
			mercY := config.OffsetWM - worldY*s

			lat, lon := MercatorToLatLon(mercX, mercY)

			if lat < cfg.Bounds[0] || lat > cfg.Bounds[2] ||
				lon < cfg.Bounds[1] || lon > cfg.Bounds[3] {
				continue
			}

			val := gribFile.GetInterpolatedData(lat, lon)

			if val != gribFile.Header.MissingValue {
				pixelColor := colormap.GetColor(val)

				idx := rowOffset + px*4
				img.Pix[idx] = pixelColor.R
				img.Pix[idx+1] = pixelColor.G
				img.Pix[idx+2] = pixelColor.B
				img.Pix[idx+3] = pixelColor.A
			}
		}
	}

	var buf bytes.Buffer
	options := &webp.Options{Lossless: false, Quality: float32(cfg.Quality)}
	err := webp.Encode(&buf, img, options)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
