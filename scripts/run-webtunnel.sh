#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 1 ]]; then echo 'usage: scripts/run-webtunnel.sh flow.json' >&2; exit 2; fi
cd "$(dirname "$0")/.."
export CENKL_UPSTREAM=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
[[ -f "$CENKL_UPSTREAM/testbed/webtunnel/client/Dockerfile" ]] || { echo 'run scripts/setup-cenkl.sh first' >&2; exit 1; }
input=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
[[ -f "$input" ]] || { echo "flow not found: $input" >&2; exit 1; }
mkdir -p work/webtunnel-certs
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server' >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/webtunnel-certs/rootCA.key -out work/webtunnel-certs/rootCA.pem -days 2 -subj '/CN=CenKL WebTunnel test CA' -addext 'basicConstraints=critical,CA:TRUE' >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -keyout work/webtunnel-certs/server.key -out work/webtunnel-certs/server.csr -subj '/CN=webtunnel-bridge' >/dev/null 2>&1
printf 'subjectAltName=DNS:webtunnel-bridge\n' > work/webtunnel-certs/server.ext
openssl x509 -req -in work/webtunnel-certs/server.csr -CA work/webtunnel-certs/rootCA.pem -CAkey work/webtunnel-certs/rootCA.key -CAcreateserial -out work/webtunnel-certs/server.crt -days 1 -extfile work/webtunnel-certs/server.ext >/dev/null 2>&1
cp work/webtunnel-certs/rootCA.pem "$CENKL_UPSTREAM/testbed/webtunnel/client/rootCA.pem"
docker build -q -t cenkl-webtunnel-client:local "$CENKL_UPSTREAM/testbed/webtunnel/client" >/dev/null
cat > work/onion-torrc <<'EOF'
SocksPort 0
DataDirectory /var/lib/tor
HiddenServiceDir /var/lib/tor/hidden_service
HiddenServicePort 9000 server:9000
Log notice stdout
EOF
run_dir="work/proxy-run-webtunnel-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$run_dir"
export TLS_CERT_FILE=./work/server-cert.pem TLS_KEY_FILE=./work/server-key.pem FLOW_FILE="$input"
export WEBTUNNEL_DESTINATION=placeholder.onion CAPTURE_FILE="$(basename "$run_dir")/proxy-link.pcap"
export WEBTUNNEL_CONCURRENCY=${WEBTUNNEL_CONCURRENCY:-16}
project="cenkl-webtunnel-$$"
dc() { docker compose -p "$project" -f compose.yaml -f compose.webtunnel.yaml "$@"; }
cleanup() { dc stop capture >/dev/null 2>&1 || true; dc down -v >/dev/null 2>&1 || true; echo "Saved results in $run_dir"; }
trap cleanup EXIT
dc up -d --build server webtunnel-server webtunnel-nginx onion-service
fingerprint=''
for attempt in {1..120}; do
  fingerprint=$(dc exec -T webtunnel-server sh -c "awk '{print \$2}' /var/lib/tor/fingerprint" 2>/dev/null || true)
  [[ -n "$fingerprint" ]] && break
  sleep 1
done
[[ -n "$fingerprint" ]] || { echo 'bridge fingerprint not available' >&2; exit 1; }
cat > work/webtunnel-torrc <<EOF
SocksPort 127.0.0.1:9050
Log notice stdout
DataDirectory /var/lib/tor
UseBridges 1
ClientTransportPlugin webtunnel exec /usr/bin/webtunnel-client
Bridge webtunnel 172.28.0.20:443 $fingerprint url=https://webtunnel-bridge/Sup3rS3cr3tPath
EOF
dc up -d webtunnel-client netsetup-client
for attempt in {1..300}; do
  if dc logs --no-color webtunnel-client 2>/dev/null | grep -q 'Bootstrapped 100%'; then break; fi
  sleep 1
done
dc logs --no-color webtunnel-client | grep -q 'Bootstrapped 100%' || { echo 'Tor client did not bootstrap' >&2; exit 1; }
for attempt in {1..120}; do
  WEBTUNNEL_DESTINATION=$(dc exec -T onion-service cat /var/lib/tor/hidden_service/hostname 2>/dev/null | tr -d '\r\n' || true)
  [[ -n "$WEBTUNNEL_DESTINATION" ]] && break
  sleep 1
done
[[ -n "$WEBTUNNEL_DESTINATION" ]] || { echo 'onion destination did not start' >&2; exit 1; }
export WEBTUNNEL_DESTINATION
dc up -d capture
for attempt in {1..50}; do
  if dc logs --no-color capture 2>/dev/null | grep -q 'listening on eth0'; then break; fi
  sleep 0.2
done
dc logs --no-color capture | grep -q 'listening on eth0' || { echo 'capture did not start' >&2; exit 1; }
status=0
dc run --rm client > "$run_dir/client.log" 2>&1 || status=$?
dc logs --no-color server > "$run_dir/server.log"
dc logs --no-color webtunnel-client webtunnel-server webtunnel-nginx onion-service > "$run_dir/proxy.log"
dc stop capture >/dev/null
expected=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["flows"]))' "$input")
observed=$(grep 'verified .* TLS records' "$run_dir/server.log" | sed -E 's/.*\| (.*): verified .*/\1/' | sort -u | wc -l | tr -d ' ')
echo "$observed/$expected distinct connections verified; capture $run_dir/proxy-link.pcap"
if (( status != 0 || observed != expected )); then echo "inspect $run_dir/client.log and $run_dir/proxy.log" >&2; exit 1; fi
