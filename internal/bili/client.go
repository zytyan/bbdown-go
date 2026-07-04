package bili

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
)

type Client struct {
	req         *req.Client
	downloadReq *req.Client
	UserAgent   string
	Cookie      string
	AccessToken string
}

func NewClient(userAgent, cookie, accessToken string) *Client {
	if userAgent == "" {
		userAgent = randomUserAgent()
	}
	c := req.C().
		SetTimeout(2*time.Minute).
		SetCommonHeader("User-Agent", userAgent).
		SetCommonHeader("Cache-Control", "no-cache")
	downloadClient := req.C().
		SetTimeout(0).
		SetCommonHeader("User-Agent", userAgent).
		SetCommonHeader("Cache-Control", "no-cache")
	return &Client{req: c, downloadReq: downloadClient, UserAgent: userAgent, Cookie: cookie, AccessToken: strings.TrimPrefix(accessToken, "access_token=")}
}

func (c *Client) FetchVideoInfo(aid int64) (*VideoInfo, error) {
	url := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?aid=%d", aid)
	resp, err := c.req.R().
		SetHeader("Referer", "https://www.bilibili.com/").
		SetHeader("Cookie", c.Cookie).
		Get(url)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("view api failed: %s", resp.Status)
	}
	body, err := resp.ToBytes()
	if err != nil {
		return nil, err
	}
	body, err = maybeGunzip(body)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("view api code %d: %s", envelope.Code, envelope.Message)
	}
	var data struct {
		AID     int64  `json:"aid"`
		BVID    string `json:"bvid"`
		Title   string `json:"title"`
		Desc    string `json:"desc"`
		Pic     string `json:"pic"`
		PubTime int64  `json:"pubdate"`
		Owner   struct {
			MID  int64  `json:"mid"`
			Name string `json:"name"`
		} `json:"owner"`
		Pages []struct {
			Page      int    `json:"page"`
			CID       int64  `json:"cid"`
			Part      string `json:"part"`
			Duration  int    `json:"duration"`
			Dimension struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"dimension"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, err
	}
	info := &VideoInfo{
		AID: data.AID, BVID: data.BVID, Title: strings.TrimSpace(data.Title), Desc: strings.TrimSpace(data.Desc),
		Pic: data.Pic, PubTime: data.PubTime, OwnerMID: data.Owner.MID, Owner: data.Owner.Name,
	}
	for _, p := range data.Pages {
		info.Pages = append(info.Pages, Page{
			Index: p.Page, AID: data.AID, BVID: data.BVID, CID: p.CID,
			Title: strings.TrimSpace(p.Part), Duration: p.Duration,
			Dimension: fmt.Sprintf("%dx%d", p.Dimension.Width, p.Dimension.Height),
		})
	}
	return info, nil
}

func maybeGunzip(body []byte) ([]byte, error) {
	if len(body) < 2 || body[0] != 0x1f || body[1] != 0x8b {
		return body, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

type DownloadCandidate struct {
	URL  string
	Kind string
}

func (c *Client) DownloadFile(candidates []DownloadCandidate, path, referer, label string) error {
	var errs []string
	for i, candidate := range candidates {
		if candidate.URL == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		host := urlHost(candidate.URL)
		size := c.probeDownloadSize(candidate.URL, referer)
		start := time.Now()
		slog.Info("开始下载候选", "label", label, "attempt", i+1, "kind", candidate.Kind, "host", host, "content_length", size, "path", path)
		err := c.downloadOne(candidate.URL, path, referer, newProgressBar(label, host, size))
		elapsed := time.Since(start)
		if err == nil {
			stat, statErr := os.Stat(path)
			if statErr != nil {
				return statErr
			}
			slog.Info("下载候选成功", "label", label, "attempt", i+1, "kind", candidate.Kind, "host", host, "bytes", stat.Size(), "elapsed", elapsed.String())
			return nil
		}
		errs = append(errs, fmt.Sprintf("%s %s: %v", candidate.Kind, host, err))
		slog.Warn("下载候选失败", "label", label, "attempt", i+1, "kind", candidate.Kind, "host", host, "elapsed", elapsed.String(), "error", err)
	}
	return fmt.Errorf("所有下载候选均失败: %s", strings.Join(errs, "; "))
}

func (c *Client) downloadOne(rawURL, path, referer string, progress *progressBar) error {
	resp, err := c.downloadReq.R().
		SetHeader("Referer", referer).
		SetHeader("Cookie", c.Cookie).
		SetHeader("Range", "bytes=0-").
		SetOutputFile(path).
		SetDownloadCallbackWithInterval(progress.Callback, 200*time.Millisecond).
		Get(rawURL)
	progress.Finish(err)
	if err != nil {
		return err
	}
	if !resp.IsSuccessState() {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	return nil
}

type progressBar struct {
	label      string
	host       string
	total      int64
	start      time.Time
	lastBytes  int64
	lastRender time.Time
	mu         sync.Mutex
}

func newProgressBar(label, host string, total int64) *progressBar {
	return &progressBar{label: label, host: host, total: total, start: time.Now()}
}

func (p *progressBar) Callback(info req.DownloadInfo) {
	if p.total <= 0 && info.Response != nil && info.Response.ContentLength > 0 {
		p.mu.Lock()
		if p.total <= 0 {
			p.total = info.Response.ContentLength
		}
		p.mu.Unlock()
	}
	p.render(info.DownloadedSize, false)
}

func (p *progressBar) Finish(err error) {
	p.mu.Lock()
	downloaded := p.lastBytes
	p.mu.Unlock()
	p.render(downloaded, true)
	if err != nil {
		fmt.Fprintln(os.Stderr)
	}
}

func (p *progressBar) render(downloaded int64, done bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if !done && !p.lastRender.IsZero() && now.Sub(p.lastRender) < 200*time.Millisecond {
		return
	}
	if downloaded > p.lastBytes {
		p.lastBytes = downloaded
	}
	p.lastRender = now
	width := 28
	percentText := "--.-%"
	filled := 0
	if p.total > 0 {
		percent := float64(p.lastBytes) / float64(p.total)
		if percent > 1 {
			percent = 1
		}
		filled = int(percent * float64(width))
		percentText = fmt.Sprintf("%5.1f%%", percent*100)
	} else if done {
		filled = width
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("=", filled) + strings.Repeat("-", width-filled)
	speed := float64(p.lastBytes) / 1024 / 1024 / max(time.Since(p.start).Seconds(), 0.001)
	line := fmt.Sprintf("\r[%s] %s %s %s/%s %.2f MiB/s %s",
		bar, p.label, percentText, formatBytes(p.lastBytes), formatBytes(p.total), speed, p.host)
	if done {
		line += "\n"
	}
	fmt.Fprint(os.Stderr, line)
}

func formatBytes(n int64) string {
	if n < 0 {
		return "?"
	}
	units := []string{"B", "KiB", "MiB", "GiB"}
	v := float64(n)
	unit := units[0]
	for i := 1; i < len(units) && v >= 1024; i++ {
		v /= 1024
		unit = units[i]
	}
	if unit == "B" {
		return fmt.Sprintf("%d%s", n, unit)
	}
	return fmt.Sprintf("%.2f%s", v, unit)
}

func (c *Client) probeDownloadSize(rawURL, referer string) int64 {
	resp, err := c.downloadReq.R().
		SetHeader("Referer", referer).
		SetHeader("Cookie", c.Cookie).
		Head(rawURL)
	if err != nil {
		slog.Debug("探测下载大小失败", "host", urlHost(rawURL), "error", err)
		return -1
	}
	if !resp.IsSuccessState() {
		slog.Debug("探测下载大小返回非成功状态", "host", urlHost(rawURL), "status", resp.Status)
		return -1
	}
	return resp.ContentLength
}

func urlHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func randomUserAgent() string {
	platforms := []string{"Windows NT 10.0; Win64", "Macintosh; Intel Mac OS X 10_15", "X11; Linux x86_64"}
	browsers := []string{"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%0.3f Safari/537.36", "Gecko/20100101 Firefox/%0.3f"}
	return fmt.Sprintf("Mozilla/5.0 (%s) "+browsers[rand.Intn(len(browsers))], platforms[rand.Intn(len(platforms))], 80+rand.Float64()*30)
}
