#!/usr/bin/env python3
"""Drive one inner TLS flow per proxy-client process lifetime."""
import argparse
import json
import pathlib
import socket
import subprocess
import sys
import time


def wait_port(port, timeout=15):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                return
        except OSError:
            time.sleep(0.05)
    raise TimeoutError(f"SOCKS port {port} did not open")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input")
    parser.add_argument("run_dir")
    parser.add_argument("proxy_container")
    parser.add_argument("port", type=int)
    parser.add_argument("cert")
    args = parser.parse_args()
    with open(args.input, encoding="utf-8") as source:
        flows = json.load(source)["flows"]
    run_dir = pathlib.Path(args.run_dir)
    current = run_dir / "current-flow.json"
    wait_port(args.port)
    def restart_proxy():
        subprocess.run(["docker", "kill", "--signal=USR1", args.proxy_container], check=True, stdout=subprocess.DEVNULL)
        time.sleep(0.05)

    with (run_dir / "client.log").open("w", encoding="utf-8") as log:
        for index, flow in enumerate(flows):
            current.write_text(json.dumps({"flows": [flow]}), encoding="utf-8")
            for attempt in range(3):
                result = subprocess.run([
                    "work/cenkl-replay-host", "-input", str(current), "-endpoint", "server:9000",
                    "-server-name", "server", "-socks5", f"127.0.0.1:{args.port}",
                    "-ca-cert", args.cert,
                ], capture_output=True, text=True, timeout=180)
                if result.returncode == 0:
                    log.write(result.stdout)
                    break
                log.write(f"{flow['id']} attempt {attempt + 1}: {result.stderr}\n")
                restart_proxy()
            else:
                raise RuntimeError(f"flow {index + 1} {flow['id']} failed after proxy restarts")
            if index + 1 < len(flows):
                restart_proxy()
            if (index + 1) % 100 == 0 or index + 1 == len(flows):
                log.flush()
                print(f"{index + 1}/{len(flows)} completed", flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        sys.exit(str(error))
