package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"

	"github.com/LasseLegarth/community-teslafleet/internal/config"
	"github.com/LasseLegarth/community-teslafleet/internal/store"
)

// TestConsumerReconnectsAfterPeerDrop reproduces the production wedge that
// froze Home Assistant's device_tracker for ~24h: fleet-telemetry's PUB peer
// dropped, go-zeromq surfaced the fault exactly once and then blocked forever
// on the next Recv(), so the old "re-dial after 5 consecutive errors" loop
// never reconnected. The consumer must recover on the FIRST error and resume
// ingesting once a peer is available again.
//
// With the pre-fix loop this test hangs on the second phase and fails on
// timeout; with the fix it passes.
func TestConsumerReconnectsAfterPeerDrop(t *testing.T) {
	addr := freeTCPAddr(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := store.New("VINTEST")

	ctx := context.Background()
	pub1 := zmq4.NewPub(ctx)
	if err := pub1.Listen(addr); err != nil {
		t.Fatalf("pub1 listen: %v", err)
	}

	c := NewConsumer(config.Ingest{ZMQAddr: addr}, st, nil, log)
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	// Phase 1: normal ingest works.
	publishUntil(t, pub1, socMsg(61), func() bool {
		n, ok := st.Snapshot("VINTEST")
		v, has := n.Num("Soc")
		return ok && has && v == 61
	}, 5*time.Second, "phase 1: field never ingested")

	// Drop the peer, then stand a fresh PUB back up on the same address (as a
	// fleet-telemetry restart would). Rebind promptly so the consumer's
	// reconnect finds a live listener.
	_ = pub1.Close()
	pub2 := listenWithRetry(t, ctx, addr)
	defer pub2.Close()

	// Phase 2: the field must update again — only possible if the consumer
	// noticed the drop and re-dialed.
	publishUntil(t, pub2, socMsg(42), func() bool {
		n, ok := st.Snapshot("VINTEST")
		v, has := n.Num("Soc")
		return ok && has && v == 42
	}, 15*time.Second, "phase 2: consumer did not reconnect after peer drop")
}

// socMsg builds a decoded fleet-telemetry V-record with a single Soc field.
func socMsg(soc int) zmq4.Msg {
	return zmq4.NewMsg([]byte(fmt.Sprintf(
		`{"vin":"VINTEST","data":[{"key":"Soc","value":{"intValue":%d}}]}`, soc)))
}

// publishUntil sends msg repeatedly (ZMQ PUB/SUB is a slow joiner) until cond
// holds or the deadline passes.
func publishUntil(t *testing.T, pub zmq4.Socket, msg zmq4.Msg, cond func() bool, timeout time.Duration, failMsg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = pub.Send(msg)
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal(failMsg)
}

// listenWithRetry binds a fresh PUB on addr, retrying briefly to ride out the
// socket teardown of the previous peer.
func listenWithRetry(t *testing.T, ctx context.Context, addr string) zmq4.Socket {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		pub := zmq4.NewPub(ctx)
		if err := pub.Listen(addr); err == nil {
			return pub
		} else {
			_ = pub.Close()
			if time.Now().After(deadline) {
				t.Fatalf("rebind %s: %v", addr, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// freeTCPAddr returns a currently-free loopback address as a ZMQ tcp:// URL.
func freeTCPAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return fmt.Sprintf("tcp://127.0.0.1:%d", port)
}
