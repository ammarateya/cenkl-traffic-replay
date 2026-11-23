# Server-side evaluator prototype

This service accepts a public GitHub repository URL or a supported proxy preset. It runs a single background job at a time and stores job state under `work/service-jobs/`. It binds to localhost by default; it has no accounts or authentication and is intended for a local demonstration.

## Start

Requirements: Node.js 18+, `npm`, Docker, Go, and a working Codex SDK authentication for GitHub setup jobs. For preset CENKL runs, also run `scripts/setup-cenkl.sh` first and install the dependencies in [PIPELINE.md](PIPELINE.md).

```sh
cd service
npm install
npm start
```

Open <http://127.0.0.1:8081>. You can also submit JSON to `POST /api/jobs` and poll `GET /api/jobs/{id}`.

```json
{"type":"github","url":"https://github.com/owner/repo"}
```

The service clones a public GitHub repository, records its exact commit, and asks the Codex SDK to write `SETUP_CANDIDATE.md` in that checkout. The agent is instructed to inspect code without executing it and runs in workspace-write mode with network access disabled. The job ends at `needs_review`. Read the candidate at `GET /api/jobs/{id}/candidate`. This is a candidate for a human to check and turn into a trusted adapter; arbitrary submitted code is not yet connected to the replay runner. A production deployment needs stronger worker isolation and job authentication.

```json
{"type":"preset","preset":"gost","sample":"model"}
```

Supported automatic presets are `gost`, `vmess`, `vless`, `trojan`, `vless-ws`, and `vision`. `smoke` (the default) replays `examples/flows.json` and saves a capture. `model` generates 200 synthetic flows of 30 TLS records each, runs the existing capture script, invokes `analyze-cenkl.sh`, and writes `report.json`. The report is available at `GET /api/jobs/{id}/report`. Its input is synthetic and its score is only a pipeline check, not a defensible estimate of real-world detectability. NaiveProxy and gRPC require their separate per-flow runner and remain manual.

The service does not send email, run a 12-hour job, or turn agent-generated setup instructions directly into Docker commands. Those steps need reviewed adapters, bounded worker execution, representative input, and report interpretation.
