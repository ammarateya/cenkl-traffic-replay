#!/usr/bin/env python3
"""Remove flows whose local capture gives CENKL a zero or negative RTT."""
import csv
import pathlib
import sys


def main(directory):
    directory = pathlib.Path(directory)
    rejected = set()
    with (directory / "obfuscation.log").open(newline="") as source, (directory / "obfuscation.filtered.log").open("w", newline="") as dest:
        reader = csv.DictReader(source, delimiter="\t")
        writer = csv.DictWriter(dest, fieldnames=reader.fieldnames, delimiter="\t")
        writer.writeheader()
        for row in reader:
            if float(row["ack_ts"]) <= float(row["syn_ts"]):
                rejected.add(row["conn_id"])
            else:
                writer.writerow(row)
    with (directory / "packets.log").open(newline="") as source, (directory / "packets.filtered.log").open("w", newline="") as dest:
        reader = csv.DictReader(source, delimiter="\t")
        writer = csv.DictWriter(dest, fieldnames=reader.fieldnames, delimiter="\t")
        writer.writeheader()
        for row in reader:
            if row["conn_id"] not in rejected:
                writer.writerow(row)
    print(f"excluded {len(rejected)} outer connections with nonpositive measured RTT")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: scripts/filter-cenkl-zero-rtt.py RUN_DIR")
    main(sys.argv[1])
