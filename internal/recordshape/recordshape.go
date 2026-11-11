package recordshape

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

// TLS 1.3 records written by Go contain a 5-byte header, a 1-byte inner
// content type, and a 16-byte authentication tag, with no optional padding.
const Overhead = 22
const MaxRecord = 16384 + Overhead

func Validate(flow replay.Flow) error {
	for i, event := range flow.Events {
		if event.Size < Overhead+1 || event.Size > MaxRecord {
			return fmt.Errorf("flow %q event %d: full TLS record size must be %d..%d", flow.ID, i, Overhead+1, MaxRecord)
		}
	}
	return nil
}

type scanner struct {
	pending []byte
	sizes   []int
	err     error
}

func (s *scanner) feed(data []byte) {
	if s.err != nil {
		return
	}
	s.pending = append(s.pending, data...)
	for len(s.pending) >= 5 {
		size := 5 + int(binary.BigEndian.Uint16(s.pending[3:5]))
		if size < 5 || size > 16645 {
			s.err = fmt.Errorf("invalid TLS record length %d", size)
			return
		}
		if len(s.pending) < size {
			return
		}
		s.sizes = append(s.sizes, size)
		s.pending = s.pending[size:]
	}
}

func (s *scanner) next() (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if len(s.sizes) == 0 {
		return 0, fmt.Errorf("no complete TLS record observed")
	}
	size := s.sizes[0]
	s.sizes = s.sizes[1:]
	return size, nil
}

// ObservedConn records encrypted TLS record lengths on the socket beneath tls.Conn.
// Arm it only after the handshake and the one-byte readiness signal.
type ObservedConn struct {
	net.Conn
	mu      sync.Mutex
	armed   bool
	in, out scanner
}

func (c *ObservedConn) Arm() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = true
	c.in = scanner{}
	c.out = scanner{}
}

func (c *ObservedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.mu.Lock()
	if c.armed && n > 0 {
		c.in.feed(p[:n])
	}
	c.mu.Unlock()
	return n, err
}

func (c *ObservedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.mu.Lock()
	if c.armed && n > 0 {
		c.out.feed(p[:n])
	}
	c.mu.Unlock()
	return n, err
}

func (c *ObservedConn) Next(direction string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if direction == "client_to_server" {
		return c.in.next()
	}
	return c.out.next()
}

// ServeFlow exchanges dummy TLS records in the order given by one shared flow.
// The readiness record is outside the measured schedule.
func ServeFlow(raw net.Conn, config *tls.Config, flow replay.Flow, timeout time.Duration, log io.Writer) error {
	if err := Validate(flow); err != nil {
		return err
	}
	observed := &ObservedConn{Conn: raw}
	conn := tls.Server(observed, config)
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if err := conn.Handshake(); err != nil {
		return fmt.Errorf("TLS handshake: %w", err)
	}
	if _, err := conn.Write([]byte{1}); err != nil {
		return fmt.Errorf("send readiness signal: %w", err)
	}
	observed.Arm()
	buf := make([]byte, 16384)
	for i, event := range flow.Events {
		plainSize := event.Size - Overhead
		if event.Direction == "client_to_server" {
			if n, err := io.ReadFull(conn, buf[:plainSize]); err != nil {
				return fmt.Errorf("event %d receive: got %d of %d bytes: %w", i, n, plainSize, err)
			}
		} else {
			if n, err := conn.Write(buf[:plainSize]); err != nil || n != plainSize {
				return fmt.Errorf("event %d send: wrote %d of %d: %v", i, n, plainSize, err)
			}
		}
		actual, err := observed.Next(event.Direction)
		if err != nil {
			return fmt.Errorf("event %d record: %w", i, err)
		}
		fmt.Fprintf(log, "%s event=%d direction=%s expected_record=%d observed_record=%d\n", flow.ID, i+1, event.Direction, event.Size, actual)
		if actual != event.Size {
			return fmt.Errorf("flow %q event %d: record size mismatch: expected %d, observed %d", flow.ID, i+1, event.Size, actual)
		}
	}
	fmt.Fprintf(log, "%s: verified %d TLS records\n", flow.ID, len(flow.Events))
	return nil
}
