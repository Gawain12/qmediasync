package main

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/models"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type recalcResult struct {
	ID              uint   `json:"id"`
	MediaType       string `json:"media_type"`
	Status          string `json:"status"`
	VideoFilename   string `json:"video_filename"`
	CurrentNewName  string `json:"current_new_name"`
	ComputedNewName string `json:"computed_new_name"`
	TmdbId          int64  `json:"tmdb_id"`
	Name            string `json:"name"`
	Year            int    `json:"year"`
	Reason          string `json:"reason"`
}

func main() {
	pathID := flag.Uint("path-id", 1, "scrape_path_id to process")
	status := flag.String("status", "", "filter by status (optional)")
	limit := flag.Int("limit", 0, "limit records (0 = no limit)")
	write := flag.Bool("write", false, "write computed new_video_base_name back to DB")
	out := flag.String("out", "", "output json file (optional)")
	flag.Parse()

	host := envOr("DB_HOST", "localhost")
    port := envOr("DB_PORT", "15438")
	user := envOr("DB_USER", "qms")
	password := envOr("DB_PASSWORD", "qms123456")
	dbname := envOr("DB_NAME", "qms")
	sslmode := envOr("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", host, user, password, dbname, port, sslmode)
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	// set global DB for HasNamingConflict
	db.Db = gdb

	var records []models.ScrapeMediaFile
	query := gdb.Where("scrape_path_id = ?", *pathID)
	if *status != "" {
		query = query.Where("status = ?", *status)
	}
	if *limit > 0 {
		query = query.Limit(*limit)
	}
	if err := query.Order("id asc").Find(&records).Error; err != nil {
		panic(err)
	}

	results := make([]recalcResult, 0)
	updated := 0
	for _, rec := range records {
		baseName := strings.TrimSuffix(filepath.Base(rec.VideoFilename), filepath.Ext(rec.VideoFilename))
		hasConflict := rec.HasNamingConflict()
		computed := ""
		reason := ""
		if rec.TmdbId > 0 && !hasConflict && rec.CheckIsZeroDayStandard() {
			computed = baseName
			reason = "keep_original"
		} else {
			computed = rec.BuildZeroDayBaseName(hasConflict)
			reason = "rebuild_0day"
		}

		if computed != rec.NewVideoBaseName {
			results = append(results, recalcResult{
				ID:              rec.ID,
				MediaType:       string(rec.MediaType),
				Status:          string(rec.Status),
				VideoFilename:   rec.VideoFilename,
				CurrentNewName:  rec.NewVideoBaseName,
				ComputedNewName: computed,
				TmdbId:          rec.TmdbId,
				Name:            rec.Name,
				Year:            rec.Year,
				Reason:          reason,
			})
			if *write {
				if err := gdb.Model(&models.ScrapeMediaFile{}).Where("id = ?", rec.ID).Update("new_video_base_name", computed).Error; err == nil {
					updated++
				}
			}
		}
	}

	payload, _ := json.MarshalIndent(results, "", "  ")
	if *out != "" {
		if err := os.WriteFile(*out, payload, 0644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %d records to %s\n", len(results), *out)
	} else {
		fmt.Printf("%s\n", string(payload))
	}
	fmt.Printf("total scanned: %d, diff: %d, updated: %d\n", len(records), len(results), updated)
}

func envOr(key, def string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	return val
}
