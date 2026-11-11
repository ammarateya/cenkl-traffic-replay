# First proxy experiment: GOST SOCKS over TLS

This uses the GOST configuration from CENKL's testbed: SOCKS over TLS. The path is:

```text
replay client -> GOST client (TCP forwarder) -> GOST server (SOCKS over TLS) -> replay server
```

The inner replay connection uses TLS 1.3. GOST adds a second, outer TLS connection between its two containers. The replay server checks the **inner** TLS record sizes and directions. GOST logs prove that it routed the connection. This experiment does not yet capture or evaluate the outer TCP packets that CenKL would observe.

## Run

From the repository root, with Docker Desktop running, create certificates for both TLS connections:

```sh
mkdir -p work
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server'
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/gost-key.pem -out work/gost-cert.pem -days 1 -subj '/CN=gost-server' -addext 'subjectAltName=DNS:gost-server'
docker compose -p cenkl-gost -f compose.yaml -f compose.proxy.yaml up -d --build server gost-server gost-client
docker compose -p cenkl-gost -f compose.yaml -f compose.proxy.yaml run --rm client
docker compose -p cenkl-gost -f compose.yaml -f compose.proxy.yaml logs server gost-client gost-server
docker compose -p cenkl-gost -f compose.yaml -f compose.proxy.yaml down
```

Look for `verified 3 TLS records` in the replay server log and a `connect` to `server:9000` in the GOST server log. To use another JSON flow, set `FLOW_FILE=./path/to/flow.json` for **both** the `up` and `run` commands. The replay server exits after the flow and must be recreated before another run. No ports are published on the host.

GOST is pinned to v3.2.6, the version listed in CENKL. The local forwarder listens on port 9001 and sends through GOST's TLS SOCKS listener on port 8443. The replay server listens on port 9000. The client verifies the inner `server` certificate despite connecting to the local forwarder's name; GOST verifies the outer `gost-server` certificate. The SOCKS5 TLS negotiation extension is disabled because the outer listener already supplies TLS. This is a single hop without multiplexing. It is a local test configuration, with no SOCKS authentication or public listener.

GOST configuration references: [SOCKS over TLS](https://gost.run/en/tutorials/protocols/socks/), [TCP forwarding through a SOCKS chain](https://www.gost.run/en/tutorials/port-forwarding/), [TLS certificate options](https://latest.gost.run/en/tutorials/tls/).

## Put the app beside the installed proxy client

Add `compose.proxy-local.yaml` to each Compose command, after the other two files. This makes the replay client share the GOST client container's **network namespace** and connect to `127.0.0.1:9001`. The programs remain separate, but their connection uses loopback, like an app talking to locally installed proxy software.

```sh
docker compose -p cenkl-gost-local -f compose.yaml -f compose.proxy.yaml -f compose.proxy-local.yaml up -d --build server gost-server gost-client
docker compose -p cenkl-gost-local -f compose.yaml -f compose.proxy.yaml -f compose.proxy-local.yaml run --rm client
docker compose -p cenkl-gost-local -f compose.yaml -f compose.proxy.yaml -f compose.proxy-local.yaml logs server gost-client gost-server
docker compose -p cenkl-gost-local -f compose.yaml -f compose.proxy.yaml -f compose.proxy-local.yaml down
```

The GOST-to-GOST connection remains a separate Docker-network connection. Sharing the namespace only changes the app-to-local-proxy hop; it does not make the dummy app part of GOST or establish that the outer packets match real user traffic.
