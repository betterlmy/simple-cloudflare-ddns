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
mkdir -p "$DATA_DIR/usr/sbin"
mkdir -p "$DATA_DIR/etc/init.d"
mkdir -p "$DATA_DIR/etc/config"
mkdir -p "$DATA_DIR/usr/lib/lua/luci/controller"
mkdir -p "$DATA_DIR/usr/lib/lua/luci/model/cbi"
mkdir -p "$DATA_DIR/usr/lib/lua/luci/view/cloudflare-ddns"
mkdir -p "$CTRL_DIR"

cp "$BINARY"                                         "$DATA_DIR/usr/bin/scfddns"
cp packaging/cloudflare-ddns-wrapper                 "$DATA_DIR/usr/sbin/cloudflare-ddns-wrapper"
cp packaging/cloudflare-ddns-ctl                     "$DATA_DIR/usr/sbin/cloudflare-ddns-ctl"
cp packaging/init.d/cloudflare-ddns                  "$DATA_DIR/etc/init.d/cloudflare-ddns"
cp packaging/default-config                           "$DATA_DIR/etc/config/cloudflare-ddns"
cp packaging/luci/controller/cloudflare-ddns.lua      "$DATA_DIR/usr/lib/lua/luci/controller/cloudflare-ddns.lua"
cp packaging/luci/model/cbi/cloudflare-ddns.lua       "$DATA_DIR/usr/lib/lua/luci/model/cbi/cloudflare-ddns.lua"
cp packaging/luci/view/cloudflare-ddns/log.htm        "$DATA_DIR/usr/lib/lua/luci/view/cloudflare-ddns/log.htm"
chmod 755 "$DATA_DIR/usr/bin/scfddns"
chmod 755 "$DATA_DIR/usr/sbin/cloudflare-ddns-wrapper"
chmod 755 "$DATA_DIR/usr/sbin/cloudflare-ddns-ctl"
chmod 755 "$DATA_DIR/etc/init.d/cloudflare-ddns"

sed -e "s/PKGVERSION/$VERSION/g" \
    -e "s/PKGARCH/$ARCH/g" \
    packaging/control > "$CTRL_DIR/control"
cp packaging/postinst  "$CTRL_DIR/postinst"
cp packaging/prerm     "$CTRL_DIR/prerm"
cp packaging/conffiles "$CTRL_DIR/conffiles"
chmod 755 "$CTRL_DIR/postinst" "$CTRL_DIR/prerm"

echo "==> 打包 IPK"

# iStoreOS/OpenWrt IPK 格式：gzip(tar(./debian-binary, ./data.tar.gz, ./control.tar.gz))
# 用 Python 全程构建，避免 macOS tar/gzip 引入系统特有扩展
python3 - "$PKG_FILE" "$DATA_DIR" "$CTRL_DIR" <<'PYEOF'
import sys, os, io, gzip, tarfile, stat

pkg_file = sys.argv[1]
data_dir = sys.argv[2]
ctrl_dir = sys.argv[3]

def make_targz(src_dir):
    buf = io.BytesIO()
    with gzip.GzipFile(fileobj=buf, mode='wb', mtime=0) as gz:
        with tarfile.open(fileobj=gz, mode='w|') as tar:
            for root, dirs, files in os.walk(src_dir):
                dirs.sort()
                rel_root = os.path.relpath(root, src_dir)
                arc_dir = '.' if rel_root == '.' else './' + rel_root
                if rel_root != '.':
                    info = tarfile.TarInfo(name=arc_dir)
                    info.type  = tarfile.DIRTYPE
                    info.mode  = 0o755
                    info.mtime = 0
                    tar.addfile(info)
                for fname in sorted(files):
                    fpath = os.path.join(root, fname)
                    info = tarfile.TarInfo(name=arc_dir + '/' + fname)
                    info.size  = os.path.getsize(fpath)
                    info.mtime = 0
                    info.mode  = stat.S_IMODE(os.stat(fpath).st_mode)
                    with open(fpath, 'rb') as f:
                        tar.addfile(info, f)
    return buf.getvalue()

ctrl_gz = make_targz(ctrl_dir)
data_gz = make_targz(data_dir)

# 外层：gzip(tar(...))，成员名加 ./ 前缀
outer = io.BytesIO()
with gzip.GzipFile(fileobj=outer, mode='wb', mtime=0) as gz:
    with tarfile.open(fileobj=gz, mode='w|') as tar:
        for name, data in [
            ('./debian-binary', b'2.0\n'),
            ('./data.tar.gz',   data_gz),
            ('./control.tar.gz', ctrl_gz),
        ]:
            info = tarfile.TarInfo(name=name)
            info.size  = len(data)
            info.mtime = 0
            info.mode  = 0o644
            tar.addfile(info, io.BytesIO(data))

with open(pkg_file, 'wb') as f:
    f.write(outer.getvalue())

print(f"Created {pkg_file} ({os.path.getsize(pkg_file)} bytes)")
PYEOF

echo "==> 完成: $PKG_FILE"
