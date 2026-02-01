package controllers

import (
	"Q115-STRM/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

type updateScrapePathFiltersReq struct {
	ID                  uint     `json:"id"`
	ExcludeDirKeywords  []string `json:"exclude_dir_keywords"`
	ExcludeFileKeywords []string `json:"exclude_file_keywords"`
	DeleteKeyword       []string `json:"delete_keyword"`
}

func UpdateScrapePathFilters(c *gin.Context) {
	var req updateScrapePathFiltersReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 不能为空", Data: nil})
		return
	}
	scrapePath := models.GetScrapePathByID(req.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	scrapePath.ExcludeDirKeywords = req.ExcludeDirKeywords
	scrapePath.ExcludeFileKeywords = req.ExcludeFileKeywords
	scrapePath.DeleteKeyword = req.DeleteKeyword
	if err := scrapePath.Save(); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "保存失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存成功", Data: nil})
}
