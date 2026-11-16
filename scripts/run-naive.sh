#!/usr/bin/env bash
set -euo pipefail
if [[ $# != 1 ]]; then echo 'usage: scripts/run-naive.sh flow.json' >&2; exit 2; fi
cd "$(dirname "$0")/.."
export CENKL_UPSTREAM=${CENKL_UPSTREAM:-"$PWD/work/cenkl-upstream"}
[[ -f "$CENKL_UPSTREAM/testbed/naiveproxy/client/config.json" ]] || { echo 'run scripts/setup-cenkl.sh first' >&2; exit 1; }
input=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
[[ -f "$input" ]] || { echo "flow not found: $input" >&2; exit 1; }
mkdir -p work "$CENKL_UPSTREAM/testbed/certs"
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server' >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$CENKL_UPSTREAM/testbed/certs/rootCA.key" -out "$CENKL_UPSTREAM/testbed/certs/rootCA.pem" -days 2 -subj '/CN=CenKL local test CA' -addext 'basicConstraints=critical,CA:TRUE' >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -keyout "$CENKL_UPSTREAM/testbed/certs/server.key" -out "$CENKL_UPSTREAM/testbed/certs/server.csr" -subj '/CN=proxy.lab' >/dev/null 2>&1
printf 'subjectAltName=DNS:proxy.lab\n' > "$CENKL_UPSTREAM/testbed/certs/server.ext"
openssl x509 -req -in "$CENKL_UPSTREAM/testbed/certs/server.csr" -CA "$CENKL_UPSTREAM/testbed/certs/rootCA.pem" -CAkey "$CENKL_UPSTREAM/testbed/certs/rootCA.key" -CAcreateserial -out "$CENKL_UPSTREAM/testbed/certs/server.crt" -days 1 -extfile "$CENKL_UPSTREAM/testbed/certs/server.ext" >/dev/null 2>&1
cp "$CENKL_UPSTREAM/testbed/certs/rootCA.pem" "$CENKL_UPSTREAM/testbed/naiveproxy/client/rootCA.pem"
cat > work/naive-Caddyfile <<'EOF'
{
  order forward_proxy before file_server
}
:443 {
  tls /etc/naiveproxy/server.crt /etc/naiveproxy/server.key
  forward_proxy {
    basic_auth user test
    acl {
      allow server
    }
    hide_ip
    hide_via
    probe_resistance
  }
}
EOF
cat > "$CENKL_UPSTREAM/testbed/naiveproxy/client/Dockerfile.replay" <<'EOF'
FROM pocat/naiveproxy@sha256:ae6dff0ca7bef5d9228ec85f366038f55d430c61245b8295c0479e351f5b0fbf
USER root
RUN printf 'deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/20250908T000000Z/ bullseye main\n\
deb [check-valid-until=no] https://snapshot.debian.org/archive/debian-security/20250908T000000Z/ bullseye-security main\n' > /etc/apt/sources.list \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates libnss3-tools \
 && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /root/.pki/nssdb && chmod 700 /root/.pki/nssdb && certutil -d sql:/root/.pki/nssdb -N --empty-password
COPY rootCA.pem /tmp/root.crt
RUN certutil -d sql:/root/.pki/nssdb -A -t "C,," -n "NaiveRoot" -i /tmp/root.crt \
 && cp /tmp/root.crt /usr/local/share/ca-certificates/naive-root.crt \
 && update-ca-certificates
EOF
export TLS_CERT_FILE=./work/server-cert.pem TLS_KEY_FILE=./work/server-key.pem FLOW_FILE="$input"
run_dir="work/proxy-run-naive-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$run_dir"
export CAPTURE_FILE="$(basename "$run_dir")/proxy-link.pcap"
project="cenkl-naive-$$"
dc() { docker compose -p "$project" -f compose.yaml -f compose.naive.yaml "$@"; }
cleanup() { dc stop capture >/dev/null 2>&1 || true; dc down >/dev/null 2>&1 || true; echo "Saved results in $run_dir"; }
trap cleanup EXIT
dc up -d --build server naive-server naive-client capture netsetup-client
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
dc logs --no-color naive-client naive-server > "$run_dir/proxy.log"
dc stop capture >/dev/null
expected=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["flows"]))' "$input")
observed=$(grep 'verified .* TLS records' "$run_dir/server.log" | sed -E 's/.*\| (.*): verified .*/\1/' | sort -u | wc -l | tr -d ' ')
echo "$observed/$expected connections verified; capture $run_dir/proxy-link.pcap"
if (( status != 0 || observed != expected )); then echo "inspect $run_dir/client.log and $run_dir/proxy.log" >&2; exit 1; fi
