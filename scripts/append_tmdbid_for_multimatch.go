package main

import (
	"Q115-STRM/internal/db"
	dbConfig "Q115-STRM/internal/db/config"
	"Q115-STRM/internal/db/database"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/scrape/rename"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type renameRunner interface {
	Rename(fileId, newName string) error
}

var yearPattern = regexp.MustCompile(`(?i)(19|20)\d{2}`)
var akaPattern = regexp.MustCompile(`(?i)\\baka\\b.*$`)
var bracketPattern = regexp.MustCompile(`[\[\(【].*?[\]\)】]`)
var techTokenPattern = regexp.MustCompile(`(?i)\\b(bluray|bdrip|dvdrip|tvrip|web-?dl|webrip|hdtv|x264|x265|h\\.?264|h\\.?265|aac|ddp|dts|flac|10bit|hdr|dovi|repack|remux|web|rip|uhd|sd|hd|french|german|japanese|korean|chinese|dual|sub|subs)\\b`)

type ambiguousEntry struct {
	ID             uint   `json:"id"`
	MediaId        uint   `json:"media_id"`
	ScrapePathId   uint   `json:"scrape_path_id"`
	FileId         string `json:"file_id"`
	CurrentName    string `json:"current_name"`
	NewName        string `json:"new_name"`
	TmdbId         int64  `json:"tmdb_id"`
	Name           string `json:"name"`
	Year           int    `json:"year"`
	AlreadyTagged  bool   `json:"already_tagged"`
	SkipReason     string `json:"skip_reason,omitempty"`
	QueryName      string `json:"query_name"`
	QueryYear      int    `json:"query_year"`
	ResultCount    int    `json:"result_count"`
	RawQueryName   string `json:"raw_query_name"`
	RawResultCount int    `json:"raw_result_count"`
}

func buildRenameRunner(sp *models.ScrapePath) (renameRunner, error) {
	switch sp.SourceType {
	case models.SourceType115:
		account, err := sp.GetAccount()
		if err != nil {
			return nil, err
		}
		client := account.Get115Client(true)
		return rename.NewRename115(context.Background(), sp, client), nil
	case models.SourceTypeOpenList:
		account, err := sp.GetAccount()
		if err != nil {
			return nil, err
		}
		client := openlist.NewClient(account.ID, account.BaseUrl, account.Username, account.Password, account.Token)
		return rename.NewRenameOpenList(context.Background(), sp, client), nil
	case models.SourceTypeLocal:
		return rename.NewRenameLocal(context.Background(), sp), nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sp.SourceType)
	}
}

func deriveQueryNameYear(filename string) (string, int) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	year := 0
	titlePart := base
	if loc := yearPattern.FindStringIndex(base); loc != nil && len(loc) == 2 {
		yearStr := base[loc[0]:loc[1]]
		if y, err := strconv.Atoi(yearStr); err == nil {
			year = y
		}
		titlePart = strings.TrimSpace(base[:loc[0]])
	}
	titlePart = strings.Trim(titlePart, " ._-[](){}")
	if titlePart == "" {
		titlePart = base
	}
	// Normalize separators to spaces
	titlePart = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(titlePart)
	titlePart = strings.Join(strings.Fields(titlePart), " ")
	return titlePart, year
}

func cleanQueryName(name string, deleteKeywords []string) string {
	n := name
	n = bracketPattern.ReplaceAllString(n, " ")
	n = akaPattern.ReplaceAllString(n, " ")
	for _, kw := range deleteKeywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		n = regexp.MustCompile("(?i)"+regexp.QuoteMeta(kw)).ReplaceAllString(n, " ")
	}
	n = techTokenPattern.ReplaceAllString(n, " ")
	n = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(n)
	n = strings.Join(strings.Fields(n), " ")
	return n
}

func checkByNameAndYear(client *models.ScrapeSettings, name string, year int) (int, bool, error) {
	tmdbClient := client.GetTmdbClient()
	// switchYear=false => use year param; switchYear=true => use primary_release_year
	search, err := tmdbClient.SearchMovie(name, year, client.GetTmdbLanguage(), true, true)
	if err != nil || search == nil {
		return 0, false, err
	}
	if search.TotalResults == 0 {
		search, err = tmdbClient.SearchMovie(name, year, client.GetTmdbLanguage(), true, false)
		if err != nil || search == nil {
			return 0, false, err
		}
	}
	if search.TotalResults > 1 && len(search.Results) > 0 {
		first := search.Results[0]
		if strings.EqualFold(first.Title, name) || strings.EqualFold(first.OriginalTitle, name) {
			return 1, false, nil
		}
		return search.TotalResults, true, nil
	}
	return search.TotalResults, false, nil
}

func main() {
	apply := flag.Bool("apply", true, "apply rename and update DB")
	dryRun := flag.Bool("dry-run", false, "do not rename, only log")
	limit := flag.Int("limit", 0, "limit number of processed records (0 = no limit)")
	progressEvery := flag.Int("progress", 0, "print progress every N items (0 = disable)")
	cacheFile := flag.String("cache", "config/logs/ambiguous_tmdb.jsonl", "cache file path for ambiguous results")
	writeCache := flag.Bool("write-cache", false, "write ambiguous results to cache file")
	applyCache := flag.Bool("apply-cache", false, "apply rename using cached results only")
	idsFilter := flag.String("ids", "", "only process specific scrape_media_file ids, comma-separated")
	printIdList := flag.Bool("print-id-list", false, "print id lists for ambiguous/noresult at end")
	sleepMs := flag.Int("sleep-ms", 1200, "sleep milliseconds after each rename to avoid 115 rate limits")
	resumeFile := flag.String("resume-file", "config/logs/append_tmdbid_checkpoint.json", "checkpoint file for resume")
	resume := flag.Bool("resume", true, "skip already processed ids recorded in resume file")
	stopOnTokenExpired := flag.Bool("stop-on-token-expired", true, "stop immediately when 115 token expired")
	flag.Parse()

	wd, _ := os.Getwd()
	helpers.RootDir = wd
	logDir := filepath.Join(wd, "config", "logs")
	_ = os.MkdirAll(logDir, 0755)
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

	processed := map[uint]bool{}
	if *resume {
		if data, err := os.ReadFile(*resumeFile); err == nil {
			var ids []uint
			if err := json.Unmarshal(data, &ids); err == nil {
				for _, id := range ids {
					processed[id] = true
				}
			}
		}
	}

	saveCheckpoint := func() {
		if !*resume {
			return
		}
		ids := make([]uint, 0, len(processed))
		for id := range processed {
			ids = append(ids, id)
		}
		data, err := json.Marshal(ids)
		if err != nil {
			return
		}
		_ = os.MkdirAll(filepath.Dir(*resumeFile), 0755)
		_ = os.WriteFile(*resumeFile, data, 0644)
	}
	if *applyCache {
		entries, err := readJsonlCache(*cacheFile)
		if err != nil {
			fmt.Printf("读取缓存失败: %v\n", err)
			return
		}
		if len(entries) == 0 {
			fmt.Println("缓存为空，无可处理记录")
			return
		}
		updated := 0
		runners := make(map[uint]renameRunner)
		for i, entry := range entries {
			if *limit > 0 && i >= *limit {
				break
			}
			mf := models.GetScrapeMediaFileById(entry.ID)
			if mf == nil {
				fmt.Printf("跳过 ID=%d: 记录不存在\n", entry.ID)
				continue
			}
			mf.QueryRelation()
			if mf.Media == nil && mf.MediaId > 0 {
				media := &models.Media{}
				if err := db.Db.Where("id = ?", mf.MediaId).First(media).Error; err == nil {
					mf.Media = media
				}
			}
			scrapePath := models.GetScrapePathByID(entry.ScrapePathId)
			if scrapePath == nil {
				fmt.Printf("跳过 ID=%d: 未找到刮削目录\n", entry.ID)
				continue
			}
			runner, ok := runners[entry.ScrapePathId]
			if !ok {
				r, err := buildRenameRunner(scrapePath)
				if err != nil {
					fmt.Printf("获取重命名实现失败 ID=%d: %v\n", entry.ID, err)
					continue
				}
				runner = r
				runners[entry.ScrapePathId] = runner
			}
			if entry.AlreadyTagged || entry.NewName == "" {
				fmt.Printf("[cache-skip] %d/%d ID=%d reason=%s\n", i+1, len(entries), entry.ID, entry.SkipReason)
				continue
			}
			fileId := entry.FileId
			if fileId == "" {
				if mf.Media != nil && mf.Media.VideoFileId != "" {
					fileId = mf.Media.VideoFileId
				} else if mf.VideoFileId != "" {
					fileId = mf.VideoFileId
				}
			}
			if fileId == "" {
				fmt.Printf("重命名失败 ID=%d: 文件ID为空\n", entry.ID)
				continue
			}
			if !*apply || *dryRun {
				fmt.Printf("[cache] %d/%d ID=%d %s => %s\n", i+1, len(entries), entry.ID, entry.CurrentName, entry.NewName)
				continue
			}
			if err := runner.Rename(fileId, entry.NewName); err != nil {
				fmt.Printf("重命名失败 ID=%d: %v\n", entry.ID, err)
				continue
			}
			base := strings.TrimSuffix(entry.NewName, filepath.Ext(entry.NewName))
			mf.NewVideoBaseName = base
			if mf.Media != nil {
				mf.Media.VideoFileName = entry.NewName
			}
			mf.Save()
			if mf.Media != nil {
				mf.Media.Save()
			}
			updated++
		}
		if *apply {
			fmt.Printf("完成：更新 %d 条记录\n", updated)
		} else {
			fmt.Printf("完成：预览 %d 条缓存记录（dry-run）\n", len(entries))
		}
		return
	}

	var mediaFiles []*models.ScrapeMediaFile
	if strings.TrimSpace(*idsFilter) != "" {
		idParts := strings.Split(*idsFilter, ",")
		idList := make([]uint, 0, len(idParts))
		for _, part := range idParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if v, err := strconv.ParseUint(part, 10, 64); err == nil {
				idList = append(idList, uint(v))
			}
		}
		if len(idList) == 0 {
			fmt.Println("ids 参数为空或无效")
			return
		}
		db.Db.Where("id IN ?", idList).Find(&mediaFiles)
	} else {
		db.Db.Where("media_type = ? AND status = ? AND tmdb_id > 0", models.MediaTypeMovie, models.ScrapeMediaStatusRenamed).Find(&mediaFiles)
	}
	if len(mediaFiles) == 0 {
		fmt.Println("没有找到符合条件的记录 (movie + renamed + tmdb_id>0)")
		return
	}

	fmt.Printf("找到 %d 条已整理电影记录\n", len(mediaFiles))
	count := 0
	found := 0
	ambiguousIDs := make([]string, 0, 32)
	noResultIDs := make([]string, 0, 32)
	updated := 0
	var cacheWriter *os.File
	if *writeCache {
		if err := os.MkdirAll(filepath.Dir(*cacheFile), 0755); err != nil {
			fmt.Printf("创建缓存目录失败: %v\n", err)
			return
		}
		f, err := os.OpenFile(*cacheFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("打开缓存文件失败: %v\n", err)
			return
		}
		cacheWriter = f
		defer cacheWriter.Close()
	}

	runners := make(map[uint]renameRunner)

	for i, mf := range mediaFiles {
		if *limit > 0 && count >= *limit {
			break
		}
		if *resume && processed[mf.ID] {
			continue
		}
		count++
		printProgress := *progressEvery > 0 && count%*progressEvery == 0

		scrapePath := models.GetScrapePathByID(mf.ScrapePathId)
		if scrapePath == nil {
			fmt.Printf("跳过 ID=%d: 未找到刮削目录\n", mf.ID)
			continue
		}

		info := helpers.ExtractMediaInfoRe(mf.VideoFilename, true, false, scrapePath.VideoExtList, scrapePath.DeleteKeyword...)
		if info == nil || strings.TrimSpace(info.Name) == "" {
			continue
		}
		rawName := strings.TrimSpace(info.Name)
		queryName := rawName
		queryYear := info.Year
		rawResultCount, ambiguous, err := checkByNameAndYear(models.GlobalScrapeSettings, queryName, queryYear)
		if err != nil {
			continue
		}
		initialRawCount := rawResultCount
		if rawResultCount == 0 {
			altName, _ := deriveQueryNameYear(mf.VideoFilename)
			if altName != "" && !strings.EqualFold(altName, rawName) {
				queryName = altName
				rawResultCount, ambiguous, err = checkByNameAndYear(models.GlobalScrapeSettings, queryName, queryYear)
				if err != nil {
					continue
				}
			}
		}
		if rawResultCount == 0 {
			cleaned := cleanQueryName(rawName, scrapePath.DeleteKeyword)
			if cleaned != "" && !strings.EqualFold(cleaned, queryName) {
				queryName = cleaned
				rawResultCount, ambiguous, err = checkByNameAndYear(models.GlobalScrapeSettings, queryName, queryYear)
				if err != nil {
					continue
				}
			}
		}
		if printProgress {
			fmt.Printf("进度: %d/%d ID=%d tmdb=%d file=%s name=%s year=%d results=%d raw_name=%s raw_results=%d\n", count, len(mediaFiles), mf.ID, mf.TmdbId, mf.VideoFilename, queryName, queryYear, rawResultCount, rawName, initialRawCount)
		}
		noResult := rawResultCount == 0
		needTag := ambiguous || noResult

		mf.QueryRelation()
		if mf.Media == nil && mf.MediaId > 0 {
			media := &models.Media{}
			if err := db.Db.Where("id = ?", mf.MediaId).First(media).Error; err == nil {
				mf.Media = media
			}
		}

		ext := filepath.Ext(mf.VideoFilename)
		base := strings.TrimSuffix(mf.VideoFilename, ext)
		hasTmdbTag := helpers.ExtractTmdbId(base) > 0
		newBase := base
		newName := base + ext
		skipReason := ""
		if needTag {
			if mf.TmdbId == 0 {
				skipReason = "tmdb_id_empty"
				if noResult {
					fmt.Printf("[noresult-skip] %d/%d ID=%d file=%s reason=%s\n", i+1, len(mediaFiles), mf.ID, mf.VideoFilename, skipReason)
				}
				newName = ""
			} else {
				newBase = fmt.Sprintf("%s.{tmdbid-%d}", base, mf.TmdbId)
				newName = newBase + ext
			}
		} else if hasTmdbTag {
			// results==1 but源文件名已经包含tmdbid，不额外处理
			skipReason = "already_tagged"
		}

		if ambiguous {
			if newName != "" {
				fmt.Printf("[ambiguous] %d/%d ID=%d %s => %s\n", i+1, len(mediaFiles), mf.ID, mf.VideoFilename, newName)
			} else if skipReason != "" {
				fmt.Printf("[ambiguous-skip] %d/%d ID=%d file=%s reason=%s\n", i+1, len(mediaFiles), mf.ID, mf.VideoFilename, skipReason)
			}
			if *printIdList {
				ambiguousIDs = append(ambiguousIDs, fmt.Sprintf("%d", mf.ID))
			}
		} else if noResult {
			if newName != "" {
				fmt.Printf("[noresult] %d/%d ID=%d %s => %s\n", i+1, len(mediaFiles), mf.ID, mf.VideoFilename, newName)
			} else if skipReason != "" {
				fmt.Printf("[noresult-skip] %d/%d ID=%d file=%s reason=%s\n", i+1, len(mediaFiles), mf.ID, mf.VideoFilename, skipReason)
			}
			if *printIdList {
				noResultIDs = append(noResultIDs, fmt.Sprintf("%d", mf.ID))
			}
		}
		if needTag {
			found++
		}
		if needTag && *writeCache && cacheWriter != nil {
			entry := ambiguousEntry{
				ID:             mf.ID,
				MediaId:        mf.MediaId,
				ScrapePathId:   mf.ScrapePathId,
				FileId:         fileIdForCache(mf),
				CurrentName:    mf.VideoFilename,
				NewName:        newName,
				TmdbId:         mf.TmdbId,
				Name:           mf.Name,
				Year:           mf.Year,
				AlreadyTagged:  hasTmdbTag,
				SkipReason:     skipReason,
				QueryName:      queryName,
				QueryYear:      queryYear,
				ResultCount:    rawResultCount,
				RawQueryName:   rawName,
				RawResultCount: initialRawCount,
			}
			if err := appendJsonl(cacheWriter, entry); err != nil {
				fmt.Printf("写入缓存失败 ID=%d: %v\n", mf.ID, err)
			}
		}
		if !*apply || *dryRun {
			continue
		}

		runner, ok := runners[mf.ScrapePathId]
		if !ok {
			r, err := buildRenameRunner(scrapePath)
			if err != nil {
				fmt.Printf("获取重命名实现失败 ID=%d: %v\n", mf.ID, err)
				continue
			}
			runner = r
			runners[mf.ScrapePathId] = runner
		}

		fileId := mf.VideoFileId
		if mf.Media != nil && mf.Media.VideoFileId != "" {
			fileId = mf.Media.VideoFileId
		}
		if fileId == "" {
			fmt.Printf("重命名失败 ID=%d: 文件ID为空\n", mf.ID)
			continue
		}
		if newName == "" {
			continue
		}
		if mf.Media != nil && mf.Media.VideoFileName == newName {
			continue
		}
		err = runner.Rename(fileId, newName)
		if err != nil {
			fmt.Printf("重命名失败 ID=%d: %v\n", mf.ID, err)
			retryOK := false
			if isNotFound(err) && scrapePath.SourceType == models.SourceType115 {
				fullPath := filepath.Join(scrapePath.SourcePath, mf.VideoFilename)
				if newId, ferr := resolve115FileIdByPath(scrapePath, fullPath); ferr == nil && newId != "" {
					if rerr := runner.Rename(newId, newName); rerr == nil {
						mf.VideoFileId = newId
						if mf.Media != nil {
							mf.Media.VideoFileId = newId
						}
						fmt.Printf("文件ID已刷新并重试成功 ID=%d newFileId=%s\n", mf.ID, newId)
						retryOK = true
					} else {
						fmt.Printf("重试重命名失败 ID=%d: %v\n", mf.ID, rerr)
					}
				} else {
					fmt.Printf("刷新文件ID失败 ID=%d: %v\n", mf.ID, ferr)
				}
			}
			if retryOK {
				// 继续正常写库
			} else {
				if *stopOnTokenExpired && isTokenExpired(err) {
					fmt.Println("检测到Token过期，已停止执行。请更新凭证后重试（支持断点续跑）。")
					saveCheckpoint()
					break
				}
				continue
			}
		}

		mf.NewVideoBaseName = newBase
		if mf.Media != nil {
			mf.Media.VideoFileName = newName
		}
		mf.Save()
		if mf.Media != nil {
			mf.Media.Save()
		}
		updated++
		processed[mf.ID] = true
		saveCheckpoint()
		if *sleepMs > 0 {
			time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
		}
	}

	if *apply && !*dryRun {
		fmt.Printf("完成：更新 %d 条记录\n", updated)
	} else {
		fmt.Printf("完成：发现 %d 条需要追加tmdbid的记录（dry-run）\n", found)
	}
	if *printIdList {
		if len(ambiguousIDs) > 0 {
			fmt.Printf("ambiguous IDs: %s\n", strings.Join(ambiguousIDs, ","))
		} else {
			fmt.Println("ambiguous IDs: (none)")
		}
		if len(noResultIDs) > 0 {
			fmt.Printf("noresult IDs: %s\n", strings.Join(noResultIDs, ","))
		} else {
			fmt.Println("noresult IDs: (none)")
		}
	}
	if *writeCache {
		fmt.Printf("缓存已写入: %s\n", *cacheFile)
	}
}

func fileIdForCache(mf *models.ScrapeMediaFile) string {
	if mf.Media != nil && mf.Media.VideoFileId != "" {
		return mf.Media.VideoFileId
	}
	if mf.VideoFileId != "" {
		return mf.VideoFileId
	}
	return ""
}

func appendJsonl(w *os.File, entry ambiguousEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func readJsonlCache(path string) ([]ambiguousEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	entries := make([]ambiguousEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry ambiguousEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func isTokenExpired(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Token") || strings.Contains(msg, "凭证") || strings.Contains(msg, "token")
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "430004") || strings.Contains(msg, "不存在") || strings.Contains(msg, "已删除")
}

func resolve115FileIdByPath(sp *models.ScrapePath, fullPath string) (string, error) {
	account, err := sp.GetAccount()
	if err != nil {
		return "", err
	}
	client := account.Get115Client(true)
	detail, err := client.GetFsDetailByPath(context.Background(), fullPath)
	if err == nil && detail != nil && detail.FileId != "" {
		return detail.FileId, nil
	}
	trimmed := strings.TrimPrefix(fullPath, "/")
	if trimmed != fullPath {
		detail, err = client.GetFsDetailByPath(context.Background(), trimmed)
		if err == nil && detail != nil && detail.FileId != "" {
			return detail.FileId, nil
		}
	}
	return "", fmt.Errorf("未找到文件路径: %s", fullPath)
}
