# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Actively maintained fork (by ownkube) of the dormant `resmoio/kubernetes-events-exporter`. Go-based Kubernetes event exporter that watches cluster events and routes them to 30+ configurable sinks (Kafka, Elasticsearch, Slack, webhooks, AWS services, GCP Pub/Sub, etc.). The codebase has been restructured from the upstream `pkg/` layout into `cmd/` + `internal/` following standard Go project layout.

## Build & Development Commands

```bash
make build        # go build (runs tidy + vet first)
make test         # go test -cover -mod=mod -v ./...
make fmt          # gofmt -s -l -w .
make vet          # go vet ./...
make tidy         # go mod tidy
make build-image  # docker build -t kubernetes-events-exporter
```

Run a single test:
```bash
go test -v -run TestFunctionName ./internal/routing/
```

## Architecture

### Event Pipeline Flow

```
K8s API (SharedInformer) -> EventWatcher -> Engine -> Route Tree -> Sinks
```

1. **cmd/exporter/main.go**: Entry point. Parses flags, loads config with env var expansion (`${VAR}`), sets up K8s client, optional leader election, starts watcher + engine.

2. **internal/cluster/**: K8s client setup and leader election (formerly `pkg/kube/client.go`, `leaderelection.go`).

3. **internal/watcher/**: SharedInformer-based event listener with age filtering, object metadata enrichment via LRU cache (formerly `pkg/kube/watcher.go`).

4. **internal/event/**: `EnhancedEvent` wraps `corev1.Event` with enriched metadata and helpers (`DeDot`, `GetTimestampMs`).

5. **internal/config/**: YAML config parsing with env var substitution.

6. **internal/routing/**: Route tree with Drop/Match rules, regex matching on Kind, Namespace, Reason, Type, etc. All conditions are AND logic (formerly `pkg/exporter/route.go`, `rule.go`).

7. **internal/pipeline/**: Engine that instantiates sinks from config, registers them, routes events (formerly `pkg/exporter/engine.go`).

8. **internal/dispatch/**: Channel and sync registries for sink delivery.

9. **internal/output/**: Sink implementations. Each sink implements `Sink` interface (`Send` + `Close`). Includes template rendering via Sprig (formerly `pkg/sinks/`).

10. **internal/observability/**: Prometheus metrics (`/metrics`, `/-/healthy`, `/-/ready`).

11. **internal/buildinfo/**: Version information.

### Adding a New Sink

1. Create `internal/output/newsink.go` implementing `Sink` interface
2. Add config struct and field to `ReceiverConfig` in `internal/output/factory.go`
3. Add instantiation case in the factory's sink builder

### Configuration

YAML-based config with env var substitution. Key structure:
- `route.routes[].match/drop`: Regex-based rules selecting receivers
- `receivers[].name` + sink-specific config block

### Deployment

Helm chart in `charts/kubernetes-events-exporter/`. Runs as non-root on distroless image.

## Key Dependencies

- Go 1.26, K8s client-go v0.26.7, ES client v8
- slog (logging), Sprig (templates), IBM/sarama (Kafka), go-yaml (config parsing)
- CI runs `go build` + `go test` on push/PR; releases publish multi-arch images to GHCR

## Improvements Over Upstream (Completed)

- Update event handler (fixes event loss) -- from ClickHouse fork
- Better event age calculation (max of timestamps) -- from LinkedIn fork
- Cache lookup validation (empty UID/ResourceVersion) -- from LinkedIn fork
- ClusterEnvironment config field -- from LinkedIn fork
- Loki: basic auth, stream label templates, TLS transport fix
- Prometheus sink (events to metrics) -- from honestica fork
- SNS FIFO topic support -- from honestica fork
- ES v8 client compatibility fix

## Remaining Issues

1. **OpenSearch/ES**: AWS IAM/Pod Identity needs SDK v2, no template in `indexFormat`.
2. **Kafka Auth**: SASL SHA256/SHA512 mechanism may still have issues. Kerberos not supported.
3. **stdout Sink**: `/dev/stdout` rename error on distroless causes sink failure.
4. **Routing**: Cannot match on `InvolvedObject.Name`, `minCount` broken.
5. **New Sinks**: Fluentd, Feishu/Lark still requested.
