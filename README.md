# CENKL traffic replay client

A first-stage Go client for sending dummy bytes according to an ordered flow list. See [DESIGN.md](DESIGN.md) for scope, limitations, and review questions.

## Run

Choose either the supplied example list or a generated random list.

Requires Go 1.22 or newer and a TCP endpoint you control that accepts arbitrary bytes. For example, with a discard service listening on port 9000:

```sh
go run ./cmd/cenkl-replay -input examples/flows.json -endpoint 127.0.0.1:9000
```

To generate sizes and directions, run:

```sh
go run ./cmd/cenkl-replay -random -flows 1 -events 10 -min-size 100 -max-size 1500 -endpoint 127.0.0.1:9000
```

Add `-save random-flows.json` to keep the exact generated list, or `-seed 42` to reproduce it later. The generated seed is printed. Every flow starts with a client-to-server event so it sends at least some bytes; later directions are chosen randomly. A TCP listener must be running first, for example `nc -l 9000 > /dev/null` in another Terminal window.

The example sends 1,500 bytes on one connection and reports its server-to-client event as unsupported. The client does not implement a listening peer.

Flags: choose `-input` or `-random`, `-endpoint` (default `127.0.0.1:9000`), `-timeout` (default `5s`). Each flow opens a new connection. Failed connections or writes stop the run with a nonzero exit status.

```sh
go test ./...
```

## Input

See [`examples/flows.json`](examples/flows.json). `size` is the number of dummy application bytes to submit for a client-to-server event, not a promise of TCP packet or TLS record size. Server-to-client events are listed for review and reported, but no response is requested or read in this stage. No timing is modeled.
