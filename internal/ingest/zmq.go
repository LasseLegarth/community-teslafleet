// Package ingest consumes Tesla fleet-telemetry's brokerless ZMQ stream.
// fleet-telemetry's zmq dispatcher PUB-binds an address; we SUB-connect. With
// transmit_decoded_records=true the payload is protojson of the Payload proto:
//
//	{"vin":"...","data":[{"key":"Soc","value":{"intValue":61}},
//	                     {"key":"Location","value":{"locationValue":{"latitude":..,"longitude":..}}}],...}
//
// Each value object has exactly one variant key; we take it generically.
package ingest

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/go-zeromq/zmq4"

	"github.com/LasseLegarth/community-teslafleet/internal/config"
	"github.com/LasseLegarth/community-teslafleet/internal/recorder"
	"github.com/LasseLegarth/community-teslafleet/internal/store"
)

type Consumer struct {
	addr     string
	store    *store.Store
	rec      *recorder.Recorder // optional JSONL recorder (may be nil)
	log      *slog.Logger
	sub      zmq4.Socket
	ctx      context.Context
	cancel   context.CancelFunc
	seenVINs map[string]bool // first-record breadcrumb (loop goroutine only)
}

type vPayload struct {
	Vin  string `json:"vin"`
	Data []struct {
		Key   string                     `json:"key"`
		Value map[string]json.RawMessage `json:"value"`
	} `json:"data"`
}

type connPayload struct {
	Vin    string `json:"vin"`
	Status string `json:"status"`
}

func NewConsumer(cfg config.Ingest, st *store.Store, rec *recorder.Recorder, log *slog.Logger) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{addr: cfg.ZMQAddr, store: st, rec: rec, log: log, ctx: ctx, cancel: cancel, seenVINs: map[string]bool{}}
}

// Start dials the fleet-telemetry PUB socket and begins consuming.
func (c *Consumer) Start() error {
	if err := c.dial(); err != nil {
		return err
	}
	go c.loop()
	return nil
}

// dial creates a fresh SUB socket, dials with retries and subscribes to all
// topics. It is used both at startup and on reconnect.
func (c *Consumer) dial() error {
	sub := zmq4.NewSub(c.ctx)
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if c.ctx.Err() != nil {
			return c.ctx.Err()
		}
		if err = sub.Dial(c.addr); err == nil {
			break
		}
		c.log.Warn("zmq dial retry", "addr", c.addr, "err", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		_ = sub.Close()
		return err
	}
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		_ = sub.Close()
		return err
	}
	c.sub = sub
	c.log.Info("zmq ingest connected", "addr", c.addr)
	return nil
}

// reconnect tears down the faulted socket and re-dials with backoff. Returns
// false if the context was cancelled while reconnecting.
func (c *Consumer) reconnect() bool {
	if c.sub != nil {
		_ = c.sub.Close()
		c.sub = nil
	}
	backoff := time.Second
	for {
		if c.ctx.Err() != nil {
			return false
		}
		if err := c.dial(); err == nil {
			c.log.Info("zmq ingest reconnected", "addr", c.addr)
			return true
		} else {
			c.log.Warn("zmq reconnect failed", "addr", c.addr, "err", err, "retry_in", backoff)
		}
		select {
		case <-c.ctx.Done():
			return false
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Consumer) Stop() {
	c.cancel()
	if c.sub != nil {
		_ = c.sub.Close()
	}
}

func (c *Consumer) loop() {
	const reconnectAfter = 5 // consecutive recv errors before re-dialing
	consecErrs := 0
	for {
		msg, err := c.sub.Recv()
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}
			consecErrs++
			c.log.Warn("zmq recv error", "err", err, "consecutive", consecErrs)
			if consecErrs >= reconnectAfter {
				// Persistent fault on the socket: tear down and re-dial instead of
				// spinning on a faulted socket forever.
				if !c.reconnect() {
					return // context cancelled
				}
				consecErrs = 0
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		consecErrs = 0
		c.handle(msg)
	}
}

func (c *Consumer) handle(msg zmq4.Msg) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic handling zmq message", "recover", r)
		}
	}()
	if len(msg.Frames) == 0 {
		return
	}
	payload := msg.Frames[len(msg.Frames)-1]

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(payload, &probe); err != nil {
		c.log.Warn("zmq top-level json unmarshal failed", "err", err)
		return
	}
	if _, ok := probe["data"]; ok {
		c.handleV(payload)
		return
	}
	if _, ok := probe["status"]; ok {
		c.handleConnectivity(payload)
		return
	}
	c.log.Debug("zmq message matched no known shape")
}

func (c *Consumer) handleV(payload []byte) {
	var p vPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		c.log.Warn("zmq v-payload unmarshal failed", "err", err)
		return
	}
	if p.Vin == "" {
		return
	}
	if !c.seenVINs[p.Vin] {
		c.seenVINs[p.Vin] = true
		c.log.Info("telemetry ingested", "vin", p.Vin)
	}
	for _, d := range p.Data {
		if d.Key == "" {
			continue
		}
		if val, ok := extractValue(d.Value); ok {
			c.store.SetField(p.Vin, d.Key, val)
			c.rec.Record(p.Vin, d.Key, val)
			// Do not log raw GPS coordinates; redact the Location value.
			if d.Key == store.FieldLocation {
				c.log.Debug("telemetry", "vin", p.Vin, "field", d.Key, "value", "<redacted>")
			} else {
				c.log.Debug("telemetry", "vin", p.Vin, "field", d.Key, "value", val)
			}
		}
	}
}

func (c *Consumer) handleConnectivity(payload []byte) {
	var p connPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.Vin == "" {
		return
	}
	status := "online"
	switch strings.ToUpper(p.Status) {
	case "DISCONNECTED":
		status = "offline"
	case "CONNECTED":
		status = "online"
	default:
		status = strings.ToLower(p.Status)
	}
	c.store.SetConnectivity(p.Vin, status)
	c.log.Info("connectivity", "vin", p.Vin, "status", status)
}

// extractValue pulls the single variant out of a protojson Value object.
// Scalars come back as float64/string/bool; Location comes back as a
// map[string]any{latitude,longitude}; enum variants come back as their name string.
func extractValue(m map[string]json.RawMessage) (any, bool) {
	for k, raw := range m {
		if k == "invalid" {
			return nil, false
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, false
		}
		return v, true
	}
	return nil, false
}
