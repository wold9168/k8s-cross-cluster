# AGENTS.md

## 仓库结构
- 这是一个多模块仓库，不是单一 Go workspace。Go 模块分别在 `lib/k8sclient`、`lib/tailscaled-client`、`sidecar/caddy-config-manager`、`sidecar/coredns-config-manager`；`tailscale-manifest/lite-mode` 是独立的 Python/`uv` 项目。
- 仓库里没有 `go.work`。跑 Go 测试时要进入对应模块目录，不要在仓库根目录想当然地执行 `go test ./...`。
- 根目录的 `make test` 只会跑 `lib/k8sclient`、`sidecar/caddy-config-manager` 和 `sidecar/coredns-config-manager`；不会覆盖 `lib/tailscaled-client`。

## 已验证命令
- 根目录回归测试：`make test`
- 单个 Go 模块：在对应模块目录执行 `go test ./...`
- `lib/k8sclient` 在根目录 `Makefile` 里实际执行的是 `go test -v .`，不是 `./...`。
- Python 安装器开发：进入 `tailscale-manifest/lite-mode` 后使用 `just setup-dev`、`just test`、`just lint`、`just format`（该目录已从 Makefile 迁移到 Justfile）
- Sidecar 镜像构建：`make sidecar-image-build`、`make caddy-config-manager-image-build`、`make coredns-config-manager-image-build`
- `sidecar/caddy-config-manager` 模块内可用 `make test` 和 `make binary-build`；`binary-build` 会产出 Linux 二进制 `caddy-config-manager`。

## Makefile 约定
- 现有两个 Makefile（根目录、`sidecar/caddy-config-manager/`）。无 `sidecar/coredns-config-manager/Makefile`；`tailscale-manifest/lite-mode/` 已迁移到 Justfile。
- 根目录镜像构建默认使用 `TAG=$(git rev-parse HEAD)`、`REGISTRY=ghcr.io`；`REPO_NAME` 优先取 git remote 里的 GitHub 仓库名，取不到时回退为当前用户名。
- `tailscale-manifest/lite-mode` 的单集群安装通过 `just install --authkey ... --login-server ...` 直接透传给 `uv run tailscale-install`；多集群批量安装用 `just install-all "ctx1:name1 ctx2:name2"`，需预先 `export TS_AUTHKEY TS_LOGIN_SERVER HEADSCALE_API_KEY`。
- `tailscale-manifest/lite-mode` 的卸载用 `just uninstall ctx=...`（单集群）或 `just uninstall-all "ctx1:name1 cx2:name2"`（批量）。注意 `--login-server` 是 installer 的顶层参数，不要塞进 `--extra-args`。

## 构建与 CI 注意点
- Docker 构建上下文必须用仓库根目录。两个 sidecar 的 Dockerfile 都用了 `COPY ./../..`，如果在模块目录单独 `docker build` 会失败。
- CI 在 `.github/workflows/docker-build.yml` 中调用根目录 `Makefile`；如果改了镜像名、tag 或构建目标，要同时更新这两处。

## 运行时连接关系
- `sidecar/caddy-config-manager` 会写入 `/config/Caddyfile`，通过 `http://localhost:2019/reload` 触发 Caddy 重载，对外 API 监听 `:8091`，metrics 监听 `:8090`。
- `sidecar/coredns-config-manager` 提供 UDP DNS `:10053`、metrics `:8080`、服务发现 API `:8081`，并更新 `kube-system/coredns` 这个 CoreDNS ConfigMap。
- 跨集群发现是 sidecar 之间互通：`coredns-config-manager` 通过 SOCKS5 `127.0.0.1:1055` 访问远端 `/svc`，`caddy-config-manager` 基于本地 Service 生成 `<service>.<namespace>.svc.<cluster>.remote` 和 `.clusterset.remote` 域名映射。

## 生成文件
- `sidecar/caddy-config-manager/docs/docs.go` 和 `sidecar/coredns-config-manager/docs/docs.go` 由 `swaggo/swag` 生成，不要手改。

## Metrics 系统

### 架构（当前）
```
lib/metrics/                           ← 共享包
├── collector.go                       ← BaseCollector（Go runtime、CPU、memory、disk、network），实现 prometheus.Collector
├── handler.go                         ← HTTP handler + StartServer()，支持 Prometheus/JSON 两种格式
├── manager.go                         ← （死代码，未被 sidecar 引用）
└── helpers.go                         ← loadAvgFromHost()

sidecar/coredns-config-manager/metrics/
├── collector.go                       ← 嵌入 lib/metrics.BaseCollector，添加 dns_record_count、dns_service_count、dns_cluster_count
├── manager.go                         ← 单例 Manager，暴露 UpdateDNSRecordCount / UpdateServiceCount / UpdateClusterCount / Start
└── collector_test.go                  ← collector 单元测试

sidecar/caddy-config-manager/metrics/
├── collector.go                       ← 嵌入 lib/metrics.BaseCollector，添加 caddy_config_update_total（Counter）、caddy_last_config_update_timestamp、caddy_service_count
└── manager.go                         ← 单例 Manager，暴露 UpdateConfigUpdate / UpdateServiceCount / Start
```

### 已修复（重构完成）
| # | 问题 | 修复内容 |
|---|------|---------|
| 1 | coredns JSON `gc_cycles` 始终为 0 | `debug.GCStats{}.NumGC` → `memStats.NumGC` |
| 2 | `go_threads` 用 `runtime.NumCgoCall()` | 改用 `process.NumThreads()` |
| 3 | 共享指标名不一致 | 统一 `_total` 后缀 |
| 4 | `caddy_config_update_total` Gauge→Counter | 改为 `prometheus.NewCounter` |
| 5 | coredns 未暴露 service/cluster count | 新增 `dns_service_count`、`dns_cluster_count` |
| 6 | caddy `Manager.Start()` 多余锁 | 移除 |
| 7 | collector 代码重复、`prevCPUTimes` 死代码、`loadAvgFromHost()` 重复 | 引入 `lib/metrics` 共享包 |
| 8 | `prometheus/client_golang` 版本不一致 | 统一 v1.23.2 |

### 现存问题（待修复）
| # | 严重度 | 问题 | 位置 |
|---|--------|------|------|
| A | 🔴 | **caddy `caddy_config_update_total` Counter 值始终为 0** — `UpdateConfigUpdate()` 只递增内部 `configUpdateCountValue`，从未调 `ConfigUpdateCount.Inc()`，Prometheus 输出永为 0 | `sidecar/caddy-config-manager/metrics/collector.go:89-93` |
| B | 🟡 | **`lib/metrics/manager.go` 是死代码** — `Manager`/`SetSetupFunc`/`Init()` 未被任何 sidecar 引用 | `lib/metrics/manager.go` |
| C | 🟡 | **`lib/metrics` 无测试；caddy metrics 无测试文件** | `lib/metrics/`、`sidecar/caddy-config-manager/metrics/` |
| D | 🟢 | `/metrics` 端点无请求超时保护 | `lib/metrics/handler.go` |
| E | 🟢 | `cpu_context_switches_total` 本质是累计值却设为 Gauge（边界模糊，可接受但非最佳实践） | `lib/metrics/collector.go` |

### 验证命令
- 共享包：`cd lib/metrics && go test -v ./...`
- coredns：`cd sidecar/coredns-config-manager && go test -v ./...`
- caddy：`cd sidecar/caddy-config-manager && go test -v ./...`
- 根目录回归：`make test`

## 其他现存问题

| # | 严重度 | 问题 | 位置 |
|---|--------|------|------|
| F | 🔴 | **Service 不存在时 CoreDNS upstream 为空** — `GetNamedServiceClusterIP` 失败后 `currentSvcClusterIP=""`，`EnsureConfig` 会生成 `forward . ` 空上游，静默破坏 DNS 解析 | `dns_config_manager.go:164-176` |
| G | 🟡 | **`GetNamedServiceClusterIP` 对 headless Service 返回 `"None"`** — 拼接端口后得到 `"None:10053"`，传给 CoreDNS 导致畸形转发地址 | `lib/k8sclient/get_service.go:49-61` |
| H | 🟡 | **`GetNamedServiceClusterIP` 未校验空 `serviceName`** — 传空字符串会发出无效 K8s API 调用，返回隐晦错误 | `lib/k8sclient/get_service.go:49-61` |
| I | 🟡 | **`getProcessThreadCount()` 出错返回 `-1`** — `-1` 被直接 Set 到 Gauge，产生语义误导 | `lib/metrics/collector.go:328-338` |
| J | 🟡 | **`network_connections` 语义错误** — 用 `PacketsSent+PacketsRecv)/100` 冒充连接数，毫无意义 | `lib/metrics/collector.go:471` |
| K | 🟡 | **`tcp_retransmit_rate` 实为输入错误率** — 用 `Errin`（接收错误）而非 TCP 重传段数 | `lib/metrics/collector.go:468-469` |
| L | 🟡 | **caddy 服务发现始终查 `default` 命名空间** — `main.go` 获取了当前 namespace 但未传给 `NewServiceDiscovery`，非 default 命名空间会发现错误的服务 | `service_discovery.go:44` |
| M | 🟡 | **根 `make test` 未覆盖 `lib/metrics`** — 缺少 `cd ./lib/metrics && go test -v ./...` | `Makefile:42-46` |
| N | 🟡 | **`lib/tailscaled-client` 无测试** | `lib/tailscaled-client/` |
| O | 🟡 | **`lib/k8sclient` 使用弃用 `io/ioutil`** — Go 1.16+ 应用 `os.ReadFile` | `lib/k8sclient/get_namespace.go:4,25` |
| P | 🟡 | **caddy `constants.go` 未使用的常量** — `syncIntervalKey`、`caddyAdminPortKey` 定义了但从未引用 | `constants.go:8,21` |
| Q | 🟡 | **9 个已弃用函数仍留在代码库中** — `GetCurrentPodServiceClusterIP`、`peer_processor.go`、`dns_updater.go`、`generate_*` 等仅被测试引用或完全无人调用 | 多处 |
| R | 🟡 | **`apply-tailscale.sh` 与 Python 安装器功能重复** — 无 Headscale 去重检查，已过时 | `tailscale-manifest/lite-mode/` |
| S | 🟢 | **YAML manifest `nodePort` 硬编码** — `30880`/`30881`/`30890`/`30891` 可能与集群其他 Service 冲突 | `tailscale-userspace-proxy.yaml` |
| T | 🟢 | **YAML manifest 使用 `:latest` 镜像标签** — 生产应固定到 commit hash 或语义版本 | `tailscale-userspace-proxy.yaml:195,234` |
| U | 🟢 | **`Justfile test-debian` 使用 `-it`** — 在 CI/非 TTY 环境下会失败 | `Justfile` |
| V | 🟢 | **`disk`/`network` 共用单个 `prevTime`** — 耦合脆弱，若调用顺序改变会损坏变化率计算 | `lib/metrics/collector.go:370-490` |

## 工程与 CI/构建

| # | 严重度 | 问题 | 位置 |
|---|--------|------|------|
| W | 🔴 | **仓库根目录无 `.gitignore`** — Go 编译产物 (`*.o`/`*.a`/`.exe`)、IDE 文件 (`.idea/`/`.vscode/`)、二进制等可能被意外提交 | 根目录 |
| X | 🔴 | **仓库根目录无 `.dockerignore`** — Docker 构建上下文包含整个仓库（含 `.venv`、`__pycache__`、测试缓存等），拖慢构建 | 根目录 |
| Y | 🟡 | **`list/` 目录含 Python virtualenv 残留** — `bin/`、`lib/`、`lib64/`、`pyvenv.cfg` 被提交进仓库，.gitignore 虽写了 `*` 但这些文件早于 ignore 规则存在 | `tailscale-manifest/lite-mode/list/` |
| Z | 🟡 | **CI 仅在 `snapshot`/`refactor-snapshot` 分支触发** — `dev`/`main` 分支的 push 不会触发构建 | `.github/workflows/docker-build.yml:6-7` |
| AA | 🟡 | **CI 构建前不跑测试** — Docker build workflow 没有 `go test` 步骤，可能在 CI 中合入未通过测试的代码 | `.github/workflows/docker-build.yml` |
| AB | 🟡 | **Go 版本不一致** — `lib/k8sclient/go.mod` 声明 `go 1.24.11`，其他所有模块和 Dockerfile 均为 `1.25.x` | `lib/k8sclient/go.mod:3` |
| AC | 🟡 | **YAML manifest 镜像地址硬编码为个人账户** — `ghcr.io/wold9168/...:latest` 在其他开发者 fork 下不可用 | `tailscale-userspace-proxy.yaml:195,234` |
| AD | 🟢 | **CI 使用 `docker buildx build --load`** — 只将镜像加载到本地 Docker daemon，`--push` 分两步执行，原子性差 | `Makefile:29` |
| AE | 🟢 | **无 `.editorconfig`、无 `.golangci.yml`、无 `pre-commit` hook** — 缺少 IDE 间代码风格一致性、lint CI 门禁 | 根目录 |

## 易踩坑
- `tailscale-manifest/lite-mode` 里的命令必须在该目录下执行。安装器和卸载器通过 `manifest_dir = "."` 从当前目录读取 YAML 清单。
- 卸载行为以当前代码为准：`tailscale-manifest/lite-mode/src/tailscale_installer/cli.py` 里即使加了 `--uninstall`，`--authkey` 和 `--cluster-name` 仍然被标记为必填。
- `lib/k8sclient.GetConfigOutOfCluster()` 会调用全局 `flag.Parse()`；如果同一进程或同一个测试二进制里重复调用，需要显式处理 flag 状态。
- Namespace 处理并不统一：`k8sclient.GetCurrentNamespace()` 会读取 `POD_NAMESPACE`，但 `sidecar/caddy-config-manager` 的服务发现如果没有接入 `WithNamespace(...)`，仍然默认查 `default` 命名空间。
- 根目录 README 明确说明 `tailscale-manifest/lite-mode` 仍然是实验性功能，不要在业务集群上测试。
