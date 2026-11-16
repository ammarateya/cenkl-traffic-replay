package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	input := flag.String("input", "", "five-column private record log")
	output := flag.String("output", "", "new replay JSON file")
	skip := flag.Int("skip", 0, "number of distinct connection IDs to skip")
	limit := flag.Int("limit", 100, "number of complete connections to extract")
	flag.Parse()
	if *input == "" || *output == "" || *skip < 0 || *limit < 1 {
		fmt.Fprintln(os.Stderr, "use -input FILE -output FILE [-skip N] [-limit N]")
		os.Exit(2)
	}
	if err := run(*input, *output, *skip, *limit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string, skip, limit int) error {
	src, err := os.Open(input)
	if err != nil {
		return err
	}
	defer src.Close()
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	if !scanner.Scan() || string(scanner.Bytes()) != "conn_id\tidx\ttimestamp\tdirection\tsize" {
		return fmt.Errorf("expected five-column TSV header")
	}
	selected := make(map[string]int, limit)
	seen := make(map[string]struct{}, skip+limit)
	invalid := make(map[string]struct{})
	var result replay.Input
	rows := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		cut := bytes.IndexByte(line, '\t')
		if cut < 1 {
			return fmt.Errorf("row %d: missing connection ID", rows+2)
		}
		id := string(line[:cut])
		pos, ok := selected[id]
		if !ok {
			if _, known := seen[id]; known {
				continue
			}
			if len(seen) >= skip+limit {
				continue
			}
			seen[id] = struct{}{}
			if len(seen) <= skip {
				continue
			}
			pos = len(result.Flows)
			selected[id] = pos
			result.Flows = append(result.Flows, replay.Flow{ID: id})
		}
		if _, bad := invalid[id]; bad {
			continue
		}
		fields := bytes.Split(line, []byte{'\t'})
		if len(fields) != 5 {
			return fmt.Errorf("row %d: expected five columns", rows+2)
		}
		idx, err := strconv.Atoi(string(fields[1]))
		if err != nil || idx != len(result.Flows[pos].Events) {
			invalid[id] = struct{}{}
			continue
		}
		direction := "client_to_server"
		if bytes.Equal(fields[3], []byte{'0'}) {
			direction = "server_to_client"
		} else if !bytes.Equal(fields[3], []byte{'1'}) {
			return fmt.Errorf("row %d: invalid direction", rows+2)
		}
		size, err := strconv.Atoi(string(fields[4]))
		if err != nil || size < 18 || size > 16401 {
			invalid[id] = struct{}{}
			continue
		}
		result.Flows[pos].Events = append(result.Flows[pos].Events, replay.Event{Direction: direction, Size: size + 5, Timestamp: string(fields[2])})
		rows++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(result.Flows) != limit {
		return fmt.Errorf("found %d of %d requested connections", len(result.Flows), limit)
	}
	if len(invalid) > 0 {
		filtered := result.Flows[:0]
		for _, flow := range result.Flows {
			if _, bad := invalid[flow.ID]; !bad {
				filtered = append(filtered, flow)
			}
		}
		result.Flows = filtered
	}
	if len(result.Flows) == 0 {
		return fmt.Errorf("all selected connections contained unsupported TLS record sizes")
	}
	dst, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		dst.Close()
		os.Remove(output)
		return err
	}
	fmt.Printf("extracted %d complete connections from %s; skipped %d connections with missing indices or unsupported sizes\n", len(result.Flows), strings.TrimSpace(input), len(invalid))
	return nil
}
