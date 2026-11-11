# TLS record replay design

## Purpose

Generate dummy TLS traffic from an ordered list of sizes and directions. The first local check is whether the client and server exchange full TLS records of the requested lengths in the requested order. This is a building block for later proxy tests; it is independent of any particular proxy.

## Input

Both programs receive the same JSON file. Each flow has an ID and an ordered list of events. An event has a direction (`client_to_server` or `server_to_client`) and a `size` measured as the **entire encrypted TLS 1.3 record**, including its 5-byte header and authentication overhead. Supported sizes are 23 through 16,406 bytes. The example file is synthetic; it is not ISP trace data.

CenKL's published model describes TCP payload sizes of captured data packets. Mapping those numbers directly to TLS record sizes is an assumption, not an equivalence. A representative input sample and a precise size convention are needed before evaluating a real proxy.

## Protocol

One flow uses one TLS connection. The server finishes the TLS handshake, sends a one-byte readiness signal, and starts observing encrypted TLS record framing beneath the TLS library. The client waits for that signal before following the list. For each entry, the named sender writes one record containing zero bytes. The receiver reads the corresponding decrypted bytes before proceeding. This naturally synchronizes turns without relying on timing sleeps. Consecutive entries in the same direction preserve their list order.

Go TLS 1.3 adds 22 bytes to each unpadded application record: a 5-byte header, a 1-byte encrypted inner content type, and a 16-byte authentication tag. Dynamic record sizing is disabled, and each event results in one application write of `size - 22` dummy bytes. The server's wrapper reads the underlying TLS record framing and compares every observed full record length with the requested size. It reports a mismatch as an error.

The readiness signal is excluded from the server's comparison log but is still a real TLS record on the network. A later proxy experiment must account for or remove that preamble before treating a capture as a CenKL flow. A single TLS record can also be split across TCP segments or share a segment with another record; this experiment verifies TLS record lengths, not TCP packetization.

## Next steps

1. Confirm a sample input flow and its TLS record size convention with the research team.
2. Decide how to derive record schedules from CenKL's TCP-payload model or from permitted trace metadata.
3. Insert one proxy client/server pair and capture the connection between them for CenKL analysis.
4. Measure flow throughput, errors, and resource use before scaling to more containers or hosts.

The current repository does not generate ISP traces, run a proxy, capture traffic, model timing, or run CenKL analysis.
