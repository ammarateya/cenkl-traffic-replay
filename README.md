# CENKL traffic replay client

A Go client for sending dummy bytes according to an ordered flow list, plus a minimal TLS server that discards received bytes. See [DESIGN.md](DESIGN.md) for scope, limitations, and review questions.

## Run with TLS

Requires Go 1.22 or newer and OpenSSL. From the repository root, create a local test certificate:

```sh
mkdir -p work
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1'
```

Start the discard server in one terminal:

```sh
go run ./cmd/cenkl-discard-server -cert work/server-cert.pem -key work/server-key.pem
```

Run the TLS client in a second terminal:

```sh
go run ./cmd/cenkl-replay -input examples/client-only.json -endpoint 127.0.0.1:9000 -tls -ca-cert work/server-cert.pem
```

The client should report 1,500 bytes sent; the server should report 1,500 bytes received. The certificate is trusted only because the client explicitly loads that local certificate. Keep the private key in `work/` and do not commit it.

For a random list, replace `-input examples/client-only.json` with `-random -flows 1 -events 10`. Random mode saves the generated list automatically. Its server-to-client entries are recorded but skipped by this client.

## Run

Choose either the supplied example list or a generated random list.

The original plain TCP mode remains available. With a plain TCP discard service listening on port 9000:

```sh
go run ./cmd/cenkl-replay -input examples/flows.json -endpoint 127.0.0.1:9000
```

To generate sizes and directions, run:

```sh
go run ./cmd/cenkl-replay -random -flows 1 -events 10 -min-size 100 -max-size 1500 -endpoint 127.0.0.1:9000
```

Random mode automatically saves the exact generated directions and sizes to `generated-flows-<seed>.json` in the current directory and prints its path. Use `-save random-flows.json` to choose another filename, or `-seed 42` to reproduce a list. Existing files are never overwritten. The list is saved before connecting, so it remains available if the network run fails. Every flow starts with a client-to-server event so it sends at least some bytes; later directions are chosen randomly. A TCP listener must be running first, for example `nc -l 9000 > /dev/null` in another Terminal window.

The example sends 1,500 bytes on one connection and reports its server-to-client event as unsupported. The TLS discard server receives client data only; it does not replay server-to-client events.

Client flags: choose `-input` or `-random`, `-endpoint` (default `127.0.0.1:9000`), `-timeout` (default `5s`), and use `-tls -ca-cert FILE` together for TLS. Each flow opens a new connection. Failed connections or writes stop the run with a nonzero exit status.

```sh
go test ./...
```

## Input

See [`examples/flows.json`](examples/flows.json). `size` is the number of dummy application bytes to submit for a client-to-server event, not a promise of TCP packet or TLS record size. Server-to-client events are listed for review and reported, but no response is requested or read in this stage. No timing is modeled.
