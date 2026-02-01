package main

import (
  "Q115-STRM/internal/helpers"
  "fmt"
)

func main() {
  names := []string{
    "India.Song.1975.Criterion.Collection.BluRay.1080p.DD1.0.x264.BMDru.mkv",
  }
  exts := []string{".mp4",".mkv",".avi",".mov",".wmv",".webm",".flv",".avi",".ts",".m4v",".iso",".rmvb",".strm"}
  for _, filename := range names {
    info := helpers.ExtractMediaInfoRe(filename, true, false, exts)
    fmt.Printf("%s => %+v\n", filename, info)
  }
}
