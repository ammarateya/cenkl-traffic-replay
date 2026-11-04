# CENKL traffic replay: client MVP design

## Goal and boundary

Provide a small, reproducible Go client that converts an ordered list of observed flow sizes and directions into dummy application bytes sent to a configurable TCP endpoint. This is a first stage for exploring whether replaying recorded traffic shapes can shorten the CENKL traffic generation cycle. It is independent of any obfuscation tool and does not contact original destinations.

This repository implements the client and a minimal TLS discard server for local testing. It does not record ISP traffic, parse packet captures, implement a proxy, run Zeek/CENKL analysis, or provide developer diagnostics. Those belong to the longer term CenKL-as-a-Service vision, not this MVP.

## Input contract

`examples/flows.json` defines the initial JSON schema. The client can also generate synthetic flows with `-random`, choosing sizes from a configured inclusive range and directions at random after an initial client-to-server event. A seed makes the list reproducible. Random mode writes the generated list to a new JSON file before connecting; `-save` selects the path. Existing files are not overwritten. Each flow has a nonempty ID and an ordered, nonempty `events` list. Each event has a `direction` (`client_to_server` or `server_to_client`) and positive `size` in bytes. Sizes over 16 MiB are rejected to catch malformed data. Each flow creates a separate TCP connection, processed sequentially. The input should contain only metadata derived from traces; no payloads, hostnames, IP addresses, or timestamps are needed.

A future trace conversion step must specify what size means. CenKL models the TCP payload bytes in each data-carrying packet belonging to a TLS connection. This MVP treats `size` as dummy application bytes to write, so it has not yet translated a target TCP payload size into an application write size after TLS overhead. The schema deliberately avoids claiming that a write recreates a captured packet.

## Client behavior

For each `client_to_server` event, write exactly `size` zero bytes to the connection, retrying partial writes. The client can use plain TCP or TLS, with TLS requiring a trusted server certificate. A minimal TLS server accepts the connection, reads the data, discards it, and reports bytes received. For `server_to_client` events, the client counts and reports an unsupported event, with no read or synthetic response. The client closes each connection after its events. It prints sent byte counts and unsupported event counts per flow. Connection and write deadlines are configurable. Invalid input and network errors exit nonzero; a network failure stops subsequent flows.

The order of client writes follows the event list, but skipped server events do not form synchronization barriers. A compatible peer and an interaction protocol are needed before replaying mixed bidirectional sequences faithfully. In particular, this client cannot make the remote endpoint send specified response sizes. The endpoint must be a service the operator controls that accepts arbitrary TCP bytes; an existing application protocol may close the connection or interpret the dummy bytes unexpectedly.

## What the network can actually reproduce

A Go `Write` is an application stream operation. TCP may split or coalesce writes into segments; retransmission, congestion control, buffering, offload, and the path change observed packet sizes. In TLS mode, the Go TLS library encrypts and frames the dummy bytes, adding TLS overhead. The client guarantees application bytes submitted per event, not segment boundaries, exact TLS record sizes, on-wire packet sizes, or timing. Comparing a packet capture against the requested sizes is a future validation step.

Random generation is for exercising the client and exploring throughput; it is not a substitute for measured ISP flow distributions.

## Evaluation after this stage

First validate client byte counts against a controlled discard endpoint. Then measure flows per unit time, bytes sent, errors, CPU, memory, and connection concurrency for candidate deployments. Compare one client to one endpoint against multiple clients sharing an endpoint, while keeping peer capacity and network location explicit. An eventual bidirectional peer could add a protocol for response sizes and ordering, followed by optional timing and proxy integration. Trace privacy and retention rules must be settled before ingesting real ISP observations.

## Review decisions

- Confirm whether input `size` is a target TCP payload length from CenKL and define how to account for TLS overhead when aiming at that target.
- Decide whether server-to-client events should be skipped with a warning (current behavior) or reject a flow until the peer exists.
- Define the minimum useful accuracy metric for the first stage: bytes accepted by a controlled endpoint, captured TCP payload volume, or something else.
