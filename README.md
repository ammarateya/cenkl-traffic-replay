# CenKL TLS record replay

This prototype makes two Go programs exchange **dummy data in an ordered list of TLS record sizes and directions**. Both programs read the same flow file. The client sends at client-to-server entries; the server sends at server-to-client entries. At each receiving turn, the program waits for the requested data before moving to the next entry. The server measures the encrypted TLS record length on its connection and reports whether each entry matched.

This is a local record-size test. It does not run a developer proxy, reproduce packet timing, or guarantee TCP segment sizes.

## Demo page

With Docker Desktop running, start the local demo from this repository's root:

```sh
go run ./cmd/cenkl-demo
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080). Choose 20, 100, or 500 events and click **Generate and run**. The page saves a random flow under `work/`, runs the two containers, and compares every server log entry with the saved directions and sizes. It shows the match count, a button to view server logs, and a link to download the saved flow. The demo runs only on your computer and uses a dedicated Docker Compose project that it cleans up after each run. The random flow is for demonstration, not a substitute for CenKL's traffic model.

## Run on one computer

You need Go 1.22 or newer and OpenSSL. From the repository root, create a local test certificate:

```sh
mkdir -p work
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1'
```

In Terminal 1, start the server:

```sh
go run ./cmd/cenkl-server
```

In Terminal 2, run the client:

```sh
go run ./cmd/cenkl-replay
```

Both use [`examples/flows.json`](examples/flows.json) by default. The example requests a 1,000-byte client record, a 500-byte client record, then a 1,400-byte server record. The server prints `expected_record` and `observed_record` for each entry and exits after the flow finishes. The certificate files stay in `work/`, which Git ignores.

To run the **two-container version**, follow [DOCKER.md](DOCKER.md). The containers communicate through Docker's private network on port 9000; the port is not published to the host.

## Use another flow list

Pass the **same JSON file to both programs** with `-input PATH`. One flow opens one TLS connection. Events are processed in list order:

```json
{
  "flows": [
    {
      "id": "example",
      "events": [
        {"direction": "client_to_server", "size": 1000},
        {"direction": "server_to_client", "size": 100},
        {"direction": "client_to_server", "size": 500}
      ]
    }
  ]
}
```

`size` is the **full encrypted TLS 1.3 record length**, including its 5-byte header and encryption overhead. The supported range is 23 through 16,406 bytes. The programs subtract 22 bytes of TLS overhead to determine how many dummy data bytes to send for each record. A one-byte readiness signal is sent after the handshake so the server starts measuring before the first scheduled record; that signal is excluded from the comparison log but would appear in a full network capture. It must be accounted for before using this setup to evaluate a proxy with CenKL.

The optional [CenKL CSV importer](CENKL_IMPORT.md) converts signed size/direction CSV rows into this JSON format. CenKL's generated sizes are based on **TCP packet payloads**, so treating those numbers as TLS record sizes is only a provisional mapping. It needs review before using imported flows as research data.

## Verify the code

```sh
go test ./...
```

See [DESIGN.md](DESIGN.md) for the design and remaining research decisions.
