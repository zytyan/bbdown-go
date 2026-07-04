# BBDown Go 迁移说明

## 目标

将 BBDown C# 应用迁移到 Go。当前阶段聚焦非交互式下载和机器可读输出。

## 优先需求

- `-app` 已实现，并作为默认解析模式。
- HTTP 请求统一通过 `github.com/imroc/req/v3`，便于后续集中替换 TLS 指纹、User-Agent 或传输层配置。
- `-info` 是追加信息模式：会获取并输出信息，但不会单独中断下载流程。
- 日志写入 stderr，stdout 只输出格式化 JSON，至少包含视频路径、标题、UP 主和简介。
- 默认输出文件名使用用户输入中的视频 ID：输入 BV/含 BV 的 URL 时使用 BV，输入 av/含 av 的 URL 或纯数字时使用 av。
- 默认不保留混流前的 `.video.m4s` 和 `.audio.m4s`，需要保留时使用 `--save-temps`。
- 下载阶段会记录候选 URL 的 host、类型、探测大小、耗时和结果，并在 base URL 失败时尝试 backup URL。
- 下载过程中在 stderr 刷新单行进度条；stdout 仍保持 JSON-only。

## 明确不实现

- 不使用 MP4Box 混流。
- 不集成 aria2 下载。
- 不实现交互式选择功能。

## 当前实现状态

- Go 模块位于仓库根目录，入口为 `cmd/bbdown-go`。
- 目录已收敛为：
  - `internal/appapi`：APP playurl 请求、gRPC framing、响应转换。
  - `internal/bili`：BV/av 转换、视频信息获取、下载和 ffmpeg 混流。
  - `proto/app`：APP 接口 protobuf 源文件。
  - `internal/pb`：由 `proto/app` 生成的新版 protobuf 类型。
- protobuf 使用 `google.golang.org/protobuf/proto`，不再依赖已弃用的 `github.com/golang/protobuf/proto`。
- 原 C# 项目目录 `BBDown/` 已删除，当前仓库只保留 Go 实现。
- 日志使用标准库 `log/slog`，默认写入 stderr。
- 已安装运行时混流依赖 `ffmpeg`。
- 已验证 `go test ./...` 和 `go build ./cmd/bbdown-go`。
- 目标视频 `https://www.bilibili.com/video/BV13TKS6VE4Y` 已验证通过：`-info` 可获取信息，APP 解析可返回流，直接下载成功，ffmpeg 可产出包含 HEVC 视频和 AAC 音频的 `BV13TKS6VE4Y.mp4`，默认会清理中间 `.m4s`，stderr/stdout 分流后 stdout 是合法格式化 JSON。
- 慢下载问题定位：`https://www.bilibili.com/video/BV1FCTt6AEhV` 的视频流在一次验证中耗时 `2m11s`，旧实现会撞上 req/http client 的 `2m` 总超时；当前下载专用 req client 已取消总超时，并保留 API 请求的 2 分钟超时。

## 尚未迁移

- 登录命令。
- server 模式。
- 番剧、课程、收藏夹、空间、合集、列表等批量 fetcher。
- 字幕、弹幕、封面独立下载等周边流程。
- 配置文件兼容和完整 C# 参数兼容。

## 开发约束

- stdout 必须保持 JSON-only；所有进度、诊断和错误都走 stderr。
- 新增网络请求应继续集中在 req 客户端相关代码里。
- APP 模式保持默认；未来 WEB/TV/intl 解析应作为显式可选模式加入。
- 优先做小而完整的行为切片，不做大范围机械翻译。
- 功能迁移状态变化后同步更新本文档。
