package recordshape

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

// RunClient follows the same ordered schedule as ServeFlow. One application
// write of size (target record size - TLS overhead) is made for each send turn.
func RunClient(in replay.Input, endpoint string, timeout time.Duration, caCert []byte, out io.Writer) error {
	return RunClientWithServerName(in, endpoint, "", timeout, caCert, out)
}

// RunClientWithServerName permits TLS verification when a local proxy endpoint
// has a different name from the destination server behind it.
func RunClientWithServerName(in replay.Input, endpoint, serverName string, timeout time.Duration, caCert []byte, out io.Writer) error {
	return RunClientViaSOCKS(in, endpoint, serverName, "", timeout, caCert, out)
}

// RunClientViaSOCKS connects to the destination through a local SOCKS5 proxy
// when proxy is nonempty. TLS is still negotiated end to end with endpoint.
func RunClientViaSOCKS(in replay.Input, endpoint, serverName, proxy string, timeout time.Duration, caCert []byte, out io.Writer) error {
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
	if serverName != "" {
		config.ServerName = serverName
	}
	buf := make([]byte, 16384)
	for _, flow := range in.Flows {
		var conn *tls.Conn
		var err error
		if proxy == "" {
			conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", endpoint, config)
		} else {
			var raw net.Conn
			raw, err = dialSOCKS5(proxy, endpoint, timeout)
			if err == nil {
				conn = tls.Client(raw, config)
				err = conn.Handshake()
				if err != nil {
					conn.Close()
				}
			}
		}
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

func dialSOCKS5(proxy, endpoint string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxy, timeout)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (net.Conn, error) { conn.Close(); return nil, err }
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fail(err)
	}
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return fail(err)
	}
	var reply [4]byte
	if _, err := io.ReadFull(conn, reply[:2]); err != nil {
		return fail(err)
	}
	if reply[0] != 5 || reply[1] != 0 {
		return fail(fmt.Errorf("SOCKS5 proxy rejected no-auth method"))
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fail(err)
	}
	port, err := net.LookupPort("tcp", portText)
	if err != nil {
		return fail(err)
	}
	if len(host) > 255 {
		return fail(fmt.Errorf("SOCKS5 destination name too long"))
	}
	request := []byte{5, 1, 0, 3, byte(len(host))}
	request = append(request, host...)
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	if _, err := conn.Write(request); err != nil {
		return fail(err)
	}
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return fail(err)
	}
	if reply[0] != 5 || reply[1] != 0 {
		return fail(fmt.Errorf("SOCKS5 connect failed: reply %d", reply[1]))
	}
	var addrLen int
	switch reply[3] {
	case 1:
		addrLen = 4
	case 4:
		addrLen = 16
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return fail(err)
		}
		addrLen = int(n[0])
	default:
		return fail(fmt.Errorf("SOCKS5 invalid reply address type"))
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addrLen+2)); err != nil {
		return fail(err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fail(err)
	}
	return conn, nil
}
