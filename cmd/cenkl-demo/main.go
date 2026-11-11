package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

//go:embed static/index.html
var assets embed.FS

var recordLine = regexp.MustCompile(`event=(\d+) direction=(client_to_server|server_to_client) expected_record=(\d+) observed_record=(\d+)`)

type runResult struct {
	Seed         int64  `json:"seed,string"`
	FlowFile     string `json:"flow_file"`
	Events       int    `json:"events"`
	ClientEvents int    `json:"client_events"`
	ServerEvents int    `json:"server_events"`
	Matched      int    `json:"matched"`
	ClientOutput string `json:"client_output"`
	ServerLog    string `json:"-"`
}

type demo struct {
	root           string
	mu             sync.Mutex
	latest         *runResult
	stateMu        sync.Mutex
	continuous     continuousState
	stopContinuous context.CancelFunc
}

type continuousState struct {
	Running  bool   `json:"running"`
	Stopping bool   `json:"stopping"`
	Runs     int    `json:"runs"`
	Matched  int    `json:"matched"`
	Journal  string `json:"journal"`
	LastFlow string `json:"last_flow"`
	Error    string `json:"error,omitempty"`
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(filepath.Join(root, "compose.yaml")); err != nil {
		fmt.Fprintln(os.Stderr, "run cenkl-demo from the repository root")
		os.Exit(1)
	}
	d := &demo{root: root}
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.index)
	mux.HandleFunc("/api/run", d.run)
	mux.HandleFunc("/api/logs", d.logs)
	mux.HandleFunc("/api/flow", d.flow)
	mux.HandleFunc("/api/continuous/start", d.startContinuous)
	mux.HandleFunc("/api/continuous/stop", d.stopContinuousRun)
	mux.HandleFunc("/api/continuous/status", d.continuousStatus)
	mux.HandleFunc("/api/continuous/journal", d.continuousJournal)
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Println("Demo available at http://127.0.0.1:8080")
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (d *demo) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(data)
}

func (d *demo) run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !d.mu.TryLock() {
		http.Error(w, "a demo run is already in progress", http.StatusConflict)
		return
	}
	defer d.mu.Unlock()
	var request struct {
		Events int `json:"events"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if request.Events < 2 || request.Events > 500 {
		http.Error(w, "events must be 2..500", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := d.execute(ctx, request.Events)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d.latest = result
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *demo) logs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	d.mu.Lock()
	result := d.latest
	d.mu.Unlock()
	if result == nil {
		http.Error(w, "run a flow first", 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, result.ServerLog)
}

func (d *demo) flow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	d.mu.Lock()
	result := d.latest
	d.mu.Unlock()
	if result == nil {
		http.Error(w, "run a flow first", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="demo-flow.json"`)
	http.ServeFile(w, r, filepath.Join(d.root, result.FlowFile))
}

func (d *demo) startContinuous(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	if !d.mu.TryLock() {
		http.Error(w, "a demo run is already in progress", 409)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	journal := fmt.Sprintf("work/demo-continuous-%d.jsonl", time.Now().UnixNano())
	d.stateMu.Lock()
	d.continuous = continuousState{Running: true, Journal: journal}
	d.stopContinuous = cancel
	d.stateMu.Unlock()
	go d.continuousLoop(ctx, journal)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.continuous)
}

func (d *demo) continuousLoop(ctx context.Context, journal string) {
	defer d.mu.Unlock()
	defer func() {
		d.stateMu.Lock()
		d.continuous.Running = false
		d.continuous.Stopping = false
		d.stopContinuous = nil
		d.stateMu.Unlock()
	}()
	path := filepath.Join(d.root, journal)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		d.continuousError(err)
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		d.continuousError(err)
		return
	}
	defer f.Close()
	for ctx.Err() == nil {
		// A stop request finishes the active flow, then ends the loop.
		batchCtx, batchCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := d.execute(batchCtx, 100)
		batchCancel()
		if err != nil {
			d.continuousError(err)
			return
		}
		entry := struct {
			Time time.Time `json:"time"`
			*runResult
			LogFile string `json:"log_file"`
		}{time.Now().UTC(), result, fmt.Sprintf("work/demo-server-%d.log", result.Seed)}
		if err := json.NewEncoder(f).Encode(entry); err != nil {
			d.continuousError(err)
			return
		}
		if err := f.Sync(); err != nil {
			d.continuousError(err)
			return
		}
		d.latest = result
		d.stateMu.Lock()
		d.continuous.Runs++
		d.continuous.Matched += result.Matched
		d.continuous.LastFlow = result.FlowFile
		d.stateMu.Unlock()
	}
}

func (d *demo) continuousError(err error) {
	d.stateMu.Lock()
	d.continuous.Error = err.Error()
	d.stateMu.Unlock()
}

func (d *demo) stopContinuousRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	d.stateMu.Lock()
	if d.continuous.Running && d.stopContinuous != nil {
		d.continuous.Stopping = true
		d.stopContinuous()
	}
	d.stateMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (d *demo) continuousStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	d.stateMu.Lock()
	state := d.continuous
	d.stateMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (d *demo) continuousJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	d.stateMu.Lock()
	journal := d.continuous.Journal
	d.stateMu.Unlock()
	if journal == "" {
		http.Error(w, "no continuous run", 404)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="demo-continuous.jsonl"`)
	http.ServeFile(w, r, filepath.Join(d.root, journal))
}

func (d *demo) execute(ctx context.Context, count int) (*runResult, error) {
	seed := time.Now().UnixNano()
	flow, clientCount, serverCount := generateFlow(count, seed)
	flowFile := fmt.Sprintf("work/demo-flow-%d.json", seed)
	if err := os.MkdirAll(filepath.Join(d.root, "work"), 0700); err != nil {
		return nil, err
	}
	if err := saveFlow(filepath.Join(d.root, flowFile), flow); err != nil {
		return nil, err
	}
	if err := writeCertificate(d.root); err != nil {
		return nil, err
	}
	env := append(os.Environ(), "FLOW_FILE=./"+flowFile, "TLS_CERT_FILE=./work/demo-cert.pem", "TLS_KEY_FILE=./work/demo-key.pem")
	compose := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-p", "cenkl-demo"}, args...)...)
		cmd.Dir = d.root
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return string(output), nil
	}
	compose("down", "--remove-orphans") // Remove only an earlier run of this dedicated demo project.
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		cmd := exec.CommandContext(cleanup, "docker", "compose", "-p", "cenkl-demo", "down")
		cmd.Dir = d.root
		cmd.Env = env
		cmd.Run()
	}()
	if _, err := compose("up", "-d", "--build", "server"); err != nil {
		return nil, err
	}
	if err := waitForServer(ctx, compose); err != nil {
		return nil, err
	}
	clientOutput, clientErr := compose("run", "--rm", "client")
	serverLog, logErr := compose("logs", "--no-color", "server")
	if logErr != nil {
		return nil, logErr
	}
	if err := os.WriteFile(filepath.Join(d.root, "work", "demo-last-server.log"), []byte(serverLog), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(d.root, "work", fmt.Sprintf("demo-server-%d.log", seed)), []byte(serverLog), 0600); err != nil {
		return nil, err
	}
	if clientErr != nil {
		return nil, clientErr
	}
	ids, err := compose("ps", "-a", "-q", "server")
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(ids)
	if id == "" {
		return nil, errors.New("server container was not found")
	}
	wait := exec.CommandContext(ctx, "docker", "wait", id)
	waitOutput, err := wait.CombinedOutput()
	if err != nil || strings.TrimSpace(string(waitOutput)) != "0" {
		return nil, fmt.Errorf("server did not exit successfully: %s: %v", strings.TrimSpace(string(waitOutput)), err)
	}
	matched, err := compareLog(flow, serverLog)
	if err != nil {
		return nil, err
	}
	return &runResult{Seed: seed, FlowFile: flowFile, Events: count, ClientEvents: clientCount, ServerEvents: serverCount, Matched: matched, ClientOutput: strings.TrimSpace(clientOutput), ServerLog: serverLog}, nil
}

func waitForServer(ctx context.Context, compose func(...string) (string, error)) error {
	for i := 0; i < 50; i++ {
		log, err := compose("logs", "--no-color", "server")
		if err == nil && strings.Contains(log, "TLS record replay server listening") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("server did not start listening")
}

func generateFlow(count int, seed int64) (replay.Input, int, int) {
	rng := mathrand.New(mathrand.NewSource(seed))
	flow := replay.Flow{ID: fmt.Sprintf("demo-%d", seed), Events: make([]replay.Event, count)}
	clientCount, serverCount := 0, 0
	for i := range flow.Events {
		direction := "client_to_server"
		if i > 0 && rng.Intn(2) == 1 {
			direction = "server_to_client"
		}
		if direction == "client_to_server" {
			clientCount++
		} else {
			serverCount++
		}
		flow.Events[i] = replay.Event{Direction: direction, Size: 64 + rng.Intn(1397)}
	}
	return replay.Input{Flows: []replay.Flow{flow}}, clientCount, serverCount
}

func saveFlow(path string, flow replay.Input) error {
	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func compareLog(flow replay.Input, log string) (int, error) {
	events := flow.Flows[0].Events
	matches := recordLine.FindAllStringSubmatch(log, -1)
	if len(matches) != len(events) {
		return 0, fmt.Errorf("server logged %d of %d events", len(matches), len(events))
	}
	for i, match := range matches {
		index, _ := strconv.Atoi(match[1])
		expected, _ := strconv.Atoi(match[3])
		observed, _ := strconv.Atoi(match[4])
		if index != i+1 || match[2] != events[i].Direction || expected != events[i].Size || observed != events[i].Size {
			return i, fmt.Errorf("event %d did not match the saved flow", i+1)
		}
	}
	if !strings.Contains(log, fmt.Sprintf("verified %d TLS records", len(events))) {
		return 0, errors.New("server did not report a verified flow")
	}
	return len(matches), nil
}

func writeCertificate(root string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "server"}, DNSNames: []string{"server"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "demo-cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(work, "demo-key.pem"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600)
}
