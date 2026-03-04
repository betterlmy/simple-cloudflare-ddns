#!/usr/bin/env python3
"""
make_ipk.py <pkg_file> <data_dir> <ctrl_dir>

Builds an iStoreOS/OpenWrt IPK package in the format:
  gzip(tar(./debian-binary, ./data.tar.gz, ./control.tar.gz))
Uses Python to avoid macOS tar/gzip platform-specific extensions.
"""
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
