#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 2 ]]; then echo 'usage: scripts/run-xray.sh vmess|vless|trojan|vless-ws|vision|grpc flow.json' >&2; exit 2; fi
kind=$1
case "$kind" in
  vmess) export XRAY_SOCKS_PORT=1082 XRAY_SERVER_PORT=10001;;
  vless) export XRAY_SOCKS_PORT=1083 XRAY_SERVER_PORT=10002;;
  trojan) export XRAY_SOCKS_PORT=1084 XRAY_SERVER_PORT=10003;;
  vless-ws) export XRAY_SOCKS_PORT=1085 XRAY_SERVER_PORT=10004;;
  vision) export XRAY_SOCKS_PORT=1087 XRAY_SERVER_PORT=10006;;
  grpc) export XRAY_SOCKS_PORT=1086 XRAY_SERVER_PORT=10005 XRAY_CONFIG_DIR=xray-grpc XRAY_CLIENT_IMAGE=teddysun/xray@sha256:7d7900e4e542a7de929e94bbf8af933ba5cd721e08fbf766e02e422dc403dd5b XRAY_CLIENT_BINARY=/usr/bin/xray;;
  *) echo "unknown Xray configuration: $kind" >&2; exit 2;;
esac
cd "$(dirname "$0")/.."
export CENKL_UPSTREAM=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
[[ -f "$CENKL_UPSTREAM/testbed/xray/client/config.json" ]] || { echo 'run scripts/setup-cenkl.sh first' >&2; exit 1; }
input=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
[[ -f "$input" ]] || { echo "flow not found: $input" >&2; exit 1; }
mkdir -p work "$CENKL_UPSTREAM/testbed/certs"
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server' >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$CENKL_UPSTREAM/testbed/certs/server.key" -out "$CENKL_UPSTREAM/testbed/certs/server.crt" -days 1 -subj '/CN=naiveproxy.lab' -addext 'subjectAltName=DNS:naiveproxy.lab' >/dev/null 2>&1
export TLS_CERT_FILE=./work/server-cert.pem TLS_KEY_FILE=./work/server-key.pem FLOW_FILE="$input"
run_dir="work/proxy-run-xray-$kind-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$run_dir"
export CAPTURE_FILE="$(basename "$run_dir")/proxy-link.pcap"
project="cenkl-xray-$kind-$$"
dc() { docker compose -p "$project" -f compose.yaml -f compose.xray.yaml "$@"; }
cleanup() { dc stop capture >/dev/null 2>&1 || true; dc down >/dev/null 2>&1 || true; echo "Saved results in $run_dir"; }
trap cleanup EXIT
dc up -d --build server xray-server xray-client capture netsetup-client
for attempt in {1..50}; do
  if dc logs --no-color capture 2>/dev/null | grep -q 'listening on eth0'; then break; fi
  sleep 0.2
done
dc logs --no-color capture | grep -q 'listening on eth0' || { echo 'capture failed to start' >&2; exit 1; }
netsetup_id=$(dc ps -a -q netsetup-client)
[[ -n "$netsetup_id" && $(docker inspect -f '{{.State.ExitCode}}' "$netsetup_id") == 0 ]] || { echo 'client network offload setup failed' >&2; exit 1; }
status=0
dc run --rm client > "$run_dir/client.log" 2>&1 || status=$?
dc logs --no-color server > "$run_dir/server.log"
dc logs --no-color xray-client xray-server > "$run_dir/proxy.log"
dc stop capture >/dev/null
expected=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["flows"]))' "$input")
observed=$(grep 'verified .* TLS records' "$run_dir/server.log" | sed -E 's/.*\| (.*): verified .*/\1/' | sort -u | wc -l | tr -d ' ')
echo "$observed/$expected connections verified; capture $run_dir/proxy-link.pcap"
if (( status != 0 || observed != expected )); then echo "inspect $run_dir/client.log and $run_dir/proxy.log" >&2; exit 1; fi
