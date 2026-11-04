#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: $0 CAPTURE.pcap [SERVER_PORT]" >&2
  exit 2
fi

capture=$1
port=${2:-9000}

# TCP's 'length' is the data payload carried by that captured segment.
# This includes the TLS handshake; inspect the later application-data packets.
tcpdump -nn -tt -r "$capture" "tcp dst port $port and src host 127.0.0.1" 2>/dev/null |
  awk '/length [1-9][0-9]*/ { for (i = 1; i <= NF; i++) if ($i == "length") { print $1, $(i+1) } }'
