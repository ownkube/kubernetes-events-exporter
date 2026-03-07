package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ownkube/kubernetes-events-exporter/internal/cluster"
	"github.com/ownkube/kubernetes-events-exporter/internal/config"
	"github.com/ownkube/kubernetes-events-exporter/internal/dispatch"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/observability"
	"github.com/ownkube/kubernetes-events-exporter/internal/pipeline"
	"github.com/ownkube/kubernetes-events-exporter/internal/watcher"
)

var (
	conf       = flag.String("conf", "config.yaml", "The config path file")
	addr       = flag.String("metrics-address", ":2112", "The address to listen on for HTTP requests.")
	kubeconfig = flag.String("kubeconfig", "", "Path to the kubeconfig file to use.")
	tlsConf    = flag.String("metrics-tls-config", "", "The TLS config file for your metrics.")
)

func main() {
	flag.Parse()

	slog.Info("Reading config file " + *conf)
	configBytes, err := os.ReadFile(*conf)
	if err != nil {
		slog.Error("cannot read config file", "error", err)
		os.Exit(1)
	}

	configBytes = []byte(os.ExpandEnv(string(configBytes)))

	cfg, err := config.ParseFromBytes(configBytes)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	if cfg.LogLevel != "" {
		var level slog.Level
		switch cfg.LogLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			slog.Error("Invalid log level", "level", cfg.LogLevel)
			os.Exit(1)
		}
		opts := &slog.HandlerOptions{Level: level, AddSource: true}
		if cfg.LogFormat == "json" {
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
		} else {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
		}
	} else {
		slog.Info("Set default log level to info. Use config.logLevel=[debug | info | warn | error] to overwrite.")
		opts := &slog.HandlerOptions{Level: slog.LevelInfo, AddSource: true}
		if cfg.LogFormat == "json" {
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
		} else {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
		}
	}

	cfg.SetDefaults()

	slog.Info("Starting with config", "config", cfg)

	if err := cfg.Validate(); err != nil {
		slog.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	kubecfg, err := cluster.NewRESTConfig(*kubeconfig)
	if err != nil {
		slog.Error("cannot get kubeconfig", "error", err)
		os.Exit(1)
	}
	kubecfg.QPS = cfg.KubeQPS
	kubecfg.Burst = cfg.KubeBurst

	observability.Init(*addr, *tlsConf)
	counters := observability.NewCounters(cfg.MetricsNamePrefix)

	engine := pipeline.NewPipeline(&cfg, &dispatch.AsyncDispatcher{Counters: counters})
	onEvent := engine.OnEvent
	if cfg.ClusterName != "" || cfg.ClusterEnvironment != "" {
		onEvent = func(ev *event.EnrichedEvent) {
			if cfg.ClusterName != "" {
				ev.ClusterName = cfg.ClusterName
			}
			if cfg.ClusterEnvironment != "" {
				ev.ClusterEnvironment = cfg.ClusterEnvironment
			}
			engine.OnEvent(ev)
		}
	}

	w := watcher.NewWatcher(kubecfg, cfg.Namespace, cfg.MaxEventAgeSeconds, counters, onEvent, cfg.OmitLookup, cfg.CacheSize)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if cfg.LeaderElection.Enabled {
		var wasLeader bool
		slog.Info("leader election enabled")

		onStoppedLeading := func(ctx context.Context) {
			select {
			case <-ctx.Done():
				slog.Info("Context was cancelled, stopping leader election loop")
			default:
				slog.Info("Lost the leader lease, stopping leader election loop")
			}
		}

		l, err := cluster.NewLeaderElector(cfg.LeaderElection.LeaderElectionID, kubecfg,
			func(_ context.Context) {
				wasLeader = true
				slog.Info("leader election won")
				w.Start()
			},
			func() {
				onStoppedLeading(ctx)
			},
			func(identity string) {
				slog.Info("new leader observed: " + identity)
			},
		)
		if err != nil {
			slog.Error("create leaderelector failed", "error", err)
			os.Exit(1)
		}

		l.Run(ctx)

		if wasLeader {
			slog.Info("waiting leaseDuration seconds before stopping", "duration", cluster.GetLeaseDuration())
			time.Sleep(cluster.GetLeaseDuration())
		}
	} else {
		slog.Info("leader election disabled")
		w.Start()
		<-ctx.Done()
	}

	slog.Info("Received signal to exit. Stopping.")
	w.Stop()
	engine.Stop()
}
