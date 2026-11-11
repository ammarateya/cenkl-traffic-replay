# CENKL traffic replay: client MVP design

## Goal and boundary

Provide a small, reproducible Go client that converts an ordered list of observed flow sizes and directions into dummy application bytes sent to a configurable TCP endpoint. This is a first stage for exploring whether replaying recorded traffic shapes can shorten the CENKL traffic generation cycle. It is independent of any obfuscation tool and does not contact original destinations.

This repository implements the client and a minimal TLS server for local testing. The default mode makes both sides follow the same ordered TLS record schedule and verifies full TLS 1.3 record lengths at the server. It does not record ISP traffic, parse packet captures, implement a proxy, run Zeek/CENKL analysis, or provide developer diagnostics. Those belong to the longer term CenKL-as-a-Service vision, not this MVP.

## Input contract

`examples/flows.json` defines the initial JSON schema. The client can also generate synthetic flows with `-random`, choosing sizes from a configured inclusive range and directions at random after an initial client-to-server event. A seed makes the list reproducible. Random mode writes the generated list to a new JSON file before connecting; `-save` selects the path. Existing files are not overwritten. Each flow has a nonempty ID and an ordered, nonempty `events` list. Each event has a `direction` (`client_to_server` or `server_to_client`) and positive `size` in bytes. Sizes over 16 MiB are rejected to catch malformed data. Each flow creates a separate TCP connection, processed sequentially. The input should contain only metadata derived from traces; no payloads, hostnames, IP addresses, or timestamps are needed.

CenKL models the TCP payload bytes in each data-carrying packet belonging to a TLS connection. The default replay mode treats `size` as a full TLS record length. The earlier `-target-segments` mode instead treats it as a target TCP payload length and subtracts TLS overhead from each client event. A packet capture is still required to establish the resulting TCP segment sizes.

In the default mode, `size` means the full encrypted TLS 1.3 record length, including the 5-byte record header and encryption overhead. The client and server read the same ordered list. Each side writes one dummy TLS record on its turns and reads on the other side's turns. The server observes encrypted record framing on the underlying connection and compares each observed length with the corresponding input entry. This mode accepts sizes from 23 through 16,406 bytes. A one-byte readiness signal after the handshake is excluded from the measured schedule.

## Client behavior

In the earlier plain TCP and TLS discard modes, each `client_to_server` event writes `size` zero bytes and each `server_to_client` event is reported as unsupported. In `-tls-records` mode, both directions are replayed as described above. The client opens one connection per flow and validates the server certificate. Invalid input and network errors exit nonzero; a network failure stops subsequent flows.

In `-tls-records` mode, both peers use the same schedule; a read at one end waits for the corresponding write at the other end. Consecutive writes in the same direction follow the input order. This works with the included server, not with an arbitrary TLS endpoint that does not know the schedule.

## What the network can actually reproduce

A Go `Write` is an application stream operation. TCP may split or coalesce writes into segments; retransmission, congestion control, buffering, offload, and the path change observed packet sizes. In TLS target mode, the client controls the TLS 1.3 record size as a way to aim at a TCP payload length, but TCP still decides segment boundaries. The README shows how to capture the local TLS connection and list its observed client-to-server TCP payload sizes. Those measurements must be compared with the requested flow before claiming segment-size fidelity. No timing is modeled.

Random generation is for exercising the client and exploring throughput; it is not a substitute for measured ISP flow distributions.

## Evaluation after this stage

First validate client byte counts against a controlled discard endpoint. Then measure flows per unit time, bytes sent, errors, CPU, memory, and connection concurrency for candidate deployments. Compare one client to one endpoint against multiple clients sharing an endpoint, while keeping peer capacity and network location explicit. An eventual bidirectional peer could add a protocol for response sizes and ordering, followed by optional timing and proxy integration. Trace privacy and retention rules must be settled before ingesting real ISP observations.

## Review decisions

- Confirm whether input `size` is a target TCP payload length from CenKL and define how to account for TLS overhead when aiming at that target.
- Decide whether server-to-client events should be skipped with a warning (current behavior) or reject a flow until the peer exists.
- Define the minimum useful accuracy metric for the first stage: bytes accepted by a controlled endpoint, captured TCP payload volume, or something else.
