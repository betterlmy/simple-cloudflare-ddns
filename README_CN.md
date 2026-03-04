# Simple Cloudflare DDNS

[English Version / 英文版本](README.md)

一个极简的 Cloudflare 动态 DNS 客户端，使用 Go 编写，无外部依赖，单二进制文件，通过环境变量配置。

**工作原理**：检测公网 IP → 与 Cloudflare DNS 对比 → IP 变化时更新

## 功能特性

- 自动选择最快的 IP 检测服务（icanhazip / ifconfig.co / ipify）
- 记忆上次成功的服务，减少延迟
- 仅在 IP 真正变化时才调用 Cloudflare API
- 支持 IPv4（`A` 记录）和 IPv6（`AAAA` 记录）
- 支持单次运行模式，适合 cron 定时任务

## 运行环境

- Go 1.18+（或使用 Docker）
- 能访问 Cloudflare API 的网络

## 配置

所有配置通过环境变量传入：

| 变量名 | 必填 | 说明 |
|--------|------|------|
| `CF_API_TOKEN` | ✅ | Cloudflare API Token（需要 DNS 编辑权限） |
| `CF_ZONE_ID` | ✅ | Cloudflare Zone ID |
| `CF_RECORD_NAME` | ✅ | 要更新的 DNS 记录，如 `home.example.com` |
| `CF_RECORD_TYPE` | ✅ | `A`（IPv4）或 `AAAA`（IPv6） |
| `CF_CHECK_INTERVAL` | — | 检查间隔（秒），默认 `300` |
| `CF_TTL` | — | DNS TTL（秒），默认沿用 Cloudflare 现有记录 |
| `CF_PROXIED` | — | `true` 或 `false`，默认沿用 Cloudflare 现有记录 |

## 快速开始

### 直接运行

```bash
export CF_API_TOKEN=your-token
export CF_ZONE_ID=your-zone-id
export CF_RECORD_NAME=home.example.com
export CF_RECORD_TYPE=A

go run main.go
```

### 编译运行

```bash
go build -o scfddns .
./scfddns
```

### 单次运行（适合 cron）

```bash
./scfddns -once
```

## Docker 部署

> 如果使用代理，请将 `*.ipify.org`、`ifconfig.co`、`*.icanhazip.com` 加入代理绕过列表。

### docker run

```bash
docker run -d \
  --name cloudflare-ddns \
  --restart unless-stopped \
  -e CF_API_TOKEN=your-token \
  -e CF_ZONE_ID=your-zone-id \
  -e CF_RECORD_NAME=home.example.com \
  -e CF_RECORD_TYPE=A \
  betterlmy/simple-cloudflare-ddns:latest
```

### Docker Compose

创建 `.env` 文件：
```env
CF_API_TOKEN=your-token
CF_ZONE_ID=your-zone-id
CF_RECORD_NAME=home.example.com
CF_RECORD_TYPE=A
```

然后运行：
```bash
docker-compose up -d
```

### 构建自己的镜像

```bash
git clone https://github.com/betterlmy/simple-cloudflare-ddns.git
cd simple-cloudflare-ddns
docker build -t simple-cloudflare-ddns:latest .
```

### 镜像特性
- 以非 root 用户运行
- 多架构：AMD64 和 ARM64
- 基于 Alpine（约 20MB）

## 命令行参数

| 参数 | 说明 |
|------|------|
| `-once` | 运行一次后退出（适合 cron 定时任务） |

## 如何获取 Cloudflare 凭证

### API Token
1. 前往 [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. 点击 "Create Token" → "编辑区域 DNS" → "使用模板"
   ![dns module](images/dnsmodule.png)
3. 设置权限：`Zone:DNS:编辑`，选择目标域名
   ![config](images/config.png)
4. 创建并复制生成的 Token

### Zone ID
1. 前往 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 选择你的域名
3. 在 "Overview" 页面右下角找到 "Zone ID"
   ![zone-id](images/zoneId.png)

## 常见问题

<details>
<summary><strong>无法获取公网 IP？</strong></summary>

- 程序会自动尝试多个服务
- 全部失败时，检查网络连接或代理绕过列表是否正确配置
- 重启程序重试
</details>

<details>
<summary><strong>DNS 更新不生效？</strong></summary>

- 确认 `CF_ZONE_ID`、`CF_RECORD_NAME`、`CF_RECORD_TYPE` 填写正确
- 检查 API Token 是否有 DNS 编辑权限
- 确认 Cloudflare 控制台没有同名但不同类型的冲突记录
</details>

<details>
<summary><strong>更新太频繁？</strong></summary>

- 增大 `CF_CHECK_INTERVAL`（建议家庭用户设置 300–600 秒）
</details>

## 许可证

MIT License

---
<div align="center">
⭐ 如果觉得好用，请给个 Star 支持一下～
</div>
