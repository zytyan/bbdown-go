package appapi

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"bbdown-go/internal/bili"
	"bbdown-go/internal/pb"

	"github.com/imroc/req/v3"
	"google.golang.org/protobuf/proto"
)

const (
	apiUGC = "https://grpc.biliapi.net/bilibili.app.playurl.v1.PlayURL/PlayView"
)

type Client struct {
	req         *req.Client
	AccessToken string
}

func NewClient(accessToken string) *Client {
	return &Client{req: req.C(), AccessToken: strings.TrimPrefix(accessToken, "access_token=")}
}

func (c *Client) PlayView(aid, cid int64, encoding string) (*bili.PlayInfo, error) {
	payload, err := makePayload(aid, cid, codecType(encoding))
	if err != nil {
		return nil, err
	}
	resp, err := c.req.R().
		SetHeaders(c.headers()).
		SetContentType("application/grpc").
		SetBody(payload).
		Post(apiUGC)
	if err != nil {
		return nil, err
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("app playurl failed: %s", resp.Status)
	}
	body, err := resp.ToBytes()
	if err != nil {
		return nil, err
	}
	msg, err := readMessage(body)
	if err != nil {
		return nil, err
	}
	var reply pb.PlayViewReply
	if err := proto.Unmarshal(msg, &reply); err != nil {
		return nil, err
	}
	return convertReply(&reply), nil
}

func makePayload(aid, cid int64, codec int32) ([]byte, error) {
	req := &pb.PlayViewReq{
		EpId: &aid, Cid: &cid, Qn: new(int64(127)), Fnval: new(int32(4048)), Download: new(uint32(0)),
		ForceHost: new(int32(2)), Fourk: new(true), Spmid: new("main.ugc-video-detail.0.0"), FromSpmid: new("main.my-history.0.0"),
		PreferCodecType: new(pb.PlayViewReq_CodeType(codec)),
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	return packMessage(data)
}

func (c *Client) headers() map[string]string {
	return map[string]string{
		"Host":                   "grpc.biliapi.net",
		"user-agent":             "Dalvik/2.1.0 (Linux; U; Android 11; M2012K11AC Build/RKQ1.200826.002) 7.32.0 os/android model/M2012K11AC mobi_app/android build/7320200 channel/xiaomi_cn_tv.danmaku.bili_zm20200902 innerVer/7320200 osVer/11 network/2 grpc-java-cronet/1.36.1",
		"te":                     "trailers",
		"x-bili-fawkes-req-bin":  mustMarshalBase64(&pb.FawkesReq{Appkey: new("android64"), Env: new("prod"), SessionId: new("dedf8669")}),
		"x-bili-metadata-bin":    mustMarshalBase64(&pb.Metadata{AccessKey: &c.AccessToken, MobiApp: new("android"), Build: new(int32(7320200)), Channel: new("xiaomi_cn_tv.danmaku.bili_zm20200902"), Buvid: new(""), Platform: new("android")}),
		"authorization":          "identify_v1 " + c.AccessToken,
		"x-bili-device-bin":      mustMarshalBase64(&pb.Device{AppId: new(int32(1)), Build: new(int32(7320200)), Buvid: new(""), MobiApp: new("android"), Platform: new("android"), Channel: new("xiaomi_cn_tv.danmaku.bili_zm20200902"), Brand: new("M2012K11AC"), Model: new("Build/RKQ1.200826.002"), Osver: new("11")}),
		"x-bili-network-bin":     mustMarshalBase64(&pb.Network{Type: new(pb.Network_WIFI), Oid: new("46007")}),
		"x-bili-restriction-bin": "",
		"x-bili-locale-bin":      mustMarshalBase64(&pb.Locale{CLocale: &pb.Locale_LocaleIds{Language: new("zh"), Region: new("CN")}}),
		"x-bili-exps-bin":        "",
		"grpc-encoding":          "gzip",
		"grpc-accept-encoding":   "identity,gzip",
		"grpc-timeout":           "17996161u",
	}
}

func convertReply(reply *pb.PlayViewReply) *bili.PlayInfo {
	out := &bili.PlayInfo{}
	if reply == nil || reply.GetVideoInfo() == nil {
		return out
	}
	info := reply.GetVideoInfo()
	out.DurationMS = info.GetTimelength()
	for _, item := range info.GetStreamList() {
		if item == nil || item.GetDashVideo() == nil || item.GetDashVideo().GetBaseUrl() == "" {
			continue
		}
		id := item.GetStreamInfo().GetQuality()
		video := item.GetDashVideo()
		out.Videos = append(out.Videos, bili.Stream{
			ID: id, Quality: qualityName(id), BaseURL: video.GetBaseUrl(), BackupURL: video.GetBackupUrl(),
			Bandwidth: video.GetBandwidth(), Codecs: codecName(video.GetCodecid()), Size: video.GetSize(),
		})
	}
	for _, item := range info.GetDashAudio() {
		addAudio(out, item, "M4A")
	}
	addAudio(out, info.GetFlac().GetAudio(), "FLAC")
	addAudio(out, info.GetDolby().GetAudio(), "E-AC-3")
	return out
}

func addAudio(out *bili.PlayInfo, item *pb.DashItem, codecs string) {
	if item == nil || item.GetBaseUrl() == "" {
		return
	}
	out.Audios = append(out.Audios, bili.Stream{
		ID: item.GetId(), Quality: fmt.Sprint(item.GetId()), BaseURL: item.GetBaseUrl(), BackupURL: item.GetBackupUrl(),
		Bandwidth: item.GetBandwidth(), Codecs: codecs, Size: item.GetSize(),
	})
}

func packMessage(input []byte) ([]byte, error) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(input); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteByte(1)
	_ = binary.Write(&out, binary.BigEndian, uint32(gz.Len()))
	out.Write(gz.Bytes())
	return out.Bytes(), nil
}

func readMessage(data []byte) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("grpc response too short")
	}
	compressed := data[0] == 1
	size := int(binary.BigEndian.Uint32(data[1:5]))
	if len(data) < 5+size {
		return nil, fmt.Errorf("grpc response truncated")
	}
	msg := data[5 : 5+size]
	if !compressed {
		return msg, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func mustMarshalBase64(m proto.Message) string {
	b, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func codecType(encoding string) int32 {
	switch strings.ToUpper(strings.ReplaceAll(encoding, "-", "")) {
	case "AVC", "H264", "CODE264":
		return 1
	case "AV1", "CODEAV1":
		return 3
	default:
		return 2
	}
}

func codecName(id uint32) string {
	switch id {
	case 7:
		return "AVC"
	case 12:
		return "HEVC"
	case 13:
		return "AV1"
	default:
		return fmt.Sprint(id)
	}
}

func qualityName(id uint32) string {
	names := map[uint32]string{6: "240P", 16: "360P", 32: "480P", 64: "720P", 74: "720P60", 80: "1080P", 112: "1080P+", 116: "1080P60", 120: "4K", 125: "HDR", 126: "Dolby", 127: "8K"}
	if v, ok := names[id]; ok {
		return v
	}
	return fmt.Sprint(id)
}
