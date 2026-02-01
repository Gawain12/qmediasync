package main

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/scrape"
	dbConfig "Q115-STRM/internal/db/config"
	"Q115-STRM/internal/db/database"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// 1. 初始化环境
	wd, _ := os.Getwd()
	helpers.RootDir = wd
	logDir := filepath.Join(wd, "config", "logs")
	os.MkdirAll(logDir, 0755)
	helpers.AppLogger = helpers.NewLogger(filepath.Join(logDir, "app.log"), true, true)
	helpers.TMDBLog = helpers.NewLogger(filepath.Join(logDir, "tmdb.log"), false, true)
	helpers.V115Log = helpers.NewLogger(filepath.Join(logDir, "v115.log"), false, true)
	helpers.OpenListLog = helpers.NewLogger(filepath.Join(logDir, "openlist.log"), false, true)
	
	config := dbConfig.Load(wd)
	
	dbConf := &database.Config{
		Mode:     "external",
		Host:     config.DB.Host,
		Port:     config.DB.Port,
		User:     config.DB.User,
		Password: config.DB.Password,
		DBName:   config.DB.Name,
		SSLMode:  config.DB.SSLMode,
		External: true,
	}
	
	mgr := database.NewManager(dbConf)
	if err := mgr.Start(context.Background()); err != nil {
		fmt.Printf("数据库连接失败: %v\n", err)
		return
	}
	db.InitDb(mgr.GetDB())
	models.LoadScrapeSettings()

	// 2. 查询待处理条目
	var mediaFiles []*models.ScrapeMediaFile
	db.Db.Where("tmdb_id > 0 AND status IN (?)", []models.ScrapeMediaStatus{
		models.ScrapeMediaStatusScanned,
		models.ScrapeMediaStatusScrapeFailed,
		models.ScrapeMediaStatusRenameFailed,
	}).Find(&mediaFiles)

	if len(mediaFiles) == 0 {
		fmt.Println("没有找到符合条件的待处理记录 (tmdb_id > 0 且未完成)")
		return
	}

	fmt.Printf("找到 %d 条待处理记录，开始批量刮削...\n", len(mediaFiles))

	// 3. 按目录分组处理
	pathGroups := make(map[uint][]*models.ScrapeMediaFile)
	for _, mf := range mediaFiles {
		pathGroups[mf.ScrapePathId] = append(pathGroups[mf.ScrapePathId], mf)
	}

	for pathID, files := range pathGroups {
		scrapePath := models.GetScrapePathByID(pathID)
		if scrapePath == nil {
			continue
		}
		fmt.Printf("\n--- 处理目录: %s (%d 条) ---\n", scrapePath.SourcePath, len(files))
		for i, file := range files {
			fmt.Printf("[%d/%d] 正在处理: %s\n", i+1, len(files), file.VideoFilename)
			file.QueryRelation() // 加载 Media 等关联对象
			file.DecodeJson()    // 关键：对 JSON 字段（如视频编码信息）进行解码，防止空指针
			file.Status = models.ScrapeMediaStatusScanned // 强制设置为待刮削，触发全流程
			scraper := scrape.NewScrape(scrapePath)
			err := scraper.ProcessSingle(file)
			if err != nil {
				fmt.Printf("  FAILED: %v\n", err)
			} else {
				fmt.Printf("  SUCCESS\n")
			}
			time.Sleep(5000 * time.Millisecond)
		}
	}

	fmt.Println("\n批量刮削任务完成！")
}
