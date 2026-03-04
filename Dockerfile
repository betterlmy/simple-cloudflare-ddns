# 多阶段构建
# 第一阶段：构建阶段
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用（使用 TARGETARCH/TARGETOS 支持多架构交叉编译）
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o scfddns .

# 第二阶段：运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求
RUN apk --no-cache add ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/scfddns .

# 更改文件权限
RUN chown -R appuser:appgroup /app

# 切换到非 root 用户
USER appuser

# 配置通过环境变量传入（CF_API_TOKEN、CF_ZONE_ID、CF_RECORD_NAME、CF_RECORD_TYPE 等）
CMD ["./scfddns"]
