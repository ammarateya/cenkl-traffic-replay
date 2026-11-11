#!/usr/bin/env bash
set -euo pipefail

mode=${1:-once}
case "$mode" in once|hour|infinite) ;; *) echo 'usage: scripts/run-proxy.sh once|hour|infinite [flow.json]' >&2; exit 2;; esac
if [[ "$mode" != once && ${2:-} != '' ]]; then echo 'a saved flow is supported only in once mode' >&2; exit 2; fi
cd "$(dirname "$0")/.."
mkdir -p work
run_id=$(date +%Y%m%d-%H%M%S)-$$
run_dir="work/proxy-run-$run_id"
mkdir -p "$run_dir"
export CAPTURE_FILE="proxy-run-$run_id/proxy-link.pcap"
project="cenkl-proxy-$$"
files=(-f compose.yaml -f compose.proxy.yaml -f compose.proxy-local.yaml -f compose.proxy-capture.yaml)
dc() { docker compose -p "$project" "${files[@]}" "$@"; }
stop=0
cleanup() {
  dc stop capture >/dev/null 2>&1 || true
  dc down >/dev/null 2>&1 || true
  echo "Saved results in $run_dir"
}
trap 'stop=1' INT TERM
trap cleanup EXIT

openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server' >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/gost-key.pem -out work/gost-cert.pem -days 1 -subj '/CN=gost-server' -addext 'subjectAltName=DNS:gost-server' >/dev/null 2>&1
printf 'time_utc\tbatch\tconnections_verified\tconnections_expected\tstatus\tflow_file\tserver_log\tclient_log\n' > "$run_dir/journal.tsv"
next_input() {
  local id=$1
  if [[ "$mode" == once && ${2:-} != '' ]]; then
    input=${2#./}
    if [[ ! -f "$input" ]]; then echo "flow not found: $input" >&2; exit 1; fi
  else
    input="$run_dir/flow-$id.json"
    go run ./cmd/cenkl-generate -events 100 -output "$input" >/dev/null
  fi
  export FLOW_FILE="./$input"
}
next_input 000001 "${2:-}"
dc up -d --build server gost-server gost-client capture
for attempt in {1..50}; do
  if dc logs --no-color capture 2>/dev/null | grep -q 'listening on eth0'; then break; fi
  sleep 0.2
done
if ! dc logs --no-color capture | grep -q 'listening on eth0'; then echo 'capture did not start' >&2; exit 1; fi

duration=${DURATION_SECONDS:-3600}
if ! [[ "$duration" =~ ^[1-9][0-9]*$ ]]; then echo 'DURATION_SECONDS must be positive' >&2; exit 2; fi
deadline=$((SECONDS + duration))
number=0
verified=0
failed=0
while (( stop == 0 )); do
  if [[ "$mode" == hour ]] && (( SECONDS >= deadline )); then break; fi
  number=$((number + 1))
  id=$(printf '%06d' "$number")
  if (( number > 1 )); then next_input "$id"; dc up -d --force-recreate server >/dev/null; fi
  client_log="$run_dir/client-$id.log"
  server_log="$run_dir/server-$id.log"
  if dc run --rm client > "$client_log" 2>&1; then client_ok=1; else client_ok=0; fi
  dc logs --no-color server > "$server_log"
  dc logs --no-color gost-client gost-server > "$run_dir/gost-$id.log"
  expected=$(grep -c '"id"' "$input")
  observed=$(grep -c 'verified .* TLS records' "$server_log" || true)
  status=failed
  if (( client_ok == 1 && observed == expected )); then status=verified; fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$id" "$observed" "$expected" "$status" "$input" "$server_log" "$client_log" >> "$run_dir/journal.tsv"
  echo "batch $id: $observed/$expected connections verified ($status)"
  verified=$((verified + observed))
  if [[ "$status" != verified ]]; then
    failed=$((failed + expected - observed))
    echo "See $client_log, $server_log, and $run_dir/gost-$id.log" >&2
  fi
  if [[ "$mode" == once ]]; then break; fi
done
dc stop capture >/dev/null
echo "Completed $number batches: $verified connections verified, $failed failed. Capture: $run_dir/proxy-link.pcap"
if [[ "$mode" == once && "$failed" -gt 0 ]]; then exit 1; fi
