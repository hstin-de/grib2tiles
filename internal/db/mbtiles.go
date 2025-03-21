package db

import (
	"database/sql"
	"fmt"
	"hstin/grib2tiles/internal/config"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dbPath string) (*sql.DB, error) {
	os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE tiles (
			zoom_level INTEGER,
			tile_column INTEGER,
			tile_row INTEGER,
			tile_data BLOB,
			PRIMARY KEY (zoom_level, tile_column, tile_row)
		);
		CREATE TABLE metadata (
			name TEXT,
			value TEXT,
			PRIMARY KEY (name)
		);
		CREATE INDEX idx_tiles on tiles (zoom_level, tile_column, tile_row);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.Exec(`
		INSERT INTO metadata VALUES
		('name', 'GRIB Tiles'),
		('type', 'overlay'),
		('version', '1.1'),
		('description', 'Tiles generated using GRIB2Tiles (https://github.com/hstin-de/grib2tiles)'),
		('format', 'webp'),
		('minzoom', '?'),
		('maxzoom', '?'),
		('bounds', '?'),
		('center', '?');
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func UpdateMetadata(db *sql.DB, config *config.Config) error {
	_, err := db.Exec("UPDATE metadata SET value = ? WHERE name = 'minzoom'", config.MinZoom)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE metadata SET value = ? WHERE name = 'maxzoom'", config.MaxZoom)
	if err != nil {
		return err
	}

	bounds := fmt.Sprintf("%f,%f,%f,%f",
		config.Bounds[1], config.Bounds[0], config.Bounds[3], config.Bounds[2])
	_, err = db.Exec("UPDATE metadata SET value = ? WHERE name = 'bounds'", bounds)
	if err != nil {
		return err
	}

	centerLat := (config.Bounds[0] + config.Bounds[2]) / 2
	centerLon := (config.Bounds[1] + config.Bounds[3]) / 2
	center := fmt.Sprintf("%f,%f,%d", centerLon, centerLat, (config.MinZoom+config.MaxZoom)/2)
	_, err = db.Exec("UPDATE metadata SET value = ? WHERE name = 'center'", center)
	if err != nil {
		return err
	}

	return nil
}
