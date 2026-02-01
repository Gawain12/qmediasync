package helpers

import (
	"encoding/json"
	"os/exec"
)

type MediainfoTrack struct {
	Type         string `json:"@type"`
	Width        string `json:"Width"`
	Height       string `json:"Height"`
	CodecID      string `json:"CodecID"`
	Format       string `json:"Format"`
	Duration     string `json:"Duration"`
	FrameRate    string `json:"FrameRate"`
	DisplayAspectRatio string `json:"DisplayAspectRatio"`
	Channels     string `json:"Channels"`
	SamplingRate string `json:"SamplingRate"`
	Language     string `json:"Language"`
	Title        string `json:"Title"`
	StreamSize   string `json:"StreamSize"`
	BitRate      string `json:"BitRate"`
}

type MediainfoMedia struct {
	Track []MediainfoTrack `json:"track"`
}

type MediainfoJson struct {
	Media MediainfoMedia `json:"media"`
}

// GetMediainfoJson 使用 mediainfo 提取媒体信息
func GetMediainfoJson(videoUrl string) (*MediainfoJson, error) {
	AppLogger.Infof("mediainfo查询url %s 的信息", videoUrl)
	cmd := exec.Command("mediainfo", "--Output=JSON", videoUrl)
	output, err := cmd.CombinedOutput()
	if err != nil {
		AppLogger.Errorf("执行mediainfo命令失败:%v", err)
		return nil, err
	}
	var mediainfoJson MediainfoJson
	if err := json.Unmarshal(output, &mediainfoJson); err != nil {
		AppLogger.Errorf("解析mediainfo输出失败:%v", err)
		return nil, err
	}
	return &mediainfoJson, nil
}
