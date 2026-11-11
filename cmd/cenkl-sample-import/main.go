package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	input := flag.String("input", "", "Aaron's five-column TSV sample")
	output := flag.String("output", "", "new replay JSON file")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-input and -output are required")
		os.Exit(2)
	}
	src, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer src.Close()
	flows, err := convert(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dst, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer dst.Close()
	enc := json.NewEncoder(dst)
	enc.SetIndent("", "  ")
	if err := enc.Encode(flows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("imported %d flows\n", len(flows.Flows))
}

func convert(src io.Reader) (replay.Input, error) {
	r := csv.NewReader(src)
	r.Comma = '\t'
	r.FieldsPerRecord = 5
	header, err := r.Read()
	if err != nil {
		return replay.Input{}, err
	}
	want := []string{"conn_id", "idx", "timestamp", "direction", "size"}
	for i := range want {
		if strings.TrimSpace(header[i]) != want[i] {
			return replay.Input{}, errors.New("expected conn_id, idx, timestamp, direction, size header")
		}
	}
	var out replay.Input
	positions := map[string]int{}
	for line := 2; ; line++ {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return replay.Input{}, fmt.Errorf("line %d: %w", line, err)
		}
		id := strings.TrimSpace(row[0])
		if id == "" {
			return replay.Input{}, fmt.Errorf("line %d: empty connection ID", line)
		}
		position, ok := positions[id]
		if !ok {
			position = len(out.Flows)
			positions[id] = position
			out.Flows = append(out.Flows, replay.Flow{ID: id})
		}
		index, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil || index != len(out.Flows[position].Events) {
			return replay.Input{}, fmt.Errorf("line %d: index must be the next position for connection %s", line, id)
		}
		timestamp := strings.TrimSpace(row[2])
		if _, err := strconv.ParseFloat(timestamp, 64); err != nil {
			return replay.Input{}, fmt.Errorf("line %d: invalid timestamp", line)
		}
		direction := "client_to_server"
		switch strings.TrimSpace(row[3]) {
		case "1":
		case "0":
			direction = "server_to_client"
		default:
			return replay.Input{}, fmt.Errorf("line %d: direction must be 0 or 1", line)
		}
		size, err := strconv.Atoi(strings.TrimSpace(row[4]))
		if err != nil || size < 18 || size > 16401 {
			return replay.Input{}, fmt.Errorf("line %d: record size excluding header must be 18..16401", line)
		}
		out.Flows[position].Events = append(out.Flows[position].Events, replay.Event{Direction: direction, Size: size + 5, Timestamp: timestamp})
	}
	if len(out.Flows) == 0 {
		return replay.Input{}, errors.New("sample contains no flows")
	}
	return out, nil
}
