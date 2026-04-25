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
- `sidecar/caddy-config-manager` 会写入 `/config/Caddyfile`，通过 `http://localhost:2019/reload` 触发 Caddy 重载，对外 API 监听 `:8081`，metrics 监听 `:8082`。
- `sidecar/coredns-config-manager` 提供 UDP DNS `:10053`、metrics `:8080`、服务发现 API `:8081`，并更新 `kube-system/coredns` 这个 CoreDNS ConfigMap。
- 跨集群发现是 sidecar 之间互通：`coredns-config-manager` 通过 SOCKS5 `127.0.0.1:1055` 访问远端 `/svc`，`caddy-config-manager` 基于本地 Service 生成 `<service>.<namespace>.svc.<cluster>.remote` 和 `.clusterset.remote` 域名映射。

## 生成文件
- `sidecar/caddy-config-manager/docs/docs.go` 和 `sidecar/coredns-config-manager/docs/docs.go` 由 `swaggo/swag` 生成，不要手改。

## 易踩坑
- `tailscale-manifest/lite-mode` 里的命令必须在该目录下执行。安装器和卸载器通过 `manifest_dir = "."` 从当前目录读取 YAML 清单。
- 卸载行为以当前代码为准，不要只信 README 或 `Makefile` 帮助文案：`tailscale-manifest/lite-mode/src/tailscale_installer/cli.py` 里即使加了 `--uninstall`，`--authkey` 和 `--cluster-name` 仍然被标记为必填。
- `lib/k8sclient.GetConfigOutOfCluster()` 会调用全局 `flag.Parse()`；如果同一进程或同一个测试二进制里重复调用，需要显式处理 flag 状态。
- Namespace 处理并不统一：`k8sclient.GetCurrentNamespace()` 会读取 `POD_NAMESPACE`，但 `sidecar/caddy-config-manager` 的服务发现如果没有接入 `WithNamespace(...)`，仍然默认查 `default` 命名空间。
- 根目录 README 明确说明 `tailscale-manifest/lite-mode` 仍然是实验性功能，不要在业务集群上测试。
