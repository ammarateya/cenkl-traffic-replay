# CENKL traffic replay: client MVP design

## Goal and boundary

Provide a small, reproducible Go client that converts an ordered list of observed flow sizes and directions into dummy application bytes sent to a configurable TCP endpoint. This is a first stage for exploring whether replaying recorded traffic shapes can shorten the CENKL traffic generation cycle. It is independent of any obfuscation tool and does not contact original destinations.

This repository implements only the client. It does not record ISP traffic, parse packet captures, speak TLS, implement a server or proxy, run Zeek/CENKL analysis, or provide developer diagnostics. Those belong to the longer term CenKL-as-a-Service vision, not this MVP.

## Input contract

`examples/flows.json` defines the initial JSON schema. The client can also generate synthetic flows with `-random`, choosing sizes from a configured inclusive range and directions at random after an initial client-to-server event. A seed makes the list reproducible, and `-save` writes it for inspection. Each flow has a nonempty ID and an ordered, nonempty `events` list. Each event has a `direction` (`client_to_server` or `server_to_client`) and positive `size` in bytes. Sizes over 16 MiB are rejected to catch malformed data. Each flow creates a separate TCP connection, processed sequentially. The input should contain only metadata derived from traces; no payloads, hostnames, IP addresses, or timestamps are needed.

A future trace conversion step must specify what size means. The recommended source is application payload length or TLS record length after clearly deciding whether the TLS header is included. Raw IP packet length is not interchangeable with either. The schema deliberately avoids claiming that a write recreates a captured packet.

## Client behavior

For each `client_to_server` event, write exactly `size` zero bytes to the connection, retrying partial writes. For `server_to_client` events, count and report an unsupported event, with no read or synthetic response. The client closes each connection after its events. It prints sent byte counts and unsupported event counts per flow. Connection and write deadlines are configurable. Invalid input and network errors exit nonzero; a network failure stops subsequent flows.

The order of client writes follows the event list, but skipped server events do not form synchronization barriers. A compatible peer and an interaction protocol are needed before replaying mixed bidirectional sequences faithfully. In particular, this client cannot make the remote endpoint send specified response sizes. The endpoint must be a service the operator controls that accepts arbitrary TCP bytes; an existing application protocol may close the connection or interpret the dummy bytes unexpectedly.

## What the network can actually reproduce

A Go `Write` is an application stream operation. TCP may split or coalesce writes into segments; retransmission, congestion control, buffering, offload, and the path change observed packet sizes. This client guarantees bytes submitted to TCP per event, not segment boundaries, TLS record boundaries, on-wire packet sizes, or timing. No TLS is emitted: zero bytes are not valid TLS records. With only a plain TCP endpoint, the current output should be treated as a baseline for testing the input and sending path, not a faithful ISP traffic replay.

Random generation is for exercising the client and exploring throughput; it is not a substitute for measured ISP flow distributions.

## Evaluation after this stage

First validate client byte counts against a controlled discard endpoint. Then measure flows per unit time, bytes sent, errors, CPU, memory, and connection concurrency for candidate deployments. Compare one client to one endpoint against multiple clients sharing an endpoint, while keeping peer capacity and network location explicit. An eventual bidirectional peer could add a protocol for response sizes and ordering, followed by optional timing and proxy integration. Trace privacy and retention rules must be settled before ingesting real ISP observations.

## Review decisions

- Confirm the size source: TLS record length, TCP payload length, or another observation.
- Decide whether server-to-client events should be skipped with a warning (current behavior) or reject a flow until the peer exists.
- Define the minimum useful accuracy metric for the first stage: bytes accepted by a controlled endpoint, captured TCP payload volume, or something else.
