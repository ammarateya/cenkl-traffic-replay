# Run the client and server in two containers

Both containers read the same ordered TLS record flow file. The client and server take turns sending dummy TLS records. The server logs each expected and observed full record size.

## Run

From the repository root, create a local certificate. The certificate includes `server` because that is the server container's network name:

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

The example list requests records of 1,000 and 500 bytes from client to server, then 1,400 bytes from server to client. The server should print matching `expected_record` and `observed_record` values for all three, followed by `verified 3 TLS records`.

The server exits after the scheduled flows. To run again, start it again. To remove the containers, use `docker compose down`.

## What this measures

The containers communicate on port 9000 **inside Docker's private network**. That port is not published to your computer or the internet. `size` means the entire encrypted TLS 1.3 record, including header and encryption overhead. The one-byte readiness signal is excluded from the server's comparison log but remains visible in a network capture. This test checks TLS record lengths and order; it does not check TCP segment sizes, timing, or proxy behavior.
