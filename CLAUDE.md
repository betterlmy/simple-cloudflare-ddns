# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 文档同步规范

修改环境变量、配置项或核心逻辑后，必须同步更新以下所有文件，不得遗漏：

| 文件                                           | 说明                          |
| ---------------------------------------------- | ----------------------------- |
| `README.md`                                    | 英文文档，配置表格            |
| `README_CN.md`                                 | 中文文档，配置表格            |
| `DOCKERHUB_README.md`                          | Docker Hub 专用文档，配置表格 |
| `CLAUDE.md`                                    | 架构说明中的配置变量列表      |
| `docker-compose.yml`                           | 注释示例                      |
| `packaging/default-config`                     | UCI 默认配置（OpenWrt）       |
| `packaging/cloudflare-ddns-wrapper`            | UCI 读取 + export（OpenWrt）  |
| `packaging/luci/model/cbi/cloudflare-ddns.lua` | LuCI 配置页表单（OpenWrt）    |

## 常用命令

```bash
# 直接运行（配置通过环境变量传入，真实值见 .env）
# export $(cat .env | xargs)
# 或手动指定：
# export CF_API_TOKEN=your-token CF_ZONE_ID=your-zone-id CF_RECORD_NAME=home.example.com CF_RECORD_TYPE=A
go run main.go

# 编译
go build -o scfddns .

# 运行编译产物
./scfddns

# 运行一次后退出（适合 cron 使用）
./scfddns -once

# 构建 Docker 镜像
docker build -t simple-cloudflare-ddns:latest .

# 通过 Docker Compose 运行（需先在 shell 或 .env 中设置环境变量）
docker-compose up -d
```

本项目没有测试文件。

## Docker Hub 推送流程

```bash
# 1. 登录 Docker Hub（只需一次）
docker login

# 2. 创建支持多架构的 buildx builder（只需一次）
#    OrbStack 需通过 --driver-opt 注入代理，127.0.0.1 在容器内要换成 host.docker.internal
docker buildx create --use \
  --driver-opt env.HTTP_PROXY=http://host.docker.internal:3306 \
  --driver-opt env.HTTPS_PROXY=http://host.docker.internal:3306

# 3. 构建多架构镜像并直接推送（在项目根目录执行）
docker buildx build --platform linux/amd64,linux/arm64 \
  -t betterlmy/simple-cloudflare-ddns:latest \
  --push .

# 4. 验证推送结果
docker buildx imagetools inspect betterlmy/simple-cloudflare-ddns:latest
```

## 架构说明

单文件 Go 应用（`main.go`），无外部依赖，模块名为 `scfddns`。

**核心流程**（`runUpdate`）：
1. 通过外部服务（icanhazip、ifconfig.co、ipify）获取公网 IP —— 优先尝试上次成功的服务
2. 从 Cloudflare API 查询当前 DNS 记录（`getDNSRecord`）
3. 若公网 IP 与 DNS 记录不一致，通过 Cloudflare PUT API 更新（`updateDNSRecord`）

**关键全局变量**：`lastReqURL`（上次成功的 IP 检测服务地址）—— 重启后重置。

**配置方式（环境变量）**：
- `CF_API_TOKEN`、`CF_ZONE_ID`、`CF_RECORD_NAME`、`CF_RECORD_TYPE`（A 或 AAAA）—— 必填
- `CF_CHECK_INTERVAL` —— 未设置时默认 300 秒
- `CF_TTL`、`CF_PROXIED` —— 可选，省略时沿用 Cloudflare 上已有记录的值
- `CF_IP_URLS` —— 可选，逗号分隔的 IP 检测服务 URL 列表，省略时使用内置默认列表

**Dockerfile**：多阶段构建，生成名为 `scfddns` 的静态二进制文件，以非 root 用户 `appuser` 运行于 Alpine 镜像，支持 AMD64 和 ARM64。

**代理说明**：若运行环境使用代理，需将 `*.ipify.org`、`ifconfig.co`、`*.icanhazip.com` 加入代理绕过列表。

## IPK 打包（iStoreOS/OpenWrt）

### 本地打包

```bash
# 生成 x86_64 IPK（同理支持 aarch64、arm_cortex-a7）
bash packaging/build-ipk.sh x86_64
```

push `v*` tag 后 GitHub Actions 自动：
- 构建三架构 IPK 并发布到 GitHub Release
- 构建 `linux/amd64`、`linux/arm64` Docker 镜像并推送到 Docker Hub（`latest` + 版本号 tag）
- 同步 `DOCKERHUB_README.md` 到 Docker Hub 仓库描述

**GitHub Actions 所需 Secrets**（Settings → Secrets → Repository secrets）：
- `DOCKERHUB_USERNAME`：Docker Hub 用户名
- `DOCKERHUB_TOKEN`：Docker Hub Personal Access Token，权限须为 **Read, Write, Delete**（只有 Read & Write 会导致 README 更新报 403 Forbidden）

### 关键文件

| 文件                                | 说明                                           |
| ----------------------------------- | ---------------------------------------------- |
| `packaging/cloudflare-ddns-wrapper` | procd 启动入口，直接读 UCI 配置并 exec scfddns |
| `packaging/cloudflare-ddns-ctl`     | LuCI 专用控制脚本，通过 ubus 直接与 procd 通信 |
| `packaging/init.d/cloudflare-ddns`  | procd 服务脚本，负责开机自启和 enabled 检查    |
| `packaging/luci/`                   | LuCI 界面（配置页 + 日志页）                   |

### 踩过的坑

**1. IPK 格式**
iStoreOS 的 IPK 是 `gzip(tar(debian-binary, data.tar.gz, control.tar.gz))`，不是标准 Debian 的 `ar` 格式。macOS 的 `tar`/`ar` 会引入系统扩展头，必须用 Python `tarfile` 模块构建。

**2. procd_set_param env 不可靠**
在 init.d 里用 `procd_set_param env CF_TTL= CF_PROXIED=`（空值）时，procd 的参数解析会静默失败，导致 scfddns 启动时读不到任何环境变量，立即 `log.Fatalf` 退出。解决方案：让 `cloudflare-ddns-wrapper` 自己 `. /lib/functions.sh` 读 UCI，init.d 完全不传 env 参数。

**3. 从 uhttpd（LuCI）调 init.d 无法注册 procd 实例**
LuCI 的 `os.execute("/etc/init.d/... start")` 运行在 uhttpd 进程上下文中，`procd_close_instance` 内部的 ubus 调用会静默失败（procd 日志：`Failed to add object: Invalid argument`），导致实例始终为空。根本原因尚不明确（与进程组/会话/文件描述符相关），但 **解决方案确定**：用 `cloudflare-ddns-ctl` 直接调用 `ubus call service set '...'`，完全绕过 init.d 的调用链，从 uhttpd 上下文也能正常注册 procd 实例。

**4. procd respawn retry=0**
在此版本 iStoreOS 上，`procd_set_param respawn 3600 5 0`（retry=0）被 procd 拒绝（Invalid argument）。用 `procd_set_param respawn`（不传参，走默认值 3600/5/5）即可。
