#!/usr/bin/env bash
# Windows 资源优化测量脚本（Linux/macOS 也可跑）
# 用 cargo-bloat 分析 release 二进制，找出体积/编译时间最大的 crate
# 用法：
#   bash scripts/measure-size.sh
#   bash scripts/measure-size.sh 1.0.0
set -e
cd "$(dirname "$0")/.."

TAG="${1:-dev}"
OUT="target/measure"
mkdir -p "$OUT"

echo "==> building release..."
cargo build --release --target-dir target

BIN=$(find target/release -maxdepth 1 -name 'linkall-controlled-win*' -type f | head -1)
if [ -z "$BIN" ]; then
  echo "no binary found in target/release/"
  exit 1
fi

# Size breakdown
echo "==> size breakdown..."
ls -lh "$BIN" | tee "$OUT/size-$TAG.txt"
size "$BIN" 2>/dev/null | tee -a "$OUT/size-$TAG.txt" || true

# Sections (Windows 资源 dump)
if command -v objdump >/dev/null 2>&1; then
  objdump -h "$BIN" 2>/dev/null | head -30 | tee -a "$OUT/size-$TAG.txt" || true
fi

# cargo-bloat analysis
if command -v cargo-bloat >/dev/null 2>&1; then
  echo "==> cargo-bloat analysis..."
  cargo bloat --release --target-dir target -n 30 2>/dev/null | tee "$OUT/bloat-$TAG.txt" || true
  cargo bloat --release --target-dir target --crates -n 30 2>/dev/null | tee "$OUT/bloat-crates-$TAG.txt" || true
else
  echo "cargo-bloat not installed. Install with: cargo install cargo-bloat --features=analyze-bloat"
fi

# Windows DLL dependency check
if command -v dumpbin >/dev/null 2>&1; then
  echo "==> DLL dependencies..."
  dumpbin /dependents "$BIN" | tee "$OUT/dlls-$TAG.txt"
fi

echo ""
echo "Done. Results in $OUT/"
ls -la "$OUT/"
