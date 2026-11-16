# Proxy-link capture and CENKL comparison

The implemented path is **saved flow JSON → synchronized Go TLS client/server → GOST client/server → capture on the GOST server's network interface → Zeek → CENKL size/direction model comparison**. The replay server verifies the *inner* TLS records. The PCAP and CENKL tools examine the *outer* proxy TCP connection, which can have different packet sizes and directions. The Go programs send dummy bytes. Input timestamps are retained as metadata, but replay does not reproduce timing.

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

The analysis uses the official CENKL Zeek script and published order-1 size/direction histogram from the pinned upstream checkout. It strips TLS handshake traffic, requires at least ten data packets, uses pseudocount zero, and computes proxy-vs-reference KL divergence. The JSON arrays report divergence by packet position, not a single pass/fail verdict. Use sufficiently large and representative samples before interpreting the numbers. The capture sidecar disables TCP segmentation and receive offloads on both GOST interfaces so its TCP segment sizes are meaningful in this Docker testbed.

## What remains

This runner is verified with **GOST only**. The upstream CENKL repository contains nine configurations, including Xray, NaiveProxy, and WebTunnel; connecting each to this runner and validating its outer-link capture remains work. CESNET TLS-Year22 supplies packet-level flow sizes/directions, not TLS record sizes. Mapping each packet length to a requested TLS record is a surrogate workload; it will not recreate the original CESNET TLS stream or timing. A CESNET importer and its sampling/filter policy need to be validated before a comparison can be interpreted.

For long runs, process bounded batches and keep summary outputs; PCAP and per-flow logs grow quickly. Do not extrapolate a one-hour synthetic random run into a result about real traffic.
