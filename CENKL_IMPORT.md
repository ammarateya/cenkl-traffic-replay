# Use CenKL-generated flows with this client

CenKL already has a statistical generator. This repository does not reimplement that model. `cenkl-import` converts the generator's CSV into the JSON format accepted by `cenkl-replay`.

## Generate a flow list with CenKL

In a checkout of [`censoredplanet/obfuscation`](https://github.com/censoredplanet/obfuscation), run the following from its `cenkl` directory:

```sh
cargo run --release -- generate \
  --traffic-profile ../models/size_direction/markov_order_1/hist.bin \
  --model ../models/size_direction/markov_order_1/model_assumptions.json \
  --num-flows 10 --flow-length 10 \
  --output samples.csv
```

This uses the published TLS model of packet sizes and directions, with timing masked. Leave off `--quantized`: the importer needs actual signed packet sizes rather than model bin numbers. CenKL writes one headerless CSV row per flow, with positive numbers for client-to-server packets and negative numbers for server-to-client packets.

## Convert and replay

From this repository:

```sh
go run ./cmd/cenkl-import -input /path/to/obfuscation/cenkl/samples.csv -output cenkl-flows.json
go run ./cmd/cenkl-replay -input cenkl-flows.json -endpoint 127.0.0.1:9000 -tls -ca-cert work/server-cert.pem -target-segments
```

Start the local TLS discard server as shown in the main README before replaying. The importer preserves event order, directions, and sizes in JSON. With `-target-segments`, client event sizes are treated as target TCP payload sizes; client events smaller than 23 bytes cannot be replayed in this mode because a TLS 1.3 record needs overhead. It never overwrites an existing output file.

The current replay client sends dummy bytes for client-to-server entries and reports server-to-client entries as unsupported. Thus this converts CenKL's generated flow descriptions into replay input, but does not yet reproduce complete two-direction flows or guarantee captured TCP segment sizes. Capture and compare packets when evaluating replay fidelity.
