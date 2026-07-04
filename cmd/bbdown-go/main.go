package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"bbdown-go/internal/appapi"
	"bbdown-go/internal/bili"
)

type options struct {
	app         bool
	info        bool
	debug       bool
	skipMux     bool
	saveTemps   bool
	cookie      string
	accessToken string
	userAgent   string
	workDir     string
	page        int
	encoding    string
}

func main() {
	if err := run(); err != nil {
		slog.Error("执行失败", "error", err)
		os.Exit(1)
	}
}

func run() error {
	opt := parseFlags()
	setupLogger(opt.debug)
	if !opt.app {
		slog.Info("当前 Go 迁移版仅实现 APP 解析，已按默认 APP 模式继续")
	}
	if opt.workDir == "" {
		opt.workDir = "."
	}
	args := flag.Args()
	if len(args) != 1 {
		return fmt.Errorf("用法: bbdown-go [选项] <BV/av/视频 URL>")
	}

	inputID, err := bili.ParseInputID(args[0])
	if err != nil {
		return err
	}
	httpClient := bili.NewClient(opt.userAgent, opt.cookie, opt.accessToken)
	slog.Info("获取视频信息", "aid", inputID.AID)
	info, err := httpClient.FetchVideoInfo(inputID.AID)
	if err != nil {
		return err
	}
	if info.BVID == "" {
		info.BVID = inputID.BVID
	}
	if len(info.Pages) == 0 {
		return fmt.Errorf("视频没有分 P 信息")
	}
	page := pickPage(info.Pages, opt.page)
	if page == nil {
		return fmt.Errorf("找不到第 %d P", opt.page)
	}

	slog.Info("APP 解析", "cid", page.CID)
	play, err := appapi.NewClient(opt.accessToken).PlayView(info.AID, page.CID, opt.encoding)
	if err != nil {
		return err
	}

	out := jsonOutput{
		Title: info.Title, UP: info.Owner, Description: info.Desc, AID: info.AID, BVID: info.BVID,
	}
	if opt.info {
		out.Info = info
		out.Streams = play
	}

	result, err := bili.DownloadAndMux(httpClient, info, *page, play, bili.DownloadOptions{
		WorkDir: opt.workDir, BaseName: inputID.FileName, SkipMux: opt.skipMux, SaveTemps: opt.saveTemps,
	})
	if err != nil {
		return err
	}
	out.VideoPath = result.Path
	out.Pages = []pageOutput{{Index: page.Index, Title: page.Title, Path: result.Path}}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

func parseFlags() options {
	var opt options
	flag.BoolVar(&opt.app, "app", true, "使用 APP 解析模式（默认）")
	flag.BoolVar(&opt.info, "info", false, "在 JSON 输出中追加视频和流信息，并继续下载")
	flag.BoolVar(&opt.debug, "debug", false, "在 stderr 输出 debug 日志")
	flag.BoolVar(&opt.skipMux, "skip-mux", false, "只下载音视频流，不执行 ffmpeg 混流")
	flag.BoolVar(&opt.saveTemps, "save-temps", false, "保留混流前的 .video.m4s 和 .audio.m4s 临时文件")
	flag.StringVar(&opt.cookie, "cookie", "", "Bilibili Cookie")
	flag.StringVar(&opt.accessToken, "access-token", "", "Bilibili APP access token")
	flag.StringVar(&opt.userAgent, "user-agent", "", "自定义 User-Agent")
	flag.StringVar(&opt.workDir, "work-dir", ".", "工作目录")
	flag.IntVar(&opt.page, "p", 1, "分 P 序号")
	flag.StringVar(&opt.encoding, "encoding-priority", "HEVC", "优先 APP 视频编码：HEVC、AVC、AV1")
	flag.Parse()
	return opt
}

type jsonOutput struct {
	VideoPath   string          `json:"video_path,omitempty"`
	Title       string          `json:"title"`
	UP          string          `json:"up"`
	Description string          `json:"description"`
	AID         int64           `json:"aid"`
	BVID        string          `json:"bvid"`
	Info        *bili.VideoInfo `json:"info,omitempty"`
	Streams     *bili.PlayInfo  `json:"streams,omitempty"`
	Pages       []pageOutput    `json:"pages,omitempty"`
}

type pageOutput struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Path  string `json:"path,omitempty"`
}

func setupLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func pickPage(pages []bili.Page, index int) *bili.Page {
	for i := range pages {
		if pages[i].Index == index {
			return &pages[i]
		}
	}
	return nil
}
