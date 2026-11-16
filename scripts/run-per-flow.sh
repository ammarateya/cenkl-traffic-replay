#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 2 ]]; then echo 'usage: scripts/run-per-flow.sh naive|grpc flow.json' >&2; exit 2; fi
kind=$1
cd "$(dirname "$0")/.."
input=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
[[ -f "$input" ]] || { echo "flow not found: $input" >&2; exit 1; }
export CENKL_UPSTREAM=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
case "$kind" in
  naive)
    scripts/run-naive.sh examples/flows.json > work/per-flow-naive-setup.log 2>&1
    files=(-f compose.yaml -f compose.naive.yaml -f compose.per-flow-naive.yaml)
    proxy_client=naive-client proxy_server=naive-server port=1080;;
  grpc)
    scripts/run-xray.sh grpc examples/flows.json > work/per-flow-grpc-setup.log 2>&1
    export XRAY_SOCKS_PORT=1086 XRAY_SERVER_PORT=10005 XRAY_CONFIG_DIR=xray-grpc
    export XRAY_CLIENT_IMAGE=teddysun/xray@sha256:7d7900e4e542a7de929e94bbf8af933ba5cd721e08fbf766e02e422dc403dd5b XRAY_CLIENT_BINARY=/usr/bin/xray
    files=(-f compose.yaml -f compose.xray.yaml -f compose.per-flow-grpc.yaml)
    proxy_client=xray-client proxy_server=xray-server port=1086;;
  *) echo "unknown proxy: $kind" >&2; exit 2;;
esac
go build -o work/cenkl-replay-host ./cmd/cenkl-replay
export TLS_CERT_FILE=./work/server-cert.pem TLS_KEY_FILE=./work/server-key.pem FLOW_FILE="$input"
run_dir="work/proxy-run-per-flow-$kind-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$run_dir"
cp work/server-cert.pem "$run_dir/server-cert.pem"
cp work/server-key.pem "$run_dir/server-key.pem"
export TLS_CERT_FILE="./$run_dir/server-cert.pem" TLS_KEY_FILE="./$run_dir/server-key.pem"
export CAPTURE_FILE="$(basename "$run_dir")/proxy-link.pcap"
project="cenkl-per-flow-$kind-$$"
dc() { docker compose -p "$project" "${files[@]}" "$@"; }
cleanup() { dc stop capture >/dev/null 2>&1 || true; dc down >/dev/null 2>&1 || true; echo "Saved results in $run_dir"; }
trap cleanup EXIT
dc up -d --build server "$proxy_server" "$proxy_client" capture netsetup-client
for attempt in {1..50}; do
  if dc logs --no-color capture 2>/dev/null | grep -q 'listening on eth0'; then break; fi
  sleep 0.2
done
dc logs --no-color capture | grep -q 'listening on eth0' || { echo 'capture did not start' >&2; exit 1; }
container_id=$(dc ps -q "$proxy_client")
[[ -n "$container_id" ]] || { echo 'proxy client did not start' >&2; exit 1; }
status=0
python3 scripts/replay-per-flow.py "$input" "$run_dir" "$container_id" "$port" "$run_dir/server-cert.pem" > "$run_dir/progress.log" 2>&1 || status=$?
dc logs --no-color server > "$run_dir/server.log"
dc logs --no-color "$proxy_client" "$proxy_server" > "$run_dir/proxy.log"
dc stop capture >/dev/null
expected=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["flows"]))' "$input")
observed=$(grep 'verified .* TLS records' "$run_dir/server.log" | sed -E 's/.*\| (.*): verified .*/\1/' | sort -u | wc -l | tr -d ' ')
echo "$observed/$expected distinct connections verified; capture $run_dir/proxy-link.pcap"
if (( status != 0 || observed != expected )); then echo "inspect $run_dir/progress.log and $run_dir/client.log" >&2; exit 1; fi
