package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/scrape"
	"Q115-STRM/internal/scrape/scan"
	"Q115-STRM/internal/synccron"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 基础配置
// TMDB API KEY或者Access Token设置
// 1. TMDB API KEY或者Access Token设置查询和保存
// 2. AI识别设置查询和保存
// 分类规则
// 1 分类列表（分电影和电视剧）
// 2 分类添加和编辑(分电影和电视剧)
// 分类数据
// 1. 电影分类数组
// 2. 电视剧分类数组
// 语言数组
// 1. 语言数组

type TmdbSettings struct {
	TmdbApiKey        string `json:"tmdb_api_key" form:"tmdb_api_key"`
	TmdbAccessToken   string `json:"tmdb_access_token" form:"tmdb_access_token"`
	TmdbUrl           string `json:"tmdb_url" form:"tmdb_url"`
	TmdbImageUrl      string `json:"tmdb_image_url" form:"tmdb_image_url"`
	TmdbLanguage      string `json:"tmdb_language" form:"tmdb_language"`
	TmdbImageLanguage string `json:"tmdb_image_language" form:"tmdb_image_language"`
	TmdbEnableProxy   bool   `json:"tmdb_enable_proxy" form:"tmdb_enable_proxy"`
}

type AiSettings struct {
	EnableAi    models.AiAction `json:"enable_ai" form:"enable_ai"`
	AiApiKey    string          `json:"ai_api_key" form:"ai_api_key"`
	AiBaseUrl   string          `json:"ai_base_url" form:"ai_base_url"`
	AiModelName string          `json:"ai_model_name" form:"ai_model_name"`
	AiPrompt    string          `json:"ai_prompt" form:"ai_prompt"`
	AiTimeout   int             `json:"ai_timeout" form:"ai_timeout"`
}

type MovieCategoryReq struct {
	ID            uint     `json:"id" form:"id"`
	Name          string   `json:"name" form:"name"`
	LanguageArray []string `json:"language_array" form:"language_array"`
	GenreIDArray  []int    `json:"genre_id_array" form:"genre_id_array"`
}

type TvshowCategoryReq struct {
	ID           uint     `json:"id" form:"id"`
	Name         string   `json:"name" form:"name"`
	CountryArray []string `json:"country_array" form:"country_array"`
	GenreIDArray []int    `json:"genre_id_array" form:"genre_id_array"`
}

func GetTmdbSettings(c *gin.Context) {
	tmdbSettings := TmdbSettings{
		TmdbApiKey:        models.GlobalScrapeSettings.TmdbApiKey,
		TmdbAccessToken:   models.GlobalScrapeSettings.TmdbAccessToken,
		TmdbUrl:           models.GlobalScrapeSettings.TmdbUrl,
		TmdbImageUrl:      models.GlobalScrapeSettings.TmdbImageUrl,
		TmdbLanguage:      models.GlobalScrapeSettings.TmdbLanguage,
		TmdbImageLanguage: models.GlobalScrapeSettings.TmdbImageLanguage,
		TmdbEnableProxy:   models.GlobalScrapeSettings.TmdbEnableProxy,
	}
	c.JSON(http.StatusOK, APIResponse[TmdbSettings]{Code: Success, Message: "", Data: tmdbSettings})
}

func SaveTmdbSettings(c *gin.Context) {
	reqData := TmdbSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if err := models.GlobalScrapeSettings.SaveTmdb(reqData.TmdbApiKey, reqData.TmdbAccessToken, reqData.TmdbUrl, reqData.TmdbImageUrl, reqData.TmdbLanguage, reqData.TmdbImageLanguage, reqData.TmdbEnableProxy); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存TMDB设置成功", Data: nil})
}

func TestTmdbSettings(c *gin.Context) {
	reqData := TmdbSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	tmpScrapeSetting := &models.ScrapeSettings{
		TmdbApiKey:        reqData.TmdbApiKey,
		TmdbAccessToken:   reqData.TmdbAccessToken,
		TmdbUrl:           reqData.TmdbUrl,
		TmdbImageUrl:      reqData.TmdbImageUrl,
		TmdbLanguage:      reqData.TmdbLanguage,
		TmdbImageLanguage: reqData.TmdbImageLanguage,
		TmdbEnableProxy:   reqData.TmdbEnableProxy,
	}
	testResult := tmpScrapeSetting.TestTmdb()
	c.JSON(http.StatusOK, APIResponse[bool]{Code: Success, Message: "", Data: testResult})
}

func SaveAiSettings(c *gin.Context) {
	reqData := AiSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if err := models.GlobalScrapeSettings.SaveAi(reqData.AiApiKey, reqData.AiBaseUrl, reqData.AiModelName, reqData.AiTimeout); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存AI识别设置成功", Data: nil})
}

func TestAiSettings(c *gin.Context) {
	reqData := AiSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if reqData.AiApiKey == "" || reqData.AiBaseUrl == "" || reqData.AiModelName == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "必须配置API Key、接口地址模型名称", Data: nil})
		return
	}
	tmpScrapeSetting := &models.ScrapeSettings{
		AiApiKey:    reqData.AiApiKey,
		AiBaseUrl:   reqData.AiBaseUrl,
		AiModelName: reqData.AiModelName,
	}
	testResult := tmpScrapeSetting.TestAi()
	if testResult != nil {
		c.JSON(http.StatusOK, APIResponse[error]{Code: BadRequest, Message: testResult.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[error]{Code: Success, Message: "测试AI识别成功", Data: nil})
}

func GetAiSettings(c *gin.Context) {
	aiSettings := AiSettings{
		AiApiKey:    models.GlobalScrapeSettings.AiApiKey,
		AiBaseUrl:   models.GlobalScrapeSettings.AiBaseUrl,
		AiModelName: models.GlobalScrapeSettings.AiModelName,
		AiTimeout:   models.GlobalScrapeSettings.AiTimeout,
	}
	c.JSON(http.StatusOK, APIResponse[AiSettings]{Code: Success, Message: "", Data: aiSettings})
}

// 电影分类
func GetMovieGenre(c *gin.Context) {
	movieGenre := helpers.MovieGenres
	c.JSON(http.StatusOK, APIResponse[[]helpers.Genre]{Code: Success, Message: "", Data: movieGenre})
}

// 电视剧分类
func GetTvshowGenre(c *gin.Context) {
	tvshowGenre := helpers.TvshowGenres
	c.JSON(http.StatusOK, APIResponse[[]helpers.Genre]{Code: Success, Message: "", Data: tvshowGenre})
}

// 语言数组
func GetLanguage(c *gin.Context) {
	language := helpers.Languages
	c.JSON(http.StatusOK, APIResponse[[]helpers.Language]{Code: Success, Message: "", Data: language})
}

func GetCountries(c *gin.Context) {
	countries := helpers.Countries
	c.JSON(http.StatusOK, APIResponse[[]helpers.Country]{Code: Success, Message: "", Data: countries})
}

// 电影分类列表
func GetMovieCategories(c *gin.Context) {
	movieGenre := models.GetMovieCategory()
	c.JSON(http.StatusOK, APIResponse[[]*models.MovieCategory]{Code: Success, Message: "", Data: movieGenre})
}

// 电视剧分类列表
func GetTvshowCategories(c *gin.Context) {
	tvshowGenre := models.GetTvshowCategory()
	c.JSON(http.StatusOK, APIResponse[[]*models.TvShowCategory]{Code: Success, Message: "", Data: tvshowGenre})
}

// 保存电影分类
func SaveMovieCategory(c *gin.Context) {
	reqData := MovieCategoryReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	movieCategory := &models.MovieCategory{
		BaseModel: models.BaseModel{
			ID: reqData.ID,
		},
		Name: reqData.Name,
	}
	if err := movieCategory.Save(reqData.Name, reqData.GenreIDArray, reqData.LanguageArray); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存电影分类成功", Data: nil})
}

// 保存电视剧分类
func SaveTvshowCategory(c *gin.Context) {
	reqData := TvshowCategoryReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	tvshowCategory := &models.TvShowCategory{
		BaseModel: models.BaseModel{
			ID: reqData.ID,
		},
		Name: reqData.Name,
	}
	if err := tvshowCategory.Save(reqData.Name, reqData.GenreIDArray, reqData.CountryArray); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存电视剧分类成功", Data: nil})
}

// 删除电影分类
func DeleteMovieCategory(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))

	if err := models.DeleteMovieCategory(uint(id)); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除电影分类成功", Data: nil})
}

// 删除电视剧分类
func DeleteTvshowCategory(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	if err := models.DeleteTvshowCategory(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除电视剧分类成功", Data: nil})
}

// 查询同步目录列表
func GetScrapePathes(c *gin.Context) {
	scrapePathes := models.GetScrapePathes()
	for _, scrapePath := range scrapePathes {
		// 检查是否正在运行
		scrapePath.IsTaskRunning = synccron.CheckTaskIsRunning(scrapePath.ID, synccron.SyncTaskTypeScrape)
	}
	c.JSON(http.StatusOK, APIResponse[[]*models.ScrapePath]{Code: Success, Message: "", Data: scrapePathes})
}

// 查询同步目录详情
func GetScrapePath(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	scrapePath := models.GetScrapePathByID(uint(id))
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	if scrapePath.EnableAi == "" {
		scrapePath.EnableAi = models.AiActionOff
	}
	c.JSON(http.StatusOK, APIResponse[*models.ScrapePath]{Code: Success, Message: "", Data: scrapePath})
}

// 添加或编辑同步目录
func SaveScrapePath(c *gin.Context) {
	reqData := models.ScrapePath{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	// 如果是115，用ID查询实际的目录
	if reqData.SourceType == models.SourceType115 {
		// 用ID查询实际的目录
		account, err := models.GetAccountById(reqData.AccountId)
		if err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
			return
		}
		// 用ID查询实际的目录
		sourcePath := models.GetPathByPathFileId(account, reqData.SourcePathId)
		if sourcePath == "" {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询来源目录失败", Data: nil})
			return
		}
		reqData.SourcePath = sourcePath
		if reqData.DestPathId != "" {
			destPath := models.GetPathByPathFileId(account, reqData.DestPathId)
			if destPath == "" {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询目标目录失败", Data: nil})
				return
			}
			reqData.DestPath = destPath
		}
	}
	helpers.AppLogger.Infof("最大线程数：%d", reqData.MaxThreads)
	if err := reqData.Save(); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存刮削目录成功", Data: nil})
}

// 删除同步目录
func DeleteScrapePath(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	if err := models.DeleteScrapePath(uint(id)); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除刮削目录成功", Data: nil})
}

// 启动识别刮削
func ScanScrapePath(c *gin.Context) {
	type ScanScrapePathReq struct {
		ID uint `json:"id" form:"id"`
	}
	reqData := ScanScrapePathReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	// 查询ScrapePath
	scrapePath := models.GetScrapePathByID(reqData.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	if err := synccron.AddSyncTask(scrapePath.ID, synccron.SyncTaskTypeScrape); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "添加刮削任务失败: " + err.Error(), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "刮削任务已添加到队列", Data: nil})
}

// 刮削记录
func GetScrapeRecords(c *gin.Context) {
	page := helpers.StringToInt(c.Query("page"))
	if page == 0 {
		page = 1
	}
	pageSize := helpers.StringToInt(c.Query("pageSize"))
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	mediaType := c.Query("type")
	status := c.Query("status")
	name := c.Query("name")
	scrapePathesCache := make(map[uint]*models.ScrapePath)
	total, scrapeRecords := models.GetScrapeMediaFilesLite(page, pageSize, mediaType, status, name)
	type scrapeMediaResp struct {
		ID              uint   `json:"id"`
		Type            string `json:"type"`
		Path            string `json:"path"`
		FileName        string `json:"file_name"`
		MediaName       string `json:"media_name"`
		OriginalName    string `json:"original_name"`
		Year            int    `json:"year"`
		SeasonNumber    int    `json:"season_number"`
		EpisodeNumber   int    `json:"episode_number"`
		Genre           string `json:"genre"`
		Country         string `json:"country"`
		Language        string `json:"language"`
		Status          string `json:"status"`
		TmdbID          int64  `json:"tmdb_id"`
		NewPath         string `json:"new_path"` // 二级分类 + 文件夹
		NewFile         string `json:"new_file"` // 新文件名
		EpisodeName     string `json:"episode_name"`
		Resolution      string `json:"resolution"`       // 分辨率
		ResolutionLevel string `json:"resolution_level"` // 分辨率等级
		IsHDR           bool   `json:"is_hdr"`           // 是否HDR
		AudioCount      int    `json:"audio_count"`      // 音频轨道数量
		SubtitleCount   int    `json:"subtitle_count"`   // 字幕轨道数量
		CreatedAt       int64  `json:"created_at"`       // 创建时间
		UpdatedAt       int64  `json:"updated_at"`       // 更新时间
		ScrapedAt       int64  `json:"scraped_at"`       // 刮削时间
		ScannedAt       int64  `json:"scanned_at"`       // 扫描时间
		RenamedAt       int64  `json:"renamed_at"`       // 整理时间
		FailedReason    string `json:"failed_reason"`    // 失败原因
		CategoryName    string `json:"category_name"`    // 分类名称
		NewDestPath     string `json:"new_dest_path"`    // 新路径
		NewDestName     string `json:"new_dest_name"`    // 新文件名
		PathIsScraping  bool   `json:"path_is_scraping"` // 是否正在刮削
		PathIsRenaming  bool   `json:"path_is_renaming"` // 是否正在整理
		SourceFullPath  string `json:"source_full_path"` // 原始路径
		DestFullPath    string `json:"dest_full_path"`   // 目标路径
		SourceType      string `json:"source_type"`      // 原始类型
		RenameType      string `json:"rename_type"`      // 重命名类型
		ScrapeType      string `json:"scrape_type"`      // 刮削类型
	}
	type scrapeListResp struct {
		Total int64              `json:"total"`
		List  []*scrapeMediaResp `json:"list"`
	}
	resp := scrapeListResp{
		Total: total,
		List:  make([]*scrapeMediaResp, len(scrapeRecords)),
	}
	episodeIds := make([]uint, 0)
	for _, scrapeMedia := range scrapeRecords {
		if scrapeMedia.MediaEpisodeId > 0 {
			episodeIds = append(episodeIds, scrapeMedia.MediaEpisodeId)
		}
	}
	episodeNameMap := models.GetMediaEpisodeNamesByIds(episodeIds)
	for i, scrapeMedia := range scrapeRecords {
		var scrapePath *models.ScrapePath
		var ok bool
		if scrapePath, ok = scrapePathesCache[scrapeMedia.ScrapePathId]; !ok {
			scrapePath = models.GetScrapePathByID(scrapeMedia.ScrapePathId)
			if scrapePath == nil {
				continue
			}
			scrapePathesCache[scrapeMedia.ScrapePathId] = scrapePath
		}
		if scrapePath == nil {
			continue
		}
		sourcePath := scrapeMedia.GetRemoteFullMoviePath()
		destPath := scrapeMedia.GetDestFullMoviePath()
		if scrapeMedia.MediaType == models.MediaTypeTvShow {
			sourcePath = scrapeMedia.GetRemoteFullSeasonPath()
			destPath = scrapeMedia.GetDestFullSeasonPath()
		}
		sourcePath = filepath.Join(sourcePath, scrapeMedia.VideoFilename)
		destPath = filepath.Join(destPath, scrapeMedia.NewVideoBaseName+scrapeMedia.VideoExt)

		resp.List[i] = &scrapeMediaResp{
			Type:            string(scrapeMedia.MediaType),
			ID:              scrapeMedia.ID,
			Path:            scrapeMedia.Path,
			FileName:        scrapeMedia.VideoFilename,
			MediaName:       scrapeMedia.Name,
			Year:            scrapeMedia.Year,
			SeasonNumber:    scrapeMedia.SeasonNumber,
			EpisodeNumber:   scrapeMedia.EpisodeNumber,
			Status:          string(scrapeMedia.Status),
			TmdbID:          scrapeMedia.TmdbId,
			NewPath:         filepath.Join(scrapeMedia.CategoryName, scrapeMedia.NewPathName),
			NewFile:         scrapeMedia.NewVideoBaseName + scrapeMedia.VideoExt,
			Resolution:      scrapeMedia.Resolution,
			ResolutionLevel: scrapeMedia.ResolutionLevel,
			IsHDR:           scrapeMedia.IsHDR,
			AudioCount:      0,
			SubtitleCount:   0,
			CreatedAt:       scrapeMedia.CreatedAt,
			UpdatedAt:       scrapeMedia.UpdatedAt,
			ScrapedAt:       scrapeMedia.ScrapeTime,
			ScannedAt:       scrapeMedia.ScanTime,
			RenamedAt:       scrapeMedia.RenameTime,
			CategoryName:    scrapeMedia.CategoryName,
			NewDestPath:     scrapeMedia.NewPathName,
			NewDestName:     scrapeMedia.NewVideoBaseName + scrapeMedia.VideoExt,
			FailedReason:    scrapeMedia.FailedReason,
			SourceFullPath:  sourcePath,
			DestFullPath:    destPath,
			SourceType:      string(scrapePath.SourceType),
			RenameType:      string(scrapePath.RenameType),
			ScrapeType:      string(scrapePath.ScrapeType),
		}
		if scrapeMedia.AudioCodecJson != "" {
			audioCodec, err := helpers.StringJson[[]*models.AudioCodec](scrapeMedia.AudioCodecJson)
			if err == nil {
				resp.List[i].AudioCount = len(audioCodec)
			}
		}
		if scrapeMedia.SubtitleCodecJson != "" {
			subtitleCodec, err := helpers.StringJson[[]*models.Subtitle](scrapeMedia.SubtitleCodecJson)
			if err == nil {
				resp.List[i].SubtitleCount = len(subtitleCodec)
			}
		}
		if scrapeMedia.MediaType == models.MediaTypeTvShow {
			if name, ok := episodeNameMap[scrapeMedia.MediaEpisodeId]; ok {
				resp.List[i].EpisodeName = name
			}
			resp.List[i].NewPath = filepath.Join(resp.List[i].NewPath, scrapeMedia.NewSeasonPathName)
		}
		if scrapePath.IsScraping && slices.Contains([]models.ScrapeMediaStatus{models.ScrapeMediaStatusScanned, models.ScrapeMediaStatusScraping}, scrapeMedia.Status) {
			resp.List[i].PathIsScraping = scrapePath.IsScraping
		}
		// if scrapeMedia.Media != nil {
		// 	if len(scrapeMedia.Media.Genres) > 0 {
		// 		for _, genre := range scrapeMedia.Media.Genres {
		// 			resp.List[i].Genre += genre.Name + ", "
		// 		}
		// 	}
		// 	if len(scrapeMedia.Media.OriginCountry) > 0 {
		// 		countries := ""
		// 		for _, country := range scrapeMedia.Media.OriginCountry {
		// 			if countries == "" {
		// 				countries = helpers.GetCountryName(country)
		// 			} else {
		// 				countries += ", " + helpers.GetCountryName(country)
		// 			}

		// 		}
		// 		resp.List[i].Country = countries
		// 	}
		// 	if scrapeMedia.Media.OriginalLanguage != "" {
		// 		resp.List[i].Language = helpers.GetLanguageName(scrapeMedia.Media.OriginalLanguage)
		// 	}
		// 	if scrapeMedia.Media.OriginalName != "" {
		// 		resp.List[i].OriginalName = scrapeMedia.Media.OriginalName
		// 	}
		// }
	}
	c.JSON(http.StatusOK, APIResponse[scrapeListResp]{Code: Success, Message: "", Data: resp})
}

func GetTmpImageUrl(path string, mediaType models.MediaType) string {
	return fmt.Sprintf("/scrape/tmp-image?path=%s&type=%s", url.QueryEscape(path), url.QueryEscape(string(mediaType)))
}

// 读取临时静态文件返回给客户端
// 主要是图片
func ScrapeTmpImage(c *gin.Context) {
	imagePath := c.Query("path")
	mediaType := models.MediaType(c.Query("type"))
	if imagePath == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "路径不能为空", Data: nil})
		return
	}
	imageRootPath := filepath.Join(helpers.RootDir, "config/tmp/刮削临时文件")
	if mediaType == models.MediaTypeTvShow {
		imageRootPath = filepath.Join(imageRootPath, "电视剧")
	} else {
		imageRootPath = filepath.Join(imageRootPath, "电影或其他")
	}
	imagePath = filepath.Join(imageRootPath, imagePath)
	// 读取文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "读取文件失败: " + err.Error(), Data: nil})
		return
	}
	defer file.Close()
	// 读取文件内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "读取文件内容失败: " + err.Error(), Data: nil})
		return
	}
	// 返回文件内容
	c.Data(http.StatusOK, "image/jpeg", fileContent)
}

func ExportScrapeRecords(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	scrapeRecords := models.GetScrapeMediaFilesByIds(idUintList)
	if len(scrapeRecords) == 0 {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "没有找到要导出的记录", Data: nil})
		return
	}
	// 生成txt文件内容
	type exportMediaResult struct {
		Path          string `json:"path"`
		FileName      string `json:"filename"`
		TvPath        string `json:"tv_path"`
		Name          string `json:"name"`
		Year          int    `json:"year"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
	}
	exportMediaList := make([]exportMediaResult, 0)
	for _, scrapeMedia := range scrapeRecords {
		exportMediaList = append(exportMediaList, exportMediaResult{
			Path:          scrapeMedia.Path,
			FileName:      scrapeMedia.VideoFilename,
			TvPath:        scrapeMedia.TvshowPath,
			Name:          scrapeMedia.Name,
			Year:          scrapeMedia.Year,
			SeasonNumber:  scrapeMedia.SeasonNumber,
			EpisodeNumber: scrapeMedia.EpisodeNumber,
		})
	}
	// json 格式化exportMediaList
	exportMediaListJson, err := json.MarshalIndent(exportMediaList, "", "  ")
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "格式化导出记录失败: " + err.Error(), Data: nil})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=刮削记录.json")
	// 返回json文件内容,触发浏览器下载
	c.Data(http.StatusOK, "application/json", exportMediaListJson)
}

// 重新刮削某个失败的记录，使用用户输入的名称和年份
func ReScrape(c *gin.Context) {
	type reScrapeReq struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		Year    int    `json:"year"`
		TmdbId  int64  `json:"tmdb_id"`
		Season  int    `json:"season"`
		Episode int    `json:"episode"`
	}
	var req reScrapeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapeMediaFile
	scrapeMedia := models.GetScrapeMediaFileById(req.ID)
	if scrapeMedia == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要重新刮削的记录", Data: nil})
		return
	}
	scrapePath := models.GetScrapePathByID(scrapeMedia.ScrapePathId)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要重新刮削的记录的刮削目录", Data: nil})
		return
	}
	oldStatus := scrapeMedia.Status
	noRollback := oldStatus == models.ScrapeMediaStatusRenamed
	err := scrapeMedia.ReScrape(req.Name, req.Year, req.TmdbId, req.Season, req.Episode, noRollback)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "重新刮削失败: " + err.Error(), Data: nil})
		return
	}
	if oldStatus == models.ScrapeMediaStatusRenamed {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，已在后台启动重新识别（不回滚源文件）...", Data: nil})
	} else {
		data := make(map[string]any)
		data["name"] = scrapeMedia.Name
		data["year"] = scrapeMedia.Year
		data["tmdb_id"] = scrapeMedia.TmdbId
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，已在后台启动刮削任务...", Data: data})
	}

	// 立即触发异步刮削
	if scrapeMedia.TmdbId > 0 {
		go func() {
			helpers.AppLogger.Infof("手动确认 ID 后立即触发刮削：ID=%d, Movie=%s", scrapeMedia.ID, scrapeMedia.Name)
			scraper := scrape.NewScrape(scrapePath)
			originalType := scrapeMedia.ScrapeType
			if noRollback {
				scrapeMedia.ScrapeType = models.ScrapeTypeOnly
			}
			err := scraper.ProcessSingle(scrapeMedia)
			if err != nil {
				helpers.AppLogger.Errorf("手动确认后的刮削失败：ID=%d, 错误=%v", scrapeMedia.ID, err)
			}
			if noRollback {
				scrapeMedia.ScrapeType = originalType
				scrapeMedia.Save()
			}
		}()
	}
}

// 清除所有刮削失败的记录
func ClearFailedScrapeRecords(c *gin.Context) {
	err := models.ClearFailedScrapeRecords([]uint{})
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所有失败的刮削记录已清除", Data: nil})
}

// 将整理中的记录标记为已整理
// 检查源文件是否存在
// 检查目标文件是否存在
func FinishScrapeMediaFile(c *gin.Context) {
	type finishScrapeReq struct {
		ID uint `json:"id"`
	}
	var req finishScrapeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapeMediaFile
	scrapeMedia := models.GetScrapeMediaFileById(req.ID)
	if scrapeMedia == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要整理的记录", Data: nil})
		return
	}
	scrapeMedia.FinishFromRenaming()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，记录已标记为已整理", Data: nil})
}

// 删除选定的刮削记录
func DeleteScrapeMediaFile(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	// 删除记录
	err := models.ClearFailedScrapeRecords(idUintList)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "删除记录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，记录已删除", Data: nil})
}

// 将所选刮削记录标记为待整理
func RenameFailedScrapeMediaFile(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	// 将这些ID对应的记录标记为待整理
	err := models.RenameFailedScrapeRecords(idUintList)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "标记记录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所选记录已标记为待整理", Data: nil})
}

func ToggleScrapePathCron(c *gin.Context) {
	type toggleScrapePathCronReq struct {
		ID uint `json:"id"`
	}
	var req toggleScrapePathCronReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapePath
	scrapePath := models.GetScrapePathByID(req.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要操作的记录", Data: nil})
		return
	}
	// 切换定时刮削
	err := scrapePath.ToggleCron()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "切换定时刮削失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，定时刮削已切换", Data: nil})
}

func StopScrape(c *gin.Context) {
	type ScanScrapePathReq struct {
		ID uint `json:"id" form:"id"`
	}
	var req ScanScrapePathReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	synccron.StopSyncTask(req.ID, synccron.SyncTaskTypeScrape)
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，刮削任务已停止", Data: nil})
}

// TruncateAllScrapeRecords 一键清空所有刮削记录
func TruncateAllScrapeRecords(c *gin.Context) {
	err := models.TruncateAllScrapeRecords()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "清空刮削记录失败: " + err.Error(), Data: nil})
		return
	}
	// 同时清除目录扫描缓存，使下次扫描变为全量扫描
	scan.ClearScannedDirsCache()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所有刮削记录已清空，下次扫描将执行全量扫描", Data: nil})
}

// ExecuteSingleScrape 单独执行某条记录的刮削（不需要等待下次全量扫描）
// 适用于用户手动确认 tmdbid 后，希望立即执行刮削的场景
func ExecuteSingleScrape(c *gin.Context) {
	type singleScrapeReq struct {
		ID uint `json:"id"`
	}
	var req singleScrapeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}

	// 查询 ScrapeMediaFile
	scrapeMedia := models.GetScrapeMediaFileById(req.ID)
	if scrapeMedia == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要刮削的记录", Data: nil})
		return
	}

	// 检查状态：只有待刮削(scanned)或刮削失败(scrape_failed)的记录才能单独执行
	if scrapeMedia.Status != models.ScrapeMediaStatusScanned && scrapeMedia.Status != models.ScrapeMediaStatusScrapeFailed {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("当前状态 %s 不允许执行刮削，只有【待刮削】或【刮削失败】的记录才能执行", scrapeMedia.Status), Data: nil})
		return
	}

	// 检查是否有 tmdbid
	if scrapeMedia.TmdbId == 0 && scrapeMedia.MediaType != models.MediaTypeOther {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请先确认 TMDB ID 后再执行刮削", Data: nil})
		return
	}

	// 获取对应的刮削目录
	scrapePath := models.GetScrapePathByID(scrapeMedia.ScrapePathId)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到对应的刮削目录配置", Data: nil})
		return
	}

	// 检查刮削目录是否正在运行
	if scrapePath.IsRunning() {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录正在运行中，请稍后再试", Data: nil})
		return
	}

	// 将状态改为待刮削，确保可以被处理
	scrapeMedia.Status = models.ScrapeMediaStatusScanned
	scrapeMedia.Save()

	// 在后台执行刮削
	go func() {
		helpers.AppLogger.Infof("开始单独执行刮削：ID=%d, 文件名=%s", scrapeMedia.ID, scrapeMedia.VideoFilename)
		scraper := scrape.NewScrape(scrapePath)
		// 直接调用 Process 方法处理单条记录
		err := scraper.ProcessSingle(scrapeMedia)
		if err != nil {
			helpers.AppLogger.Errorf("单独执行刮削失败：ID=%d, 错误=%v", scrapeMedia.ID, err)
		} else {
			helpers.AppLogger.Infof("单独执行刮削成功：ID=%d, 文件名=%s", scrapeMedia.ID, scrapeMedia.VideoFilename)
		}
	}()

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "刮削任务已在后台启动，请稍后刷新查看结果", Data: nil})
}

// ExecuteBatchScrapeByTmdbid 批量触发所有已有 tmdbid 但未完成的记录
func ExecuteBatchScrapeByTmdbid(c *gin.Context) {
	var mediaFiles []*models.ScrapeMediaFile
	// 查询 [待刮削/刮削失败/整理失败] 且 [已有 TmdbId] 的记录
	models.GetDb().Where("tmdb_id > 0 AND status IN (?)", []models.ScrapeMediaStatus{
		models.ScrapeMediaStatusScanned,
		models.ScrapeMediaStatusScrapeFailed,
		models.ScrapeMediaStatusRenameFailed,
	}).Find(&mediaFiles)

	if len(mediaFiles) == 0 {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "没有找到符合条件的待处理记录", Data: nil})
		return
	}

	// 在后台循环处理
	go func() {
		helpers.AppLogger.Infof("开始批量执行刮削，共 %d 条记录", len(mediaFiles))
		// 按路径分组处理，复用 Scraper 实例
		pathGroups := make(map[uint][]*models.ScrapeMediaFile)
		for _, mf := range mediaFiles {
			pathGroups[mf.ScrapePathId] = append(pathGroups[mf.ScrapePathId], mf)
		}

		for pathID, files := range pathGroups {
			scrapePath := models.GetScrapePathByID(pathID)
			if scrapePath == nil {
				continue
			}
			for _, file := range files {
				helpers.AppLogger.Infof("批量处理 - 开始刮削: %s", file.VideoFilename)
				scraper := scrape.NewScrape(scrapePath)
				err := scraper.ProcessSingle(file)
				if err != nil {
					helpers.AppLogger.Errorf("批量处理 - 刮削失败: %s, 错误: %v", file.VideoFilename, err)
				}
				// 稍微停顿一下，避免并发太高或网盘限速
				time.Sleep(2000 * time.Millisecond)
			}
		}
		helpers.AppLogger.Infof("批量执行刮削任务全部提交完成")
	}()

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: fmt.Sprintf("已成功触发 %d 条记录的批量刮削任务，请刷新页面查看进度", len(mediaFiles)), Data: nil})
}

// ExportMappingRecords 导出刮削映射记录（源路径 -> 目标路径）
// 专门用于备份手动矫正过的映射关系
func ExportMappingRecords(c *gin.Context) {
	scrapePathId := c.Query("scrape_path_id")
	onlyReScrape := c.Query("only_re_scrape") // 是否只导出手动矫正过的

	type mappingRecord struct {
		ID              uint   `json:"id"`
		Name            string `json:"name"`
		Year            int    `json:"year"`
		TmdbId          int64  `json:"tmdb_id"`
		SourceFullPath  string `json:"source_full_path"` // 完整源路径
		DestFullPath    string `json:"dest_full_path"`   // 完整目标路径
		VideoFilename   string `json:"video_filename"`   // 原始文件名
		NewVideoName    string `json:"new_video_name"`   // 新文件名
		Status          string `json:"status"`           // 状态
		IsReScrape      bool   `json:"is_re_scrape"`     // 是否手动矫正过
		Resolution      string `json:"resolution"`       // 分辨率
		ResolutionLevel string `json:"resolution_level"` // 分辨率等级
		CategoryName    string `json:"category_name"`    // 分类名称
	}

	var mediaFiles []*models.ScrapeMediaFile
	query := models.GetDb().Model(&models.ScrapeMediaFile{})

	// 只导出已完成重命名的记录
	query = query.Where("status = ?", models.ScrapeMediaStatusRenamed)

	if scrapePathId != "" {
		id, _ := strconv.ParseUint(scrapePathId, 10, 32)
		query = query.Where("scrape_path_id = ?", uint(id))
	}

	if onlyReScrape == "true" || onlyReScrape == "1" {
		query = query.Where("is_re_scrape = ?", true)
	}

	query.Order("created_at DESC").Find(&mediaFiles)

	if len(mediaFiles) == 0 {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "没有找到符合条件的映射记录", Data: []mappingRecord{}})
		return
	}

	records := make([]mappingRecord, 0, len(mediaFiles))
	for _, mf := range mediaFiles {
		sourceFullPath := ""
		destFullPath := ""

		if mf.MediaType == models.MediaTypeTvShow {
			sourceFullPath = filepath.Join(mf.GetRemoteFullSeasonPath(), mf.VideoFilename)
			destFullPath = filepath.Join(mf.GetDestFullSeasonPath(), mf.NewVideoBaseName+mf.VideoExt)
		} else {
			sourceFullPath = filepath.Join(mf.GetRemoteFullMoviePath(), mf.VideoFilename)
			destFullPath = filepath.Join(mf.GetDestFullMoviePath(), mf.NewVideoBaseName+mf.VideoExt)
		}

		records = append(records, mappingRecord{
			ID:              mf.ID,
			Name:            mf.Name,
			Year:            mf.Year,
			TmdbId:          mf.TmdbId,
			SourceFullPath:  sourceFullPath,
			DestFullPath:    destFullPath,
			VideoFilename:   mf.VideoFilename,
			NewVideoName:    mf.NewVideoBaseName + mf.VideoExt,
			Status:          string(mf.Status),
			IsReScrape:      mf.IsReScrape,
			Resolution:      mf.Resolution,
			ResolutionLevel: mf.ResolutionLevel,
			CategoryName:    mf.CategoryName,
		})
	}

	c.JSON(http.StatusOK, APIResponse[[]mappingRecord]{Code: Success, Message: "导出成功", Data: records})
}
