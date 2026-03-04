#!/bin/sh
# build-ipk.sh - 本地打包脚本
# 用法: ./build-ipk.sh <架构>
# 架构可选: x86_64 | aarch64 | arm_cortex-a7
# 示例: ./build-ipk.sh x86_64

set -e

PKG_NAME=cloudflare-ddns
VERSION=${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")}
ARCH=${1:-x86_64}

case "$ARCH" in
  x86_64)
    GOARCH=amd64
    GOARM=
    ;;
  aarch64)
    GOARCH=arm64
    GOARM=
    ;;
  arm_cortex-a7)
    GOARCH=arm
    GOARM=7
    ;;
  *)
    echo "未知架构: $ARCH"
    echo "支持: x86_64, aarch64, arm_cortex-a7"
    exit 1
    ;;
esac

BUILD_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_DIR"' EXIT

PKG_FILE="${PKG_NAME}_${VERSION}_${ARCH}.ipk"

echo "==> 编译 scfddns (GOARCH=$GOARCH GOARM=$GOARM)"
BINARY="$BUILD_DIR/scfddns"
CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH GOARM=$GOARM \
  go build -ldflags="-s -w" -o "$BINARY" .

echo "==> 构建 IPK 目录结构"
DATA_DIR="$BUILD_DIR/data"
CTRL_DIR="$BUILD_DIR/control"
mkdir -p "$DATA_DIR/usr/bin"
mkdir -p "$DATA_DIR/etc/init.d"
mkdir -p "$DATA_DIR/etc/config"
mkdir -p "$DATA_DIR/usr/lib/lua/luci/controller"
mkdir -p "$DATA_DIR/usr/lib/lua/luci/model/cbi"
mkdir -p "$CTRL_DIR"

cp "$BINARY"                                      "$DATA_DIR/usr/bin/scfddns"
cp packaging/init.d/cloudflare-ddns               "$DATA_DIR/etc/init.d/cloudflare-ddns"
cp packaging/default-config                        "$DATA_DIR/etc/config/cloudflare-ddns"
cp packaging/luci/controller/cloudflare-ddns.lua   "$DATA_DIR/usr/lib/lua/luci/controller/cloudflare-ddns.lua"
cp packaging/luci/model/cbi/cloudflare-ddns.lua    "$DATA_DIR/usr/lib/lua/luci/model/cbi/cloudflare-ddns.lua"
chmod 755 "$DATA_DIR/usr/bin/scfddns"
chmod 755 "$DATA_DIR/etc/init.d/cloudflare-ddns"

sed -e "s/PKGVERSION/$VERSION/g" \
    -e "s/PKGARCH/$ARCH/g" \
    packaging/control > "$CTRL_DIR/control"
cp packaging/postinst "$CTRL_DIR/postinst"
cp packaging/prerm    "$CTRL_DIR/prerm"
chmod 755 "$CTRL_DIR/postinst" "$CTRL_DIR/prerm"

echo "==> 打包 IPK"
echo "2.0" > "$BUILD_DIR/debian-binary"

(cd "$DATA_DIR" && tar czf "$BUILD_DIR/data.tar.gz" .)
(cd "$CTRL_DIR" && tar czf "$BUILD_DIR/control.tar.gz" .)

ar r "$PKG_FILE" \
  "$BUILD_DIR/debian-binary" \
  "$BUILD_DIR/control.tar.gz" \
  "$BUILD_DIR/data.tar.gz"

echo "==> 完成: $PKG_FILE"
