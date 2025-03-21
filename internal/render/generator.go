package render

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"database/sql"
	"hstin/grib2tiles/internal/colormap"
	"hstin/grib2tiles/internal/config"
	"hstin/grib2tiles/internal/db"
	"hstin/grib2tiles/parser"
	"os"
	"path/filepath"
)

func Generate(cfg *config.Config) error {
	startTime := time.Now()

	outputDir := filepath.Dir(cfg.OutputFile)
	if outputDir != "." && outputDir != "" {
		os.MkdirAll(outputDir, 0755)
	}

	if cfg.Verbose {
		fmt.Println("Loading GRIB file...")
	}

	fileContent, err := ioutil.ReadFile(cfg.GribFile)
	if err != nil {
		return fmt.Errorf("failed to read GRIB file: %v", err)
	}

	grib := parser.ProcessGRIB(fileContent)
	gribFile := &grib

	if cfg.Bounds[0] == -90.0 && cfg.Bounds[1] == -180.0 &&
		cfg.Bounds[2] == 90.0 && cfg.Bounds[3] == 180.0 {

		computeBoundsFromGRIB(cfg, gribFile)
	}

	if cfg.Verbose {
		fmt.Println("Loading color map...")
	}
	if err := colormap.Load(cfg.ColorMap); err != nil {
		return fmt.Errorf("failed to load color map: %v", err)
	}

	if cfg.Verbose {
		fmt.Println("Initializing tiles database...")
	}
	database, err := db.InitDB(cfg.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer database.Close()

	db.UpdateMetadata(database, cfg)

	if cfg.Verbose {
		fmt.Println("Generating tiles...")
	}

	if err := generateTiles(database, cfg, gribFile); err != nil {
		return fmt.Errorf("failed to generate tiles: %v", err)
	}

	if cfg.Verbose {
		fmt.Println("Optimizing database...")
	}
	database.Exec("VACUUM")

	elapsed := time.Since(startTime)

	fmt.Printf("Tile generation complete! Took %s\n", elapsed)

	return nil
}

func computeBoundsFromGRIB(config *config.Config, gribFile *parser.GRIBFile) {
	la1 := gribFile.Header.La1
	la2 := gribFile.Header.La2
	lo1 := gribFile.Header.Lo1
	lo2 := gribFile.Header.Lo2
	dx := gribFile.Header.DX
	dy := gribFile.Header.DY

	// Normalize Lo1 if needed
	if lo1 > 180 {
		lo1 -= 360
	}

	gribMinLat := math.Min(la1, la2)
	gribMaxLat := math.Max(la1, la2)
	gribMinLon := math.Min(lo1, lo2)
	gribMaxLon := math.Max(lo1, lo2)

	latBuffer := math.Abs(dy)
	lonBuffer := math.Abs(dx)

	config.Bounds = [4]float64{
		gribMinLat - latBuffer,
		gribMinLon - lonBuffer,
		gribMaxLat + latBuffer,
		gribMaxLon + lonBuffer,
	}

	if config.Verbose {
		fmt.Printf("  Using bounds: %.6f,%.6f to %.6f,%.6f\n",
			config.Bounds[0], config.Bounds[1], config.Bounds[2], config.Bounds[3])
	}
}

func generateTiles(db *sql.DB, cfg *config.Config, gribFile *parser.GRIBFile) error {
	stmt, err := db.Prepare("INSERT INTO tiles (zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	var wg sync.WaitGroup
	jobQueue := make(chan TileJob, 1000)
	resultQueue := make(chan TileResult, 1000)

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobQueue {
				tileData, err := RenderTile(gribFile, int(job.Z), int(job.X), int(job.Y), cfg)
				if err != nil {
					continue
				}

				resultQueue <- TileResult{
					Z:    job.Z,
					X:    job.X,
					Y:    job.Y,
					Data: tileData,
				}
			}
		}()
	}

	var totalTiles int64 = 0
	for z := cfg.MinZoom; z <= cfg.MaxZoom; z++ {
		minX, minY := LatLonToTile(cfg.Bounds[0], cfg.Bounds[1], z)
		maxX, maxY := LatLonToTile(cfg.Bounds[2], cfg.Bounds[3], z)

		if minX > maxX {
			minX, maxX = maxX, minX
		}
		if minY > maxY {
			minY, maxY = maxY, minY
		}

		tilesAtZoom := int64((maxX - minX + 1) * (maxY - minY + 1))
		totalTiles += tilesAtZoom
	}

	fmt.Printf("Generating %d tiles across zoom levels %d-%d\n", totalTiles, cfg.MinZoom, cfg.MaxZoom)

	var completedTiles int64 = 0
	startTime := time.Now()

	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

	go func() {
		defer ticker.Stop()
		lastCompleted := int64(0)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				current := atomic.LoadInt64(&completedTiles)
				elapsed := time.Since(startTime).Seconds()

				if current == lastCompleted && current > 0 {
					continue
				}

				percent := int(float64(current) / float64(totalTiles) * 100)
				tilesPerSec := float64(current) / elapsed

				var eta string
				if tilesPerSec > 0 {
					remaining := float64(totalTiles-current) / tilesPerSec
					if remaining < 60 {
						eta = fmt.Sprintf("%.0fs", remaining)
					} else if remaining < 3600 {
						eta = fmt.Sprintf("%.1fm", remaining/60)
					} else {
						eta = fmt.Sprintf("%.1fh", remaining/3600)
					}
				} else {
					eta = "calculating..."
				}

				fmt.Printf("%d/%d tiles (%d%%) | %.1f tiles/sec | Elapsed: %.0fs | ETA: %s\n",
					current, totalTiles, percent, tilesPerSec, elapsed, eta)

				lastCompleted = current
			}
		}
	}()

	var dbWg sync.WaitGroup
	dbWg.Add(1)
	go func() {
		defer dbWg.Done()

		for result := range resultQueue {
			tmsY := (1 << result.Z) - 1 - result.Y
			_, err := stmt.Exec(result.Z, result.X, tmsY, result.Data)
			if err != nil {
				log.Printf("Error inserting tile: %v", err)
			}

			atomic.AddInt64(&completedTiles, 1)
		}
	}()

	for z := cfg.MinZoom; z <= cfg.MaxZoom; z++ {
		minX, minY := LatLonToTile(cfg.Bounds[0], cfg.Bounds[1], z)
		maxX, maxY := LatLonToTile(cfg.Bounds[2], cfg.Bounds[3], z)

		if minX > maxX {
			minX, maxX = maxX, minX
		}
		if minY > maxY {
			minY, maxY = maxY, minY
		}

		if cfg.Verbose {
			fmt.Printf("Zoom level %d: generating %d x %d = %d tiles\n",
				z, (maxX - minX + 1), (maxY - minY + 1), (maxX-minX+1)*(maxY-minY+1))
		}

		for x := minX; x <= maxX; x++ {
			for y := minY; y <= maxY; y++ {
				jobQueue <- TileJob{
					Z: uint8(z),
					X: uint32(x),
					Y: uint32(y),
				}
			}
		}
	}

	close(jobQueue)
	wg.Wait()

	close(resultQueue)
	dbWg.Wait()

	done <- true

	final := atomic.LoadInt64(&completedTiles)
	totalTime := time.Since(startTime).Seconds()
	avgRate := float64(final) / totalTime

	fmt.Printf("\nTile generation completed! Generated %d tiles in %.1f seconds (%.1f tiles/sec)\n",
		final, totalTime, avgRate)

	return nil
}
