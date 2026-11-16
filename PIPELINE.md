# Proxy-link capture and CENKL comparison

The path is **saved flow JSON → synchronized Go TLS client/server → selected proxy client/server → capture on the proxy server's network interface → Zeek → CENKL size/direction model comparison**. The replay server verifies the *inner* TLS records. The PCAP and CENKL tools examine the *outer* proxy TCP connection, which can have different packet sizes and directions. The Go programs send dummy bytes. Input timestamps are retained as metadata, but replay does not reproduce timing.

## Reproduce a small private-data run

Requirements: Go, Docker Desktop, OpenSSL, Rust/Cargo, Git, and roughly 1 GB of space for the CENKL build. The private log stays on your machine and is never committed.

```sh
scripts/setup-cenkl.sh
go run ./cmd/cenkl-private-batch -input /absolute/path/to/records.00_00_00-04_00_00.log -output work/private-1000.json -limit 1000
scripts/run-proxy.sh once work/private-1000.json
scripts/analyze-cenkl.sh work/proxy-run-YYYYMMDD-HHMMSS-PID
```

Replace the last path with the directory printed by `run-proxy.sh`. The batch importer selects the first 1,000 distinct connection IDs, scans the entire log so interleaved connections remain whole, maps direction `1` to client→server and `0` to server→client, and adds five bytes to Aaron's stated record lengths for the TLS header. A connection with a missing index or unsupported size is excluded in full, so the result can contain fewer than 1,000 connections. Use `-skip N` for another batch. This format is private TLS-record data; do not add five bytes to CESNET packet lengths.

The run directory contains `journal.tsv`, per-batch client/server/proxy logs, the input copy/path, and `proxy-link.pcap`. The analysis adds `obfuscation.log`, `packets.log`, `flows.bin`, `proxy-hist.bin`, `divergence-delta.json`, and `divergence-cumulative.json`. Zeek may reject some outer connections; compare its accepted count to the replay journal. A replay verification is not a CENKL acceptance or a scientific result.

The analysis uses the official CENKL Zeek script and published order-1 size/direction histogram from the pinned upstream checkout. It strips TLS handshake traffic, requires at least ten data packets, uses pseudocount zero, and computes proxy-vs-reference KL divergence. The JSON arrays report divergence by packet position, not a single pass/fail verdict. The analysis excludes outer flows whose measured RTT is zero or negative; the upstream normalizer divides by RTT even when timestamp is masked, and would otherwise fail on a local capture. At least 100 accepted outer flows are required by default. Use sufficiently large and representative samples before interpreting the numbers. The capture sidecar disables TCP segmentation and receive offloads on the proxy interfaces so its TCP segment sizes are meaningful in this Docker testbed.

## Proxy configurations

The runners reuse pinned configurations from the [CENKL upstream repository](https://github.com/censoredplanet/obfuscation). A small flow has been verified through each configuration. GOST and five Xray modes create a separate outer connection for each inner flow:

```sh
scripts/run-proxy.sh once work/private-1000.json
scripts/run-xray.sh vmess work/private-1000.json
scripts/run-xray.sh vless work/private-1000.json
scripts/run-xray.sh trojan work/private-1000.json
scripts/run-xray.sh vless-ws work/private-1000.json
scripts/run-xray.sh vision work/private-1000.json
```

NaiveProxy and Xray gRPC reuse an outer connection across many inner flows. For a CENKL per-connection comparison, use the per-flow runner, which restarts the proxy client process between flows and keeps the capture running:

```sh
scripts/run-per-flow.sh naive work/private-1000.json
scripts/run-per-flow.sh grpc work/private-1000.json
```

The WebTunnel/Tor configuration has a separate runner, `scripts/run-webtunnel.sh`. It connects the dummy TLS server through a local onion service and captures the bridge link. It has been tested with small batches; its large-run cost should be measured separately. Each runner prints its result directory; then pass that directory to `scripts/analyze-cenkl.sh`. The CENKL script checks **outer** flow count because replaying thousands of inner connections through one multiplexed outer connection gives only one CENKL sample.

## What remains

CESNET TLS-Year22 supplies packet-level flow sizes/directions, not TLS record sizes. Mapping each packet length to a requested TLS record is a surrogate workload; it will not recreate the original CESNET TLS stream or timing. The included converter reads the first **up to 30 packets per original connection**, discards any entire connection with a size outside 23–16,406 bytes, maps `+1` to client→server and `-1` to server→client, and does not add five bytes. Its sampling/filter policy needs validation before a comparison can be interpreted.

For a bounded CESNET-TLS-Year22 XS sample (requires Python and `pip install tables`):

```sh
python3 scripts/import-cesnet.py --input /absolute/path/to/CESNET-TLS-Year22-XS.h5 --output work/cesnet-1000.json --limit 1000
scripts/run-proxy.sh once work/cesnet-1000.json
scripts/analyze-cenkl.sh work/proxy-run-YYYYMMDD-HHMMSS-PID
```

For long runs, process bounded batches and keep summary outputs; PCAP and per-flow logs grow quickly. Do not extrapolate a one-hour synthetic random run into a result about real traffic.
