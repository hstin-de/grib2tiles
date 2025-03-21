# GRIB2Tiles

A high-performance, command-line tool for converting GRIB2 weather data files into map tiles that can be used with web mapping libraries like Leaflet, OpenLayers, or MapLibre.

## Features

- Converts GRIB2 weather data to MBTiles format
- Fast, parallel rendering with multiple worker threads
- High-quality WebP image encoding
- Customizable color maps for different weather parameters
- Control over zoom levels and geographic bounds
- Interpolation for smooth rendering at all zoom levels
- Built in Go for performance and portability

## Installation

### Prerequisites

- Go 1.16 or later
- ecCodes library (for GRIB2 file parsing)

```bash
# Install ecCodes library
# On Ubuntu/Debian:
sudo apt-get install libeccodes-dev

# On macOS with Homebrew:
brew install eccodes
```

### Build from source

```bash
git clone https://github.com/hstin-de/grib2tiles.git
cd grib2tiles
go build
```

## Usage

Basic usage:

```bash
./grib2tiles input.grib2 output.mbtiles
```

With options:

```bash
./grib2tiles -zoom 0-10 -colors ./colors/t_2m.txt -workers 8 input.grib2 output.mbtiles
```

### Command-line options

```
Options:
  -zoom string
        Zoom levels to render (MIN-MAX) (default "0-7")
  -colors string
        Color map file for visualization (default "colors.txt")
  -area string
        Bounding box (minLon,minLat,maxLon,maxLat)
  -workers int
        Number of parallel workers (default: all available CPUs)
  -quality int
        WebP quality (1-100) (default 90)
  -verbose
        Show detailed progress
  -help
        Show help
```

## Color Maps

Color maps are defined in simple text files with the following format:

```
# Comments start with #
# Format: threshold R G B A
-inf 13 26 43 255   # Values below the first threshold
0.1 32 58 96 255    # Values >= 0.1
10.0 240 184 0 255  # Values >= 10.0
```

Each line specifies a threshold value and an RGBA color. Values greater than or equal to the threshold will use that color.

## Examples

Convert a temperature GRIB file to MBTiles with zoom levels 0-8:

```bash
./grib2tiles -zoom 0-8 -colors colors/t_2m.txt t_2m.grib2 t_2m.mbtiles
```

Convert a specific region with higher quality:

```bash
./grib2tiles -zoom 3-12 -area -10,35,30,60 -quality 95 europe.grib2 europe.mbtiles
```

## Viewing the Tiles

MBTiles files can be served using various tools:

- [MBTiles Server](https://github.com/consbio/mbtileserver)
- [TileServer GL](https://github.com/maptiler/tileserver-gl)

Or you can extract the tiles using tools like [mbutil](https://github.com/mapbox/mbutil).

## License

MIT License