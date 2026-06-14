// Command gateway is the community-teslafleet service: it ingests Tesla Fleet
// Telemetry (via fleet-telemetry's MQTT output) and exposes it as a local Fleet
// API (for TeslaMate to poll, free) and as Home Assistant MQTT auto-discovery.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/legarth/community-teslafleet/internal/commands"
	"github.com/legarth/community-teslafleet/internal/config"
	"github.com/legarth/community-teslafleet/internal/fleetapi"
	"github.com/legarth/community-teslafleet/internal/hadiscovery"
	"github.com/legarth/community-teslafleet/internal/ingest"
	"github.com/legarth/community-teslafleet/internal/recorder"
	"github.com/legarth/community-teslafleet/internal/store"
	"github.com/legarth/community-teslafleet/internal/vehicledata"
	"github.com/legarth/community-teslafleet/internal/wss"
)

func main() {
	cfgPath := flag.String("config", envOr("TGW_CONFIG", "/config/config.yaml"), "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	log := newLogger(cfg.LogLevel)
	log.Info("starting community-teslafleet",
		"vehicles", len(cfg.Vehicles), "fleetapi", cfg.FleetAPI.Enabled, "ha", cfg.HA.Enabled)

	vins := make([]string, 0, len(cfg.Vehicles))
	for _, v := range cfg.Vehicles {
		vins = append(vins, v.VIN)
	}
	st := store.New(vins...)

	// Load per-VIN templates (captured vehicle_data). Missing template → skeleton.
	tmpls := map[string]*vehicledata.Template{}
	for _, v := range cfg.Vehicles {
		t, err := vehicledata.LoadTemplate(v.Template)
		if err != nil {
			log.Warn("template", "vin", v.VIN, "err", err)
		}
		tmpls[v.VIN] = t
	}

	// Optional JSONL recorder (debug value changes over time, e.g. a whole drive).
	var rec *recorder.Recorder
	if cfg.Recording.Enabled {
		r, err := recorder.New(cfg.Recording.Path, cfg.Recording.MaxMB, log)
		if err != nil {
			log.Error("recorder init failed", "err", err)
		} else {
			rec = r
			defer rec.Close()
		}
	}

	// Telemetry consumer (brokerless ZMQ from fleet-telemetry).
	consumer := ingest.NewConsumer(cfg.Ingest, st, rec, log)
	if err := consumer.Start(); err != nil {
		log.Error("zmq ingest start failed", "err", err)
		os.Exit(1)
	}
	defer consumer.Stop()

	// Command relay (optional): HA buttons/switches → signed Tesla commands.
	var relay *commands.Relay
	if cfg.Commands.Enabled {
		relay = commands.NewRelay(cfg.Commands, vins, st, log)
		log.Info("command relay enabled", "proxy", cfg.Commands.ProxyURL)
		// Auto-name each vehicle from the Fleet API (display_name) so the HA device
		// is "Tesla - <name>" instead of the VIN. Only when not explicitly named.
		for i := range cfg.Vehicles {
			if cfg.Vehicles[i].DisplayName != cfg.Vehicles[i].VIN {
				continue
			}
			if dn, err := relay.VehicleDisplayName(cfg.Vehicles[i].VIN); err == nil && dn != "" {
				cfg.Vehicles[i].DisplayName = "Tesla - " + dn
				log.Info("auto-named vehicle", "name", cfg.Vehicles[i].DisplayName)
			} else if err != nil {
				log.Warn("could not fetch vehicle display name; keeping default", "err", err)
			}
		}
	}

	// Home Assistant publisher (optional).
	var publisher *hadiscovery.Publisher
	if cfg.HA.Enabled {
		publisher = hadiscovery.NewPublisher(&cfg, st, relay, log)
		if err := publisher.Start(); err != nil {
			log.Error("ha publisher start failed", "err", err)
		} else {
			defer publisher.Stop()
		}
	}

	// Fleet API emulator + legacy WSS streaming (both TeslaMate-facing, same port).
	var srv *http.Server
	if cfg.FleetAPI.Enabled {
		api := fleetapi.NewServer(st, &cfg, tmpls, relay, cfg.Commands.EnrollFile, log)
		router := api.Routes()
		wssSrv := wss.NewServer(st, &cfg, log)
		router.Handle("/streaming/*", wssSrv.Handler())
		router.Handle("/streaming", wssSrv.Handler())
		srv = &http.Server{
			Addr:    cfg.HTTP.Listen,
			Handler: router,
			// WriteTimeout is intentionally left unset: the WSS handler serves
			// long-lived streaming connections.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		go func() {
			log.Info("fleet api + wss listening", "addr", cfg.HTTP.Listen)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("fleet api server error", "err", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Info("shutting down")
	if srv != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}
}

func newLogger(level string) *slog.Logger {
	lvl := slog.LevelInfo
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
