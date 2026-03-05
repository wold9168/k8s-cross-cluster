# k8s-cross-cluster

基于 tailscale 为 k8s cluster 提供了一种 L5 的跨集群互联方案

## 可行性验证

`tailscale-manifest/lite-mode` 目录下的安装脚本仍有较多问题，请不要在业务集群上进行测试。

```bash
# 假设你已经有了两个 k8s 集群，其 context 分别为 cluster1 和 cluster2
cd tailscale-manifest/lite-mode
make ARGS="--authkey your-headscale-preauth-key --extra-args='--login-server your-headscale-server-ip-and-port' --context cluster1 --cluster-name na" install
make ARGS="--authkey your-headscale-preauth-key --extra-args='--login-server your-headscale-server-ip-and-port' --context cluster2 --cluster-name nb" install
# --cluster-name 影响对应集群在你的 headscale 中注册的 HostName
# --cluster-name foo，意味着对应的实例在 headscale 中的注册名为 foo-tsgateway
# 请确保 headscale 中没有重名节点

# 在 cluster1 中拉取 gcr.io/google-samples/kubernetes-bootcamp:v1 作为测试镜像
kubectl create deployment kubernetes-bootcamp --image=gcr.io/google-samples/kubernetes-bootcamp:v1 --context cluster1
kubectl expose deployment kubernetes-bootcamp --port=80 --target-port=8080 --name=k8sbc --context cluster1

# 在 cluster2 中拉取 debian:12 作为测试镜像
kubectl --context cluster2 run -it --rm \
  --image=debian:12 \
  --restart=Never \
  debian-test \
  -- bash
# 以下指令在 debian-test 测试镜像中进行
$ apt update && apt install -y curl
$ curl -x socks5://tailscale-proxy.default.svc.cluster.local:1055 -k -v https://k8sbc.default.svc.na.remote
# 使用 服务名.命名空间.svc.tailscale节点名（无-tsgateway后缀）.remote 作为远程连接的域名
$ exit

# 清理现场，卸载服务
make ARGS="--authkey your-headscale-preauth-key --login-server your-headscale-server-ip-and-port --context cluster1 --cluster-name na" uninstall
make ARGS="--authkey your-headscale-preauth-key --login-server your-headscale-server-ip-and-port --context cluster2 --cluster-name nb" uninstall
# 清理现场脚本并不具备清理 headscale 中的 na-tsgateway, nb-tsgateway 节点的功能，该部分需要手动清理
```
