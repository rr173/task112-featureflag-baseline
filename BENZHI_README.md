# task112-featureflag

特性开关与渐进式发布服务。为功能开关（feature flag）提供多变量、定向规则、分群（segment）、百分比灰度发布与审计能力，支持按目标上下文（用户、属性）求值、规则优先级匹配、确定性灰度哈希、分群归属判定与变更审计。所有开关、分群、规则与审计事件持久化到 SQLite，进程重启后可完整恢复。

## 主要输入与输出

- 输入：HTTP JSON 请求（`/flags`、`/segments`、`/evaluate` 等），含 `key`、`name`、`variants`、`rollout_percent`、`rules`、`target_key`、`attributes` 等字段。
- 输出：JSON 响应（开关定义、求值结果 `variant_key`/`value`/`reason`、分群列表、审计记录、统计指标等）。管理端点需要 `X-Admin-Token` 头。

## 本地命令

```bash
go build ./...       # 编译
go run . --smoke-test  # 自检（不依赖外部服务、不依赖真实时间睡眠）
go run .             # 启动 HTTP 服务（默认 :8080，SQLite 文件 featureflag.db）
go test ./...        # 测试
```

## Docker 构建

构建脚本 `build_benzhi_docker.sh` 接收两个参数：

1. 镜像名（默认 `my-project`）
2. 目标平台（默认 `linux/amd64`）

```bash
# amd64
bash ./build_benzhi_docker.sh go-task-benzhi:amd64 linux/amd64
docker run -it go-task-benzhi:amd64
# arm64
bash ./build_benzhi_docker.sh go-task-benzhi:arm64 linux/arm64
docker run -it go-task-benzhi:arm64
```

进入容器后可用 `go version` 确认工具链版本为 `go1.26.3`。

## 双架构主镜像

主 `Dockerfile` 为多阶段构建（`golang:1.26.3-bookworm` 构建 + `alpine:3.20` 运行，`CGO_ENABLED=0`）：

```bash
docker buildx build --platform linux/amd64 --load -t go-task-check:amd64 .
docker run --rm go-task-check:amd64 --smoke-test
docker buildx build --platform linux/arm64 --load -t go-task-check:arm64 .
docker run --rm go-task-check:arm64 --smoke-test
```

## 技术栈

- Go `1.26.3`（`GOTOOLCHAIN=local`）
- SQLite 引擎 `3.46.1`，纯 Go 驱动 `modernc.org/sqlite v1.52.0`（`CGO_ENABLED=0`）
- 依赖下载：`GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=sum.golang.google.cn`
