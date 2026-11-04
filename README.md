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
go run ./cmd/cenkl-replay -input examples/client-only.json -endpoint 127.0.0.1:9000 -tls -ca-cert work/server-cert.pem -target-segments
```

The two input sizes, 1,000 and 500, are now **target TCP payload sizes**. In this mode the client uses TLS 1.3 and subtracts 22 bytes of TLS record overhead from each event. It should report 1,456 dummy application bytes sent with a target of 1,500 TCP payload bytes; the server should report 1,456 bytes received. The certificate is trusted only because the client explicitly loads that local certificate. Keep the private key in `work/` and do not commit it.

## Check TCP segment sizes

CenKL measures the number of TCP payload bytes in each captured data packet. To inspect those sizes in this local setup, run a packet capture in a third terminal **before** running the TLS client:

```sh
sudo tcpdump -i lo0 -s 0 -w work/tls-capture.pcap 'tcp port 9000'
```

Run the single-flow TLS client command above, then stop `tcpdump` with Ctrl+C. Show the client-to-server segments and their TCP payload sizes:

```sh
./scripts/show-client-segments.sh work/tls-capture.pcap
```

The output is `capture-time payload-bytes` for every data-carrying client segment. The first entries include the TLS handshake; the later entries carry the dummy application data. In `-target-segments` mode, look for target sizes of 1,000 and 500 among those later entries. If they differ, the packet capture is the result to trust: TCP may split or combine records. This capture step requires permission to capture on the loopback interface. The helper reads the capture; it does not alter packets.

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

Client flags: choose `-input` or `-random`, `-endpoint` (default `127.0.0.1:9000`), `-timeout` (default `5s`), and use `-tls -ca-cert FILE` together for TLS. Add `-target-segments` to interpret each input size as a desired TCP payload length; without it, each size means application bytes. Target mode supports client event sizes from 23 through 16,406 bytes and sends one TLS 1.3 record per event. Each flow opens a new connection. Failed connections or writes stop the run with a nonzero exit status.

```sh
go test ./...
```

## Input

See [`examples/flows.json`](examples/flows.json). The meaning of `size` depends on the mode: application bytes by default, target TCP payload bytes with `-target-segments`. A target is not proof of the observed packet size; use the capture step above to check. Server-to-client events are listed for review and reported, but no response is requested or read in this stage. No timing is modeled.
