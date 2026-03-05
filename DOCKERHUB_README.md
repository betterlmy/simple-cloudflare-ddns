# Simple Cloudflare DDNS

A minimal Cloudflare Dynamic DNS client written in Go. No dependencies, single binary, configured entirely via environment variables.

**How it works**: Detect public IP → Compare with Cloudflare DNS record → Update only when changed

- IPv4 (`A`) and IPv6 (`AAAA`) support
- Auto-selects the fastest IP detection service (icanhazip / ifconfig.co / ipify)
- Multi-arch: `linux/amd64`, `linux/arm64`
- Runs as non-root user, Alpine-based (~20MB)

## Quick Start

### Docker run

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

```yaml
services:
  cloudflare-ddns:
    image: betterlmy/simple-cloudflare-ddns:latest
    restart: unless-stopped
    environment:
      CF_API_TOKEN: your-token
      CF_ZONE_ID: your-zone-id
      CF_RECORD_NAME: home.example.com
      CF_RECORD_TYPE: A
      # CF_CHECK_INTERVAL: 300
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `CF_API_TOKEN` | Yes | Cloudflare API token (DNS edit permission) |
| `CF_ZONE_ID` | Yes | Cloudflare Zone ID |
| `CF_RECORD_NAME` | Yes | DNS record to update, e.g. `home.example.com` |
| `CF_RECORD_TYPE` | Yes | `A` (IPv4) or `AAAA` (IPv6) |
| `CF_CHECK_INTERVAL` | No | Check interval in seconds (default: `300`) |
| `CF_TTL` | No | DNS TTL in seconds (default: inherits existing record) |
| `CF_PROXIED` | No | `true` or `false` (default: inherits existing record) |

## How to Get Cloudflare Credentials

**API Token**
1. Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Create Token → "Edit zone DNS" template
3. Set permissions: `Zone:DNS:Edit`, select your zone

**Zone ID**
1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. Select your domain → find "Zone ID" in the bottom-right of the Overview page

## Notes

- If your environment uses a proxy, add `*.ipify.org`, `ifconfig.co`, `*.icanhazip.com` to the bypass list.
- For cron usage, run the binary with `-once` flag (not applicable in Docker daemon mode).

## Source

[github.com/betterlmy/simple-cloudflare-ddns](https://github.com/betterlmy/simple-cloudflare-ddns)
