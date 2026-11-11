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
go run ./cmd/cenkl-replay -input cenkl-flows.json
```

Start the scheduled TLS server with the same JSON file before replaying: `go run ./cmd/cenkl-server -input cenkl-flows.json`. The importer preserves event order, directions, and sizes in JSON. It never overwrites an existing output file.

The default mode exchanges dummy records in both directions and verifies their full TLS record lengths at the server. **Important:** CenKL's generated signed sizes originate from its TCP-payload model. Using those numbers as full TLS record sizes is a prototype mapping, not a conversion proven to preserve the original packet sizes. Values below 23 bytes cannot be represented as these TLS 1.3 records. Capture and compare packets separately when evaluating TCP segment fidelity.
