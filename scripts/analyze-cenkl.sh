#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 1 ]]; then echo 'usage: scripts/analyze-cenkl.sh work/proxy-run-.../' >&2; exit 2; fi
cd "$(dirname "$0")/.."
run_dir=$(cd "$1" && pwd)
upstream=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
obfs="$upstream/cenkl/target/release/obfs"
model="$upstream/models/size_direction/markov_order_1"
if [[ ! -x "$obfs" || ! -f "$model/hist.bin" ]]; then
  echo "CENKL checkout or compiled obfs missing at $upstream; see PROXY.md" >&2
  exit 1
fi
if [[ ! -s "$run_dir/proxy-link.pcap" ]]; then echo 'proxy-link.pcap missing or empty' >&2; exit 1; fi

docker run --rm \
  -v "$run_dir:/data" \
  -v "$upstream/cenkl:/cenkl:ro" \
  -w /data zeek/zeek:latest \
  zeek -r /data/proxy-link.pcap /cenkl/obfuscation.zeek > "$run_dir/zeek-output.log" 2>&1
if [[ ! -s "$run_dir/obfuscation.log" || ! -s "$run_dir/packets.log" ]]; then
  echo 'Zeek did not produce usable CENKL logs; inspect zeek-output.log' >&2; exit 1
fi

"$obfs" zeek2flows --flows "$run_dir/obfuscation.log" --packets "$run_dir/packets.log" --output "$run_dir/flows.bin" > "$run_dir/zeek2flows-output.log" 2>&1
cat > "$run_dir/model.toml" <<EOF
[flow]
strip_tls_handshake = true

[flow.source_a.Empirical]
path = "$run_dir/flows.bin"
flow_filter = "tlsDataPackets >= 10"

[model]
model_assumptions = "$model/model_assumptions.json"
pseudocount = 0.0
EOF
"$obfs" pipeline histograms --config "$run_dir/model.toml" --output "$run_dir/proxy-hist.bin" > "$run_dir/model-output.log" 2>&1
"$obfs" histograms divergence --left "$run_dir/proxy-hist.bin" --right "$model/hist.bin" --output "$run_dir/divergence.bin"
"$obfs" divergence delta --input "$run_dir/divergence.bin" > "$run_dir/divergence-delta.json"
"$obfs" divergence cumulative --input "$run_dir/divergence.bin" > "$run_dir/divergence-cumulative.json"
echo "CENKL analysis saved in $run_dir"
echo "Zeek accepted $(($(wc -l < "$run_dir/obfuscation.log") - 1)) outer proxy connections"
