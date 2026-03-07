# k8s-cross-cluster

基于 tailscale 为 k8s cluster 提供了一种 L5 的跨集群互联方案

`tailscale-manifest/lite-mode` 目录下的安装脚本仍有较多问题，请不要在业务集群上进行测试。

## 轻量模式与可行性验证

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

### 轻量模式工作原理

项目流程

```mermaid
flowchart TB
    subgraph 安装阶段["安装阶段"]
        A1["tailscale-manifest/lite-mode<br/>Python 安装器"] --> A2["生成 K8s 资源清单"]
        A2 --> A3["ServiceAccount<br/>RBAC"]
        A2 --> A4["ConfigMap<br/>tailscale-extra-args<br/>cluster-name"]
        A2 --> A5["Secret<br/>tailscale-auth"]
        A2 --> A6["Deployment<br/>Tailscale Userspace Proxy"]
    end

    subgraph 运行阶段["运行阶段 - Sidecar 模式"]
        B1["caddy-config-manager<br/>HTTP 代理配置"] <--> K8sAPI["K8s API Server"]
        B2["coredns-config-manager<br/>DNS 配置管理"] <--> K8sAPI
        B2 <--> TS["Tailscale<br/>Local API"]
        
        B1 --> B3["ServiceDiscovery<br/>服务发现"]
        B1 --> B4["CaddyConfigGenerator<br/>配置生成"]
        B1 --> B5["写入 /config/Caddyfile"]
        
        B2 --> B6["PeerDiscovery<br/>节点发现"]
        B2 --> B7["DNSRecordManager<br/>DNS记录管理"]
        B2 --> B8["DNSServer<br/>:10053"]
        B2 --> B9["更新 CoreDNS ConfigMap"]
    end

    subgraph 跨集群通信["跨集群通信流程"]
        C1["Client Pod"] -->|"DNS查询<br/>k8sbc.default.svc.na.remote"| C2["集群 DNS<br/>CoreDNS"]
        C2 -->|"匹配 *.svc.*.remote"| C3["Sidecar DNS<br/>:10053"]
        C3 -->|"从 PeerInfo 获取<br/>返回 Tailscale IP"| C1
        
        C1 -->|"HTTP 请求<br/>k8sbc.default.svc.na.remote"| C4["Caddy反向代理"]
        C4 -->|"转发到 ClusterIP"| C5["本地 Service"]
        C5 -->|"通过 Tailscale VPN"| C6["目标集群<br/>Tailscale Pod"]
        
        C1b["Client Pod"] -->|"DNS查询<br/>myapp.default.svc.clusterset.remote"| C2b["集群 DNS<br/>CoreDNS"]
        C2b -->|"匹配 *.svc.clusterset.remote"| C3b["Sidecar DNS<br/>:10053"]
        C3b -->|"负载均衡到任一集群<br/>返回 Tailscale IP"| C1b
    end

    A6 --> B1
    A6 --> B2
    
    style A1 fill:#f9f,stroke:#333
    style B1 fill:#bbf,stroke:#333
    style B2 fill:#bbf,stroke:#333
    style C1 fill:#bfb,stroke:#333
```

时序图

```mermaid
sequenceDiagram
    participant U as 用户/应用
    participant ClusterDNS as 集群 DNS
    participant SidecarDNS as coredns-config-manager
    participant PD as PeerDiscovery
    participant Caddy as caddy-config-manager
    participant TS as Tailscale
    participant K8sAPI as K8s API

    Note over U,ClusterDNS: 访问跨集群服务<br/>域名: k8sbc.default.svc.na.remote

    U->>ClusterDNS: DNS 查询
    ClusterDNS->>SidecarDNS: 转发 *.svc.na.remote
    SidecarDNS->>PD: 获取 PeerInfo
    PD->>TS: 获取对端节点信息
    TS-->>PD: PeerInfo (Tailscale IPs)
    SidecarDNS-->>U: 返回目标集群 Tailscale IP

    U->>Caddy: HTTP 请求 (k8sbc.default.svc.na.remote)
    Caddy->>K8sAPI: 查询 Service ClusterIP
    K8sAPI-->>Caddy: 返回 ClusterIP
    Caddy->>U: 反向代理到 ClusterIP

    Note over U,TS: 请求通过 Tailscale VPN 发送到目标集群
```

具体而言，发送端通过 SOCKS5_PROXY 环境变量获取代理的配置，对远程集群的所有请求都被解析到远程集群的反向代理上，每个集群的反向代理负责将入站流量转发到本集群的服务上。


### 服务发现

服务发现架构

```mermaid
graph TB
    subgraph "Local Cluster"
        A[Client Pod] -->|DNS Query| B[CoreDNS]
        B -->|Forward| C[Embedded DNS Server<br/>:10053]
        C -->|Intercept| D[LoadBalancer]
        D -->|Query| E[ServiceDiscovery]
        
        F[DNSConfigManager] -->|Sync Every 10s| G[LoadBalancer.RefreshServices]
        G -->|Refresh Peer Cache| H[PeerDiscovery.GetPeers]
        H -->|Get PeerInfo| I[Tailscale]
        G -->|Fetch Services| E[ServiceDiscovery.DiscoverServices]
        
        D -->|Get Tailscale IP| J[peerCache]
        J -->|clusterName → PeerInfo| K[Tailscale IPs]
        
        E[ServiceDiscovery] -->|HTTP GET :8080/svc| L[Remote svc Handler]
        E -->|Cache| M[(Service Cache<br/>cluster → services)]
    end
    
    subgraph "Remote Cluster 1"
        N[Peer Node 1<br/>na-tsgateway] -->|HTTP :8080/svc| O[svc Handler]
        O -->|Return| P[Service List<br/>myapp.default, api.prod, ...]
        N -->|PeerInfo| Q[Tailscale IPs<br/>100.64.0.1]
    end
    
    subgraph "Remote Cluster 2"
        R[Peer Node 2<br/>nb-tsgateway] -->|HTTP :8080/svc| S[svc Handler]
        S -->|Return| T[Service List<br/>myapp.default, db.staging, ...]
        R -->|PeerInfo| U[Tailscale IPs<br/>100.64.0.2]
    end
    
    E -->|HTTP GET| N
    E -->|HTTP GET| R
    H -->|Get Peers| N
    H -->|Get Peers| R

```

服务发现流程

```mermaid
sequenceDiagram
    participant DM as DNSConfigManager
    participant LB as LoadBalancer
    participant SD as ServiceDiscovery
    participant PD as PeerDiscovery
    participant TS as Tailscale
    participant RC1 as Remote Cluster 1
    participant RC2 as Remote Cluster 2
    participant Cache as Service Cache

    DM->>LB: RefreshServices(ctx)
    LB->>PD: GetPeers(ctx)
    PD->>TS: Status()
    TS-->>PD: Peer List
    PD-->>LB: []PeerInfo
    LB->>LB: Update peerCache (clusterName → TailscaleIPs)
    
    LB->>SD: DiscoverServices(ctx)
    SD->>PD: GetPeers(ctx)
    PD-->>SD: []PeerInfo
    
    par Parallel Fetch from All Peers
        SD->>RC1: HTTP GET http://peer1:8080/svc
        RC1-->>SD: RemoteServiceList<br/>{timestamp, services, count}
        SD->>Cache: Store cluster1 → services
        
        SD->>RC2: HTTP GET http://peer2:8080/svc
        RC2-->>SD: RemoteServiceList<br/>{timestamp, services, count}
        SD->>Cache: Store cluster2 → services
    end
    
    SD-->>LB: Discovery Complete
    LB-->>DM: Refresh Complete
```

```mermaid
stateDiagram-v2
    [*] --> Initializing: DNSConfigManager.Initialize()
    Initializing --> Ready: Initial Discovery Complete
    
    Ready --> Refreshing: Sync() triggered (every 10s)
    Refreshing --> Ready: Discovery Complete
    
    Ready --> QueryHandling: DNS Query Received
    QueryHandling --> Ready: Response Sent
    
    QueryHandling --> CacheMiss: No endpoints found
    CacheMiss --> Refreshing: Trigger Refresh
    
    Ready --> Error: Discovery Failed
    Error --> Ready: Retry on Next Sync

```

`.clusterset.remote` 采用轮询原则进行分布在不同集群上的同名服务的负载均衡。轮换范围不包括当前集群的服务。

```mermaid
graph TB
    A["myapp.default.svc.clusterset.remote"] --> B{ParseClustersetDomain}
    B -->|"parts[0]"| C["myapp (serviceName)"]
    B -->|"parts[1]"| D["default (namespace)"]
    B -->|"parts[2]"| E["svc (fixed)"]
    B -->|"parts[3]"| F["clusterset (fixed)"]
    B -->|"parts[4]"| G["remote (fixed)"]
    
    C --> H[Lookup in Cache]
    D --> H
    H --> I{Found?}
    I -->|"Yes"| J[Return Endpoints]
    I -->|"No"| K[NXDOMAIN]
    
    J --> L[Round-Robin Select]
    L --> M[从 peerCache 获取 Tailscale IP]
    M --> N[返回 Tailscale IP 而不是 ClusterIP]
```

## 增强模式

增强模式期望实现一个 L3 的代理。

WIP

- [ ] 从发送端集群到 Tailscale 节点到接收端集群的 Tailscale 节点的路由规则暂时没有可用的配置方案

```bash
minikube start --driver=kvm2 --kvm-network=minikube-net2 --profile=cluster2 --host-only-cidr=192.168.140.0/24
minikube start --driver=kvm2 --profile=cluster2 --host-only-cidr=192.168.140.128/25 --service-cluster-ip-range=10.112.0.0/12 # 使得两个集群服务的 CIDR 错开

cd tailscale-manifest/lite-mode
make ARGS="--authkey your-headscale-preauth-key --extra-args='--login-server your-headscale-server-ip-and-port --advertise-route=10.96.0.0/12' --context cluster1 --cluster-name na" install # 10.96.0.0/12 是 k8s 创建服务所默认使用的 CIDR，集群创建时通过 --service-cluster-ip-range 参数控制
make ARGS="--authkey your-headscale-preauth-key --extra-args='--login-server your-headscale-server-ip-and-port --advertise-routes=10.112.0.0/12' --context cluster2 --cluster-name nb" install

kubectl create deployment kubernetes-bootcamp --image=gcr.io/google-samples/kubernetes-bootcamp:v1 --context cluster1
kubectl expose deployment kubernetes-bootcamp --port=80 --target-port=8080 --name=k8sbc --context cluster1

# 在 cluster2 中拉取 debian:12 作为测试镜像，并赋予特权模式
kubectl --context cluster2 run -it --rm \
  --image=debian:12 \
  --restart=Never --privileged \
  debian-test \
  -- bash

# 获取 Tailscale 的 Pod IP
kubectl get pods -l app=tailscale-proxy -o yaml --context cluster2|grep -i podip:

# 以下指令在 debian-test 测试镜像中进行
$ apt update && apt install -y curl iproute2
$ ip route add 10.96.0.0/12 via <PodIP> onlink dev eth0
$ curl -k -v https://k8sbc.default.svc.na.remote
# 使用 服务名.命名空间.svc.tailscale节点名（无-tsgateway后缀）.remote 作为远程连接的域名
$ exit

# 清理现场，卸载服务
make ARGS="--authkey your-headscale-preauth-key --login-server your-headscale-server-ip-and-port --context cluster1 --cluster-name na" uninstall
make ARGS="--authkey your-headscale-preauth-key --login-server your-headscale-server-ip-and-port --context cluster2 --cluster-name nb" uninstall
```
