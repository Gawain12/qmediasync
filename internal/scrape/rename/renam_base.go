package rename

import (
	"Q115-STRM/internal/models"
	"context"
)

type RenameBase struct {
	scrapePath *models.ScrapePath
	ctx        context.Context
}

func (r *RenameBase) isRootPath(idOrPath string) bool {
	if r == nil || r.scrapePath == nil || idOrPath == "" {
		return false
	}
	if idOrPath == r.scrapePath.SourcePathId || idOrPath == r.scrapePath.SourcePath {
		return true
	}
	if idOrPath == r.scrapePath.DestPathId || idOrPath == r.scrapePath.DestPath {
		return true
	}
	return false
}
