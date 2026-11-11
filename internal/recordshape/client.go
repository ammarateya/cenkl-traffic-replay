package recordshape

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

// RunClient follows the same ordered schedule as ServeFlow. One application
// write of size (target record size - TLS overhead) is made for each send turn.
func RunClient(in replay.Input, endpoint string, timeout time.Duration, caCert []byte, out io.Writer) error {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("CA certificate must contain a PEM certificate")
	}
	for _, flow := range in.Flows {
		if err := Validate(flow); err != nil {
			return err
		}
	}
	config := &tls.Config{
		MinVersion:                  tls.VersionTLS13,
		MaxVersion:                  tls.VersionTLS13,
		RootCAs:                     roots,
		DynamicRecordSizingDisabled: true,
		SessionTicketsDisabled:      true,
	}
	buf := make([]byte, 16384)
	for _, flow := range in.Flows {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", endpoint, config)
		if err != nil {
			return fmt.Errorf("flow %q connect: %w", flow.ID, err)
		}
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			conn.Close()
			return err
		}
		var ready [1]byte
		if _, err := io.ReadFull(conn, ready[:]); err != nil || ready[0] != 1 {
			conn.Close()
			return fmt.Errorf("flow %q: server readiness signal missing: %v", flow.ID, err)
		}
		for i, event := range flow.Events {
			plainSize := event.Size - Overhead
			if event.Direction == "client_to_server" {
				if n, err := conn.Write(buf[:plainSize]); err != nil || n != plainSize {
					conn.Close()
					return fmt.Errorf("flow %q event %d send: wrote %d of %d: %v", flow.ID, i+1, n, plainSize, err)
				}
			} else {
				if _, err := io.ReadFull(conn, buf[:plainSize]); err != nil {
					conn.Close()
					return fmt.Errorf("flow %q event %d receive: %w", flow.ID, i+1, err)
				}
			}
		}
		if err := conn.Close(); err != nil {
			return fmt.Errorf("flow %q close: %w", flow.ID, err)
		}
		fmt.Fprintf(out, "%s: completed %d scheduled TLS records\n", flow.ID, len(flow.Events))
	}
	return nil
}
