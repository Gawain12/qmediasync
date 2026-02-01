package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type tmdbCandidate struct {
	TmdbId       int64   `json:"tmdb_id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Year         int     `json:"year"`
	Overview     string  `json:"overview"`
	Poster       string  `json:"poster"`
	Backdrop     string  `json:"backdrop"`
	TmdbUrl      string  `json:"tmdb_url"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int64   `json:"vote_count"`
	Popularity   float64 `json:"popularity"`
}

type scrapeRecordResp struct {
	ID            uint   `json:"id"`
	MediaType     string `json:"media_type"`
	Name          string `json:"name"`
	Year          int    `json:"year"`
	TmdbId        int64  `json:"tmdb_id"`
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	VideoFilename string `json:"video_filename"`
	Path          string `json:"path"`
	FailedReason  string `json:"failed_reason"`
}

func GetScrapeRecord(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "缺少参数 id", Data: nil})
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "无效的 id", Data: nil})
		return
	}
	record := models.GetScrapeMediaFileById(uint(id))
	if record == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未找到刮削记录", Data: nil})
		return
	}
	resp := scrapeRecordResp{
		ID:            record.ID,
		MediaType:     string(record.MediaType),
		Name:          record.Name,
		Year:          record.Year,
		TmdbId:        record.TmdbId,
		SeasonNumber:  record.SeasonNumber,
		EpisodeNumber: record.EpisodeNumber,
		VideoFilename: record.VideoFilename,
		Path:          record.Path,
		FailedReason:  record.FailedReason,
	}
	c.JSON(http.StatusOK, APIResponse[scrapeRecordResp]{Code: Success, Message: "", Data: resp})
}

func SearchTmdbCandidates(c *gin.Context) {
	idStr := c.Query("id")
	name := strings.TrimSpace(c.Query("name"))
	yearStr := strings.TrimSpace(c.Query("year"))
	mediaType := strings.TrimSpace(c.Query("media_type"))
	tmdbIdStr := strings.TrimSpace(c.Query("tmdb_id"))
	if tmdbIdStr == "" {
		tmdbIdStr = strings.TrimSpace(c.Query("tmdbid"))
	}

	var record *models.ScrapeMediaFile
	var scrapePath *models.ScrapePath
	var tmdbId int64
	if tmdbIdStr != "" {
		if parsed, err := strconv.ParseInt(tmdbIdStr, 10, 64); err == nil && parsed > 0 {
			tmdbId = parsed
		}
	}
	if idStr != "" {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "无效的 id", Data: nil})
			return
		}
		record = models.GetScrapeMediaFileById(uint(id))
		if record == nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "未找到刮削记录", Data: nil})
			return
		}
		scrapePath = models.GetScrapePathByID(record.ScrapePathId)
		if name == "" {
			name = record.Name
		}
		if yearStr == "" && record.Year > 0 {
			yearStr = fmt.Sprintf("%d", record.Year)
		}
		if mediaType == "" {
			mediaType = string(record.MediaType)
		}
		if tmdbId == 0 && record.TmdbId > 0 {
			tmdbId = record.TmdbId
		}
	}

	if name == "" && record != nil {
		// 尝试从文件名中提取名称与年份
		videoExtList := []string{}
		deleteKeywords := []string{}
		if scrapePath != nil {
			videoExtList = scrapePath.VideoExtList
			deleteKeywords = scrapePath.DeleteKeyword
		}
		isMovie := record.MediaType == models.MediaTypeMovie
		info := helpers.ExtractMediaInfoRe(record.VideoFilename, isMovie, false, videoExtList, deleteKeywords...)
		if info != nil && info.Name != "" {
			name = helpers.CleanFileName(info.Name)
			if yearStr == "" && info.Year > 0 {
				yearStr = fmt.Sprintf("%d", info.Year)
			}
		}
	}

	if name == "" && tmdbId == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "缺少参数 name", Data: nil})
		return
	}
	if mediaType == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "缺少参数 media_type", Data: nil})
		return
	}
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	client := models.GlobalScrapeSettings.GetTmdbClient()
	language := models.GlobalScrapeSettings.GetTmdbLanguage()
	imageBase := strings.TrimRight(models.GlobalScrapeSettings.GetTmdbImageUrl(), "/")
	webBase := "https://www.themoviedb.org"
	buildImage := func(size string, path string) string {
		if path == "" {
			return ""
		}
		return fmt.Sprintf("%s/t/p/%s%s", imageBase, size, path)
	}

	candidates := make([]tmdbCandidate, 0)
	seen := make(map[int64]struct{})
	appendCandidate := func(item tmdbCandidate) {
		if item.TmdbId <= 0 {
			return
		}
		if _, ok := seen[item.TmdbId]; ok {
			return
		}
		seen[item.TmdbId] = struct{}{}
		candidates = append(candidates, item)
	}

	if tmdbId > 0 {
		switch models.MediaType(mediaType) {
		case models.MediaTypeTvShow:
			tvDetail, err := client.GetTvDetail(tmdbId, language)
			if err == nil && tvDetail != nil {
				appendCandidate(tmdbCandidate{
					TmdbId:       tvDetail.ID,
					Name:         tvDetail.Name,
					OriginalName: tvDetail.OriginalName,
					Year:         helpers.ParseYearFromDate(tvDetail.FirstAirDate),
					Overview:     tvDetail.Overview,
					Poster:       buildImage("w342", tvDetail.PosterPath),
					Backdrop:     buildImage("w780", tvDetail.BackdropPath),
					TmdbUrl:      fmt.Sprintf("%s/tv/%d", webBase, tvDetail.ID),
					VoteAverage:  tvDetail.VoteAverage,
					VoteCount:    tvDetail.VoteCount,
					Popularity:   tvDetail.Popularity,
				})
			} else {
				fallbackName := name
				fallbackYear := year
				if record != nil {
					if record.Name != "" {
						fallbackName = record.Name
					}
					if record.Year > 0 {
						fallbackYear = record.Year
					}
				}
				appendCandidate(tmdbCandidate{
					TmdbId:  tmdbId,
					Name:    fallbackName,
					Year:    fallbackYear,
					TmdbUrl: fmt.Sprintf("%s/tv/%d", webBase, tmdbId),
				})
			}
		case models.MediaTypeMovie:
			movieDetail, err := client.GetMovieDetail(tmdbId, language)
			if err == nil && movieDetail != nil {
				appendCandidate(tmdbCandidate{
					TmdbId:       movieDetail.ID,
					Name:         movieDetail.Title,
					OriginalName: movieDetail.OriginalTitle,
					Year:         helpers.ParseYearFromDate(movieDetail.ReleaseDate),
					Overview:     movieDetail.Overview,
					Poster:       buildImage("w342", movieDetail.PosterPath),
					Backdrop:     buildImage("w780", movieDetail.BackdropPath),
					TmdbUrl:      fmt.Sprintf("%s/movie/%d", webBase, movieDetail.ID),
					VoteAverage:  movieDetail.VoteAverage,
					VoteCount:    movieDetail.VoteCount,
					Popularity:   movieDetail.Popularity,
				})
			} else {
				fallbackName := name
				fallbackYear := year
				if record != nil {
					if record.Name != "" {
						fallbackName = record.Name
					}
					if record.Year > 0 {
						fallbackYear = record.Year
					}
				}
				appendCandidate(tmdbCandidate{
					TmdbId:  tmdbId,
					Name:    fallbackName,
					Year:    fallbackYear,
					TmdbUrl: fmt.Sprintf("%s/movie/%d", webBase, tmdbId),
				})
			}
		}
	}

	if name != "" {
		switch models.MediaType(mediaType) {
		case models.MediaTypeTvShow:
			resp, err := client.SearchTv(name, year, language, true)
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "TMDB 查询失败: " + err.Error(), Data: nil})
				return
			}
			if resp != nil {
				for _, item := range resp.Results {
					appendCandidate(tmdbCandidate{
						TmdbId:       item.ID,
						Name:         item.Name,
						OriginalName: item.OriginalName,
						Year:         helpers.ParseYearFromDate(item.FirstAirDate),
						Overview:     item.Overview,
						Poster:       buildImage("w185", item.PosterPath),
						Backdrop:     buildImage("w780", item.BackdropPath),
						TmdbUrl:      fmt.Sprintf("%s/tv/%d", webBase, item.ID),
						VoteAverage:  item.VoteAverage,
						VoteCount:    item.VoteCount,
						Popularity:   item.Popularity,
					})
				}
			}
		case models.MediaTypeMovie:
			resp, err := client.SearchMovie(name, year, language, true, false)
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "TMDB 查询失败: " + err.Error(), Data: nil})
				return
			}
			if resp != nil {
				for _, item := range resp.Results {
					appendCandidate(tmdbCandidate{
						TmdbId:       item.ID,
						Name:         item.Title,
						OriginalName: item.OriginalTitle,
						Year:         helpers.ParseYearFromDate(item.ReleaseDate),
						Overview:     item.Overview,
						Poster:       buildImage("w185", item.PosterPath),
						Backdrop:     buildImage("w780", item.BackdropPath),
						TmdbUrl:      fmt.Sprintf("%s/movie/%d", webBase, item.ID),
						VoteAverage:  item.VoteAverage,
						VoteCount:    item.VoteCount,
						Popularity:   item.Popularity,
					})
				}
			}
		default:
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "media_type 仅支持 movie/tvshow", Data: nil})
			return
		}
	}

	payload := gin.H{
		"query": gin.H{
			"id":         idStr,
			"name":       name,
			"year":       year,
			"media_type": mediaType,
		},
		"items": candidates,
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "", Data: payload})
}
