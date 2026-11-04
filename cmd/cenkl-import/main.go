// cenkl-import converts CenKL's dequantized size/direction CSV into replay JSON.
package main

import (
	"bufio"
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
	input := flag.String("input", "", "CenKL generate CSV with size/direction model (no timing, no quantized bins)")
	output := flag.String("output", "", "new replay JSON path")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "-input and -output are required")
		os.Exit(2)
	}
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	src, err := os.Open(input)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()
	if err := convert(src, dst); err != nil {
		dst.Close()
		os.Remove(output)
		return err
	}
	return nil
}

func convert(src io.Reader, dst io.Writer) error {
	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1
	writer := bufio.NewWriter(dst)
	if _, err := writer.WriteString("{\"flows\":[\n"); err != nil {
		return err
	}
	count := 0
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("CSV row %d: %w", count+1, err)
		}
		if len(row) == 0 {
			return fmt.Errorf("CSV row %d is empty", count+1)
		}
		flow := replay.Flow{ID: fmt.Sprintf("cenkl-%d", count+1)}
		for col, field := range row {
			value, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64)
			if err != nil {
				return fmt.Errorf("CSV row %d column %d: expected signed packet size (use CenKL size/direction model without timing or --quantized)", count+1, col+1)
			}
			if value == 0 || value > 16<<20 || value < -(16<<20) {
				return fmt.Errorf("CSV row %d column %d: size out of range", count+1, col+1)
			}
			event := replay.Event{Direction: "client_to_server", Size: int(value)}
			if value < 0 {
				event.Direction = "server_to_client"
				event.Size = int(-value)
			}
			flow.Events = append(flow.Events, event)
		}
		data, err := json.Marshal(flow)
		if err != nil {
			return err
		}
		if count > 0 {
			if _, err := writer.WriteString(",\n"); err != nil {
				return err
			}
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return errors.New("CSV contains no flows")
	}
	if _, err := writer.WriteString("\n]}\n"); err != nil {
		return err
	}
	return writer.Flush()
}
