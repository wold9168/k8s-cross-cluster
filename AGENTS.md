# AGENTS.md

## 仓库结构
- 这是一个多模块仓库，不是单一 Go workspace。Go 模块分别在 `lib/k8sclient`、`lib/tailscaled-client`、`sidecar/caddy-config-manager`、`sidecar/coredns-config-manager`；`tailscale-manifest/lite-mode` 是独立的 Python/`uv` 项目。
- 仓库里没有 `go.work`。跑 Go 测试时要进入对应模块目录，不要在仓库根目录想当然地执行 `go test ./...`。
- 根目录的 `make test` 只会跑 `lib/k8sclient`、`sidecar/caddy-config-manager` 和 `sidecar/coredns-config-manager`；不会覆盖 `lib/tailscaled-client`。

## 已验证命令
- 根目录回归测试：`make test`
- 单个 Go 模块：在对应模块目录执行 `go test ./...`
- `lib/k8sclient` 在根目录 `Makefile` 里实际执行的是 `go test -v .`，不是 `./...`。
- Python 安装器开发：进入 `tailscale-manifest/lite-mode` 后使用 `make setup-dev`、`make test`、`make lint`、`make format`
- Sidecar 镜像构建：`make sidecar-image-build`、`make caddy-config-manager-image-build`、`make coredns-config-manager-image-build`
- `sidecar/caddy-config-manager` 模块内可用 `make test` 和 `make binary-build`；`binary-build` 会产出 Linux 二进制 `caddy-config-manager`。

## Makefile 约定
- 现有三个 Makefile 的默认目标都是 `help`；直接执行 `make` 只会显示帮助，不会自动测试或构建。
- 根目录镜像构建默认使用 `TAG=$(git rev-parse HEAD)`、`REGISTRY=ghcr.io`；`REPO_NAME` 优先取 git remote 里的 GitHub 仓库名，取不到时回退为当前用户名。
- `tailscale-manifest/lite-mode` 的安装参数通过 `make ARGS="..." install` 透传给 `uv run tailscale-install`。
- `tailscale-manifest/lite-mode` 的卸载目标要求显式提供上下文：要么用 `make CONTEXT=... uninstall`，要么在 `ARGS` 中带 `--context ...`。

## 构建与 CI 注意点
- Docker 构建上下文必须用仓库根目录。两个 sidecar 的 Dockerfile 都用了 `COPY ./../..`，如果在模块目录单独 `docker build` 会失败。
- CI 在 `.github/workflows/docker-build.yml` 中调用根目录 `Makefile`；如果改了镜像名、tag 或构建目标，要同时更新这两处。

## 运行时连接关系
- `sidecar/caddy-config-manager` 会写入 `/config/Caddyfile`，通过 `http://localhost:2019/reload` 触发 Caddy 重载，对外 API 监听 `:8091`，metrics 监听 `:8090`。
- `sidecar/coredns-config-manager` 提供 UDP DNS `:10053`、metrics `:8080`、服务发现 API `:8081`，并更新 `kube-system/coredns` 这个 CoreDNS ConfigMap。
- 跨集群发现是 sidecar 之间互通：`coredns-config-manager` 通过 SOCKS5 `127.0.0.1:1055` 访问远端 `/svc`，`caddy-config-manager` 基于本地 Service 生成 `<service>.<namespace>.svc.<cluster>.remote` 和 `.clusterset.remote` 域名映射。

## 生成文件
- `sidecar/caddy-config-manager/docs/docs.go` 和 `sidecar/coredns-config-manager/docs/docs.go` 由 `swaggo/swag` 生成，不要手改。

## Metrics 系统重构计划

### 现状问题（按优先级）
| # | 严重度 | 问题 | 影响范围 |
|---|--------|------|----------|
| 1 | 🔴高 | `GetAllMetrics()` 中 `gc_cycles` 始终为 0 — coredns 用 `debug.GCStats{}.NumGC`（零值结构体），JSON 输出永久错误；caddy 的 `getGCCycles()` 正确用了 `memStats.NumGC` | coredns |
| 2 | 🔴高 | `go_threads` 指标语义错误 — 两个 sidecar 都用 `runtime.NumCgoCall()` 采集线程数，该函数返回 cgo 调用累计次数，不是 OS 线程数 | 两者 |
| 3 | 🔴高 | 共享指标命名跨组件不一致 — `go_gc_cycles` vs `go_gc_cycles_total`，`cpu_context_switches` vs `cpu_context_switches_total`，`disk_iops` vs `disk_iops_total`，导致 dashboard/告警规则无法复用 | 两者 |
| 4 | 🟡中 | `caddy_config_update_total` 是 Gauge 而非 Counter — 语义为单调递增的累计更新数，应用 Counter | caddy |
| 5 | 🟡中 | coredns 计算了 service/cluster 数量但未暴露为 metrics — `Sync()` 中 `serviceCount`/`clusterCount` 仅写入日志 | coredns |
| 6 | 🟡中 | caddy `Manager.Start()` 多余加锁 — `StartServer` 仅读取初始化后不可变的 `collector`，coredns 不加锁，行为不一致 | caddy |
| 7 | 🟡中 | 两个 collector.go 文件 ~95% 代码重复（约 600 行通用收集逻辑），`loadAvgFromHost()` 完全相同，`prevCPUTimes` 字段两边都声明但从未使用 | 两者 |
| 8 | 🟡中 | 两个 go.mod 的 `prometheus/client_golang` 版本不同（coredns v1.20.5，caddy v1.23.2） | 两者 |
| 9 | 🟢低 | `/metrics` 端点无请求超时保护 | 两者 |
| 10 | 🟢低 | `cpu_context_switches` 本质是累计值却设为 Gauge（边界模糊，可接受但非最佳实践） | 两者 |

### 目标架构（重构后）
```
lib/metrics/                           ← 新建共享包
├── collector.go                       ← 通用指标收集器（Go runtime、CPU、memory、disk、network），实现 prometheus.Collector
├── handler.go                         ← 通用 HTTP handler + StartServer()，支持 Prometheus 和 JSON 两种格式
├── manager.go                         ← 通用 Manager 单例
└── helpers.go                         ← loadAvgFromHost() 等工具函数

sidecar/coredns-config-manager/metrics/  ← 仅保留业务相关代码
├── collector.go                       ← 嵌入 lib/metrics.Collector，添加 dns_record_count、dns_service_count、dns_cluster_count
└── manager.go                         ← Manager 适配层，暴露 UpdateDNSRecordCount / UpdateServiceClusterCount

sidecar/caddy-config-manager/metrics/   ← 仅保留业务相关代码
├── collector.go                       ← 嵌入 lib/metrics.Collector，添加 caddy_config_update_total（Counter）、caddy_last_config_update_timestamp（Gauge）、caddy_service_count（Gauge）
└── manager.go                         ← Manager 适配层，暴露 UpdateConfigUpdate / UpdateServiceCount
```

### 实施步骤（按顺序，每步独立可验证）
1. **创建 `lib/metrics` 共享包** — 提取通用 Collector（含所有 runtime/CPU/memory/disk/network 指标）、Handler、Manager、`loadAvgFromHost()`，统一指标命名（统一采用 `_total` 后缀版本）。修复 `go_threads` 语义（改用 `process.NumThreads()` 或 `runtime.GOMAXPROCS` + 文档说明 Go 无直接线程数 API）。
2. **更新依赖版本** — 将两个 go.mod 中 `prometheus/client_golang` 统一到最新兼容版本（v1.23.2），`go mod tidy`。
3. **重构 coredns metrics** — 删除原有 collector/handler/manager 中的通用部分，改为引用 `lib/metrics`，仅保留 `dns_record_count` 及新增 `dns_service_count`、`dns_cluster_count`。
4. **重构 caddy metrics** — 删除原有通用部分，改为引用 `lib/metrics`，将 `caddy_config_update_total` 从 Gauge 改为 Counter，删除 `Manager.Start()` 中多余锁。
5. **更新调用方** — 修改 `dns_config_manager.go` 的 `Sync()` 方法传入 `serviceCount`/`clusterCount`；验证 `app.go` 中 metrics 初始化路径不受影响。
6. **更新测试并验证** — 运行所有现有 metrics 测试，修复可能因指标名变化导致的断言失败。
7. **清理死代码** — 删除 `prevCPUTimes` 字段。

### 验证命令
- 共享包测试：`cd lib/metrics && go test -v ./...`
- coredns 测试：`cd sidecar/coredns-config-manager && go test -v ./...`
- caddy 测试：`cd sidecar/caddy-config-manager && go test -v ./...`
- 根目录回归：`make test`

## 易踩坑
- `tailscale-manifest/lite-mode` 里的命令必须在该目录下执行。安装器和卸载器通过 `manifest_dir = "."` 从当前目录读取 YAML 清单。
- 卸载行为以当前代码为准，不要只信 README 或 `Makefile` 帮助文案：`tailscale-manifest/lite-mode/src/tailscale_installer/cli.py` 里即使加了 `--uninstall`，`--authkey` 和 `--cluster-name` 仍然被标记为必填。
- `lib/k8sclient.GetConfigOutOfCluster()` 会调用全局 `flag.Parse()`；如果同一进程或同一个测试二进制里重复调用，需要显式处理 flag 状态。
- Namespace 处理并不统一：`k8sclient.GetCurrentNamespace()` 会读取 `POD_NAMESPACE`，但 `sidecar/caddy-config-manager` 的服务发现如果没有接入 `WithNamespace(...)`，仍然默认查 `default` 命名空间。
- 根目录 README 明确说明 `tailscale-manifest/lite-mode` 仍然是实验性功能，不要在业务集群上测试。
