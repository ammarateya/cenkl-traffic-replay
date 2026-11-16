#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p work
upstream=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
if [[ ! -d "$upstream/.git" ]]; then
  git clone https://github.com/censoredplanet/obfuscation.git "$upstream"
fi
git -C "$upstream" fetch origin 63caf4199358e697b3dec9fe01a3806a106c4a7a
git -C "$upstream" checkout --detach 63caf4199358e697b3dec9fe01a3806a106c4a7a
cargo build --release --manifest-path "$upstream/cenkl/Cargo.toml"
echo "CENKL ready: $upstream/cenkl/target/release/obfs"
