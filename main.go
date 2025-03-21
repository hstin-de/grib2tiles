package main

import (
	"flag"
	"fmt"
	"hstin/grib2tiles/internal/config"

	"hstin/grib2tiles/internal/render"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	// Setup custom usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Light Pollution Tiles Generator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options] input.grib output.mbtiles\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  Basic:    %s input.grib output.mbtiles\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  With zoom:  %s -zoom 3-12 input.grib output.mbtiles\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Preview:  %s -preview input.grib output.mbtiles\n", os.Args[0])
	}

	zoom := flag.String("zoom", "0-7", "Zoom levels to render (MIN-MAX)")
	colors := flag.String("colors", "colors.txt", "Color map file for visualization")
	area := flag.String("area", "", "Bounding box (minLon,minLat,maxLon,maxLat)")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of parallel workers (default: all available CPUs)")
	quality := flag.Int("quality", 90, "WebP quality (1-100)")
	verbose := flag.Bool("verbose", false, "Show detailed progress")
	help := flag.Bool("help", false, "Show help")

	// Parse flags
	flag.Parse()

	// Show help if requested
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Get positional arguments
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: Missing input or output file\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options] input.grib output.mbtiles\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	// Verify input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Input file not found: %s\n", inputFile)
		os.Exit(1)
	}

	// Parse zoom levels (MIN-MAX)
	minZoom, maxZoom := 0, 10
	if *zoom != "" {
		parts := strings.Split(*zoom, "-")
		if len(parts) == 2 {
			min, err1 := strconv.Atoi(parts[0])
			max, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				minZoom, maxZoom = min, max
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid zoom format. Use MIN-MAX (e.g., 0-10)\n")
				os.Exit(1)
			}
		} else if len(parts) == 1 {
			// Single zoom level
			z, err := strconv.Atoi(parts[0])
			if err == nil {
				minZoom, maxZoom = z, z
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid zoom value\n")
				os.Exit(1)
			}
		}
	}

	// Parse bounding box
	bounds := [4]float64{-90.0, -180.0, 90.0, 180.0} // Default world
	if *area != "" {
		parts := strings.Split(*area, ",")
		if len(parts) != 4 {
			fmt.Fprintf(os.Stderr, "Error: Area format should be minLon,minLat,maxLon,maxLat\n")
			os.Exit(1)
		}

		minLon, err1 := strconv.ParseFloat(parts[0], 64)
		minLat, err2 := strconv.ParseFloat(parts[1], 64)
		maxLon, err3 := strconv.ParseFloat(parts[2], 64)
		maxLat, err4 := strconv.ParseFloat(parts[3], 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid area values\n")
			os.Exit(1)
		}

		// Store in our internal format [minLat, minLon, maxLat, maxLon]
		bounds = [4]float64{minLat, minLon, maxLat, maxLon}
	}

	// Create config
	cfg := &config.Config{
		GribFile:   inputFile,
		ColorMap:   *colors,
		OutputFile: outputFile,
		MinZoom:    minZoom,
		MaxZoom:    maxZoom,
		NumWorkers: *workers,
		Bounds:     bounds,
		Quality:    *quality,
		Verbose:    *verbose,
	}

	// Show configuration summary if verbose
	if *verbose {
		fmt.Println("Configuration:")
		fmt.Printf("  Input: %s\n", inputFile)
		fmt.Printf("  Output: %s\n", outputFile)
		fmt.Printf("  Zoom: %d to %d\n", minZoom, maxZoom)
		fmt.Printf("  Workers: %d\n", *workers)

		if *area != "" {
			fmt.Printf("  Area: %s\n", *area)
		} else {
			fmt.Println("  Area: Using GRIB file extents")
		}
	}

	// Generate tiles
	if err := render.Generate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Start preview server if requested
	// if cfg.WebServer {
	// 	fmt.Printf("Starting preview server at http://localhost:%d\n", cfg.ServerPort)
	// 	fmt.Printf("Press Ctrl+C to stop the server\n")
	// 	preview.StartServer(cfg)
	// }
}
