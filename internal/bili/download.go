package bili

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type DownloadResult struct {
	Path string
}

type DownloadOptions struct {
	WorkDir   string
	BaseName  string
	SkipMux   bool
	SaveTemps bool
}

func DownloadAndMux(client *Client, info *VideoInfo, page Page, play *PlayInfo, opt DownloadOptions) (*DownloadResult, error) {
	if len(play.Videos) == 0 {
		return nil, fmt.Errorf("没有可下载的视频流")
	}
	if len(play.Audios) == 0 {
		return nil, fmt.Errorf("没有可下载的音频流")
	}
	sort.SliceStable(play.Videos, func(i, j int) bool { return play.Videos[i].ID > play.Videos[j].ID })
	sort.SliceStable(play.Audios, func(i, j int) bool { return play.Audios[i].Bandwidth > play.Audios[j].Bandwidth })
	video := play.Videos[0]
	audio := play.Audios[0]

	base := sanitize(opt.BaseName)
	if base == "" {
		base = sanitize(info.BVID)
	}
	if len(info.Pages) > 1 {
		base = filepath.Join(base, fmt.Sprintf("[P%02d]%s", page.Index, sanitize(page.Title)))
	}
	videoPath := filepath.Join(opt.WorkDir, base+".video.m4s")
	audioPath := filepath.Join(opt.WorkDir, base+".audio.m4s")
	outputPath := filepath.Join(opt.WorkDir, base+".mp4")
	if err := os.MkdirAll(filepath.Dir(videoPath), 0o755); err != nil {
		return nil, err
	}
	referer := "https://www.bilibili.com/video/" + info.BVID
	slog.Info("下载视频流", "quality", video.Quality, "codec", video.Codecs)
	if err := client.DownloadFile(streamCandidates(video), videoPath, referer, "video"); err != nil {
		return nil, err
	}
	slog.Info("下载音频流", "codec", audio.Codecs)
	if err := client.DownloadFile(streamCandidates(audio), audioPath, referer, "audio"); err != nil {
		return nil, err
	}
	if opt.SkipMux {
		return &DownloadResult{Path: videoPath}, nil
	}
	slog.Info("ffmpeg 混流", "path", outputPath)
	if err := runFFmpeg(videoPath, audioPath, outputPath); err != nil {
		return nil, err
	}
	if !opt.SaveTemps {
		removeTemp(videoPath)
		removeTemp(audioPath)
	}
	return &DownloadResult{Path: outputPath}, nil
}

func runFFmpeg(videoPath, audioPath, outputPath string) error {
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", videoPath, "-i", audioPath, "-c", "copy", outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeTemp(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("删除临时文件失败", "path", path, "error", err)
	}
}

func streamCandidates(stream Stream) []DownloadCandidate {
	candidates := []DownloadCandidate{{URL: stream.BaseURL, Kind: "base"}}
	for i, backup := range stream.BackupURL {
		candidates = append(candidates, DownloadCandidate{URL: backup, Kind: fmt.Sprintf("backup-%d", i+1)})
	}
	return candidates
}

var invalidPathChars = regexp.MustCompile(`[\\/:*?"<>|]+`)

func sanitize(s string) string {
	s = strings.TrimSpace(invalidPathChars.ReplaceAllString(s, "_"))
	return s
}
