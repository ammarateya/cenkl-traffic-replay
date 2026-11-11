package recordshape

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func TestScannerHandlesSplitRecords(t *testing.T) {
	var s scanner
	record := []byte{23, 3, 3, 0, 2, 1, 2}
	s.feed(record[:3])
	s.feed(record[3:])
	got, err := s.next()
	if err != nil || got != 7 {
		t.Fatalf("record size=%d, err=%v", got, err)
	}
}

func TestMixedDirectionTLSRecords(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	pair, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	flow := replay.Flow{ID: "mixed", Events: []replay.Event{
		{Direction: "client_to_server", Size: 1000},
		{Direction: "client_to_server", Size: 500},
		{Direction: "server_to_client", Size: 1400},
		{Direction: "client_to_server", Size: 250},
	}}
	results := make(chan struct {
		log string
		err error
	}, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			results <- struct {
				log string
				err error
			}{err: err}
			return
		}
		var log strings.Builder
		err = ServeFlow(conn, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, DynamicRecordSizingDisabled: true, SessionTicketsDisabled: true}, flow, 5*time.Second, &log)
		results <- struct {
			log string
			err error
		}{log: log.String(), err: err}
	}()
	var clientLog strings.Builder
	if err := RunClient(replay.Input{Flows: []replay.Flow{flow}}, ln.Addr().String(), 5*time.Second, certPEM, &clientLog); err != nil {
		t.Fatal(err)
	}
	result := <-results
	if result.err != nil {
		t.Fatal(result.err)
	}
	if !strings.Contains(result.log, "verified 4 TLS records") {
		t.Fatal(result.log)
	}
	if strings.Count(result.log, "expected_record=") != 4 {
		t.Fatal(result.log)
	}
}
