# bbdown-go

`bbdown-go` 是一个用 Go 编写的 Bilibili 视频下载工具，目前聚焦普通 BV/av 视频的非交互式下载。

## 当前能力

- 默认使用 APP 端解析模式。
- 使用 `req/v3` 发起 HTTP 请求，便于后续调整 TLS 指纹、User-Agent 等传输行为。
- 下载视频流和音频流后使用 `ffmpeg` 混流为 MP4。
- stdout 只输出格式化 JSON，stderr 输出日志。
- `-info` 会在 JSON 中追加视频信息和可用流信息，但仍继续下载。
- 默认输出文件名使用输入中的视频 ID：
  - 输入 BV 或含 BV 的 URL：输出 `BVxxxxxxxxxx.mp4`
  - 输入 av 或含 av 的 URL：输出 `av123456.mp4`
  - 输入纯数字：按 av 处理
- 默认混流后删除 `.video.m4s` 和 `.audio.m4s` 临时文件；使用 `--save-temps` 可保留。
- 下载日志会输出每个候选下载地址的类型、host、探测大小、耗时和结果；当 base URL 下载失败时会继续尝试 backup URL。
- 下载过程中会在 stderr 使用单行进度条显示当前音视频流的下载进度、速度和 CDN host。

## 安装依赖

需要 Go 1.26 或更新版本，以及 `ffmpeg`。

Debian/Ubuntu 可执行：

```bash
sudo apt-get update
sudo apt-get install -y ffmpeg
```

## 构建

```bash
go build -o bbdown-go ./cmd/bbdown-go
```

## 基本使用

下载 BV 视频：

```bash
./bbdown-go https://www.bilibili.com/video/BV13TKS6VE4Y
```

下载 av 视频：

```bash
./bbdown-go https://www.bilibili.com/video/av116832254036435
```

指定输出目录：

```bash
./bbdown-go -work-dir ./downloads https://www.bilibili.com/video/BV13TKS6VE4Y
```

输出中追加视频信息和流信息：

```bash
./bbdown-go -info https://www.bilibili.com/video/BV13TKS6VE4Y
```

保留混流前的临时音视频文件：

```bash
./bbdown-go --save-temps https://www.bilibili.com/video/BV13TKS6VE4Y
```

只下载音视频流，不执行混流：

```bash
./bbdown-go --skip-mux https://www.bilibili.com/video/BV13TKS6VE4Y
```

## 常用参数

| 参数 | 说明 |
| --- | --- |
| `-app` | 使用 APP 解析模式，当前默认开启 |
| `-info` | 在 JSON 中追加视频信息和流信息，并继续下载 |
| `-work-dir` | 设置下载目录 |
| `-p` | 选择单个分 P，当前只支持整数 |
| `-encoding-priority` | 设置 APP 视频编码偏好：`HEVC`、`AVC`、`AV1` |
| `--save-temps` | 保留 `.video.m4s` 和 `.audio.m4s` |
| `--skip-mux` | 跳过 ffmpeg 混流 |
| `--cookie` | 设置 Bilibili Cookie |
| `--access-token` | 设置 Bilibili APP access token |
| `--user-agent` | 设置自定义 User-Agent |
| `--debug` | 输出 debug 日志 |

## 下载问题定位

下载阶段的日志会写入 stderr，例如：

```text
level=INFO msg=开始下载候选 label=video attempt=1 kind=base host=... content_length=...
level=INFO msg=下载候选成功 label=video attempt=1 kind=base host=... bytes=... elapsed=...
```

这些字段含义如下：

- `label`：当前下载的是 `video` 还是 `audio`。
- `attempt`：当前尝试第几个候选地址。
- `kind`：`base` 或 `backup-N`。
- `host`：实际下载使用的 CDN host。
- `content_length`：HEAD 探测到的大小；`-1` 表示服务端未返回或探测失败。
- `elapsed`：该候选地址下载耗时。

如果某个 CDN 失败，会记录 `下载候选失败` 并自动尝试下一个 backup URL。

下载过程中还会在 stderr 刷新单行进度条：

```text
[==============--------------] video  50.0% 10.00MiB/20.00MiB 2.50 MiB/s example.cdn
```

如果服务端没有返回总大小，百分比会显示为 `--.-%`，但仍会显示已下载大小和速度。

## JSON 输出

stdout 会输出格式化 JSON，例如：

```json
{
  "video_path": "downloads/BV13TKS6VE4Y.mp4",
  "title": "视频标题",
  "up": "UP 主",
  "description": "视频简介",
  "aid": 116832254036435,
  "bvid": "BV13TKS6VE4Y",
  "info": {
    "aid": 116857033987530,
    "bvid": "BV1FCTt6AEhV",
    "title": "【全面评测】你想知道的 Steam Machine 的一切！体验+性能+拆机！",
    "desc": "",
    "pic": "http://i1.hdslb.com/bfs/archive/51bb611d87f51c1ca0350319f4c75a79bcede670.jpg",
    "pubtime": 1783097108,
    "owner_mid": 27899754,
    "owner": "贪玩歌姬小宁子",
    "pages": [
      {
        "index": 1,
        "aid": 116857033987530,
        "bvid": "BV1FCTt6AEhV",
        "cid": 39632831285,
        "title": "steam machine-B-07040217-无美颜有字幕",
        "duration": 930,
        "dimension": "3840x2160"
      }
    ]
  }
}
```

使用 `-info` 时会额外包含 `info` 和 `streams` 字段。

## Protobuf

APP 接口使用的 protobuf 源文件位于 `proto/app`，生成后的 Go 类型位于 `internal/pb`。

重新生成 protobuf 代码需要安装 `protoc` 和 `protoc-gen-go`，然后执行：

```bash
protoc --proto_path=. --go_out=. --go_opt=module=bbdown-go $(find proto/app -name '*.proto' | sort)
```

## 当前限制

当前版本不是 C# 版 BBDown 的完整功能迁移。以下能力尚未实现：

- 登录命令。
- server 模式。
- WEB/TV/intl 解析模式。
- 番剧、课程、收藏夹、空间、合集、列表等批量下载。
- 字幕、弹幕、封面独立下载。
- 交互式选择。
- MP4Box 和 aria2。
- 完整文件名模板、清晰度优先级、多线程下载、配置文件兼容等高级功能。
