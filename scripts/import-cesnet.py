#!/usr/bin/env python3
"""Convert a bounded CESNET TLS-Year22 HDF5 sample to surrogate replay flows.

Requires PyTables: python -m pip install tables
"""
import argparse
import json
import sys

import tables


def convert(path, output, limit, skip):
    flows = []
    scanned = 0
    rejected = 0
    with tables.open_file(path, mode="r") as database:
        for table in database.root.flows:
            for row in table:
                if scanned < skip:
                    scanned += 1
                    continue
                scanned += 1
                length = int(row["PPI_LEN"])
                ppi = row["PPI"]
                if not 1 <= length <= 30:
                    rejected += 1
                    continue
                events = []
                for index in range(length):
                    direction = int(ppi[1, index])
                    size = int(ppi[2, index])
                    if direction not in (-1, 1) or not 23 <= size <= 16406:
                        break
                    events.append({
                        "direction": "client_to_server" if direction == 1 else "server_to_client",
                        "size": size,
                    })
                if len(events) != length:
                    rejected += 1
                    continue
                flows.append({"id": f"cesnet-{table._v_name}-{scanned}", "events": events})
                if len(flows) == limit:
                    with open(output, "x", encoding="utf-8") as dest:
                        json.dump({"flows": flows}, dest, indent=2)
                        dest.write("\n")
                    print(f"saved {len(flows)} flows from {scanned - skip} examined rows; skipped {rejected} unsupported flows")
                    return
    raise ValueError(f"only {len(flows)} valid flows found after examining {scanned - skip} rows")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="CESNET-TLS-Year22-XS.h5 or similar")
    parser.add_argument("--output", required=True, help="new JSON replay file")
    parser.add_argument("--limit", type=int, default=1000)
    parser.add_argument("--skip", type=int, default=0, help="number of HDF rows before sampling")
    args = parser.parse_args()
    if args.limit < 1 or args.skip < 0:
        parser.error("--limit must be positive and --skip nonnegative")
    try:
        convert(args.input, args.output, args.limit, args.skip)
    except (OSError, ValueError, tables.HDF5ExtError) as error:
        sys.exit(str(error))
