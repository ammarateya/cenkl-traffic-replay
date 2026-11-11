# Two-container TLS record replay

This local experiment runs the dummy TLS client and TLS server in separate containers. Both read the same ordered flow list. For each entry, the named side sends one TLS 1.3 record containing dummy bytes; the other side receives it. The server logs the full encrypted TLS record length, including its 5-byte header and encryption overhead, and compares it with the list. The one-byte readiness signal used to start the exchange is excluded from the comparison.

## Run

From the repository root, create a short-lived local certificate. The `server` name must be in the certificate because the client connects to the server container by that name.

```sh
mkdir -p work
openssl req -x509 -newkey rsa:2048 -nodes -keyout work/server-key.pem -out work/server-cert.pem -days 1 -subj '/CN=server' -addext 'subjectAltName=DNS:server,IP:127.0.0.1'
```

Start the server, run the client, then read the server's comparison log:

```sh
docker compose up -d --build server
docker compose run --rm client
docker compose logs server
```

The example in `examples/flows.json` requests record sizes of 1,000 and 500 bytes from client to server, followed by 1,400 bytes from server to client. The server should print `expected_record` and `observed_record` with matching values for all three events, then `verified 3 TLS records`.

To repeat the test, start the server again. It exits after the listed flows finish. Stop and remove the containers with `docker compose down`.

## Input and limits

The input file is mounted into both containers from `examples/flows.json`. To try another list, edit that file and run the commands again. Each flow opens a separate TLS connection. `size` means the **entire TLS record on the connection**, including its header and encryption overhead, and must be 23 through 16,406 bytes. No original traffic contents are used.

This verifies TLS record lengths at the server. It does not prove TCP segment lengths, record timing, or what a proxy would emit. There is no proxy in this local experiment. Once this passes with a representative flow list, a proxy client and server can be inserted between these two containers and their connection captured for CenKL.
