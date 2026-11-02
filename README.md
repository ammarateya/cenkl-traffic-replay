# CENKL traffic replay client

A first-stage Go client for sending dummy bytes according to an ordered flow list. See [DESIGN.md](DESIGN.md) for scope, limitations, and review questions.

## Run

Requires Go 1.22 or newer and a TCP endpoint you control that accepts arbitrary bytes. For example, with a discard service listening on port 9000:

```sh
go run ./cmd/cenkl-replay -input examples/flows.json -endpoint 127.0.0.1:9000
```

The example sends 1,500 bytes on one connection and reports its server-to-client event as unsupported. The client does not implement a listening peer.

Flags: `-input` (required), `-endpoint` (default `127.0.0.1:9000`), `-timeout` (default `5s`). Each flow opens a new connection. Failed connections or writes stop the run with a nonzero exit status.

```sh
go test ./...
```

## Input

See [`examples/flows.json`](examples/flows.json). `size` is the number of dummy application bytes to submit for a client-to-server event, not a promise of TCP packet or TLS record size. Server-to-client events are listed for review and reported, but no response is requested or read in this stage. No timing is modeled.
