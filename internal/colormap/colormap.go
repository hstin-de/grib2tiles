package colormap

import (
	"bufio"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

type ColorMapEntry struct {
	ValueThreshold float64
	Color          color.RGBA
}

var (
	colorMap     []ColorMapEntry
	defaultColor = color.RGBA{13, 26, 43, 255}
)

func Load(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening color map file: %v", err)
	}
	defer file.Close()

	colorMap = []ColorMapEntry{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			log.Printf("Warning: Invalid line format in color map file: %s", line)
			continue
		}

		var threshold float64
		if fields[0] == "-inf" {
			threshold = math.Inf(-1)
		} else {
			val, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				log.Printf("Warning: Invalid threshold value: %s", fields[0])
				continue
			}
			threshold = val
		}

		r, _ := strconv.Atoi(fields[1])
		g, _ := strconv.Atoi(fields[2])
		b, _ := strconv.Atoi(fields[3])
		a, _ := strconv.Atoi(fields[4])

		colorMap = append(colorMap, ColorMapEntry{
			ValueThreshold: threshold,
			Color:          color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)},
		})
	}

	if len(colorMap) == 0 {
		return fmt.Errorf("no valid entries found in color map file")
	}
	return nil
}

func GetColor(value float64) color.RGBA {
	if value <= 0 {
		return defaultColor
	}

	for i := len(colorMap) - 1; i >= 0; i-- {
		if value >= colorMap[i].ValueThreshold {
			return colorMap[i].Color
		}
	}

	if len(colorMap) > 0 {
		return colorMap[0].Color
	}

	return defaultColor
}