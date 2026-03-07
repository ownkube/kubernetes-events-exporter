# Kubernetes Event Exporter

> **Actively maintained fork** of [resmoio/kubernetes-events-exporter](https://github.com/resmoio/kubernetes-events-exporter) (itself a fork of the original [Opsgenie exporter](https://github.com/opsgenie/kubernetes-events-exporter)). The upstream repository has been dormant since Resmo was acquired by JumpCloud in 2024. This fork consolidates the best fixes and features from multiple community forks (ClickHouse, LinkedIn, honestica) into a single, maintained project.

This tool allows exporting the often missed Kubernetes events to various outputs so that they can be used for
observability or alerting purposes. You won't believe what you are missing.

## What's New in This Fork

### Reliability Fixes
- **Update event handling** — The watcher now processes event updates (not just additions), fixing the #1 cause of missing events. Previously, when Kubernetes aggregated events and incremented `count`, the update was silently dropped. *(ported from ClickHouse fork)*
- **Improved event age calculation** — Uses `max(creationTimestamp, lastTimestamp)` to determine event age, preventing false discards when timestamps are inconsistent. *(ported from LinkedIn fork)*
- **Cache lookup validation** — Validates that `UID` and `ResourceVersion` exist before using them as cache keys, preventing panics on events with incomplete references. *(ported from LinkedIn fork)*
- **Loki TLS transport fix** — The Loki sink now correctly uses its configured TLS transport instead of ignoring it.

### New Features
- **Prometheus sink** — Convert Kubernetes events into Prometheus gauge metrics, filterable by resource kind and reason. Enables alerting on events via Prometheus/Grafana.
- **Loki basic auth** — New `username` and `password` fields for Loki, supporting Grafana Enterprise/Cloud.
- **Loki stream label templates** — Stream labels now support Go templates (e.g., `"{{ .InvolvedObject.Namespace }}"`).
- **ClusterEnvironment field** — New config option to tag events with an environment label (prod, staging, etc.).
- **SNS FIFO support** — New `messageGroupId` field enables ordered delivery via AWS SNS FIFO topics.

### Build & Compatibility
- Restructured codebase from `pkg/` to `cmd/` + `internal/` following standard Go project layout
- Updated to Go 1.26, Elasticsearch client v8, IBM/sarama (Kafka)
- Fixed pre-existing build issues (LRU cache API, ES v8 compatibility)

## Deployment

### Helm

```bash
# Install from this repo
helm install kubernetes-events-exporter ./charts/kubernetes-events-exporter \
  --namespace monitoring --create-namespace

# Or with custom values
helm install kubernetes-events-exporter ./charts/kubernetes-events-exporter \
  --namespace monitoring --create-namespace \
  -f my-values.yaml
```

Minimal `my-values.yaml` to send events to Loki:

```yaml
config:
  logLevel: info
  logFormat: json
  maxEventAgeSeconds: 60
  route:
    routes:
      - match:
          - receiver: "loki"
  receivers:
    - name: "loki"
      loki:
        url: http://loki.monitoring:3100/loki/api/v1/push
        streamLabels:
          app: kube-events
          namespace: "{{ .InvolvedObject.Namespace }}"
```

See [`charts/kubernetes-events-exporter/values.yaml`](charts/kubernetes-events-exporter/values.yaml) for all options including resources, tolerations, serviceMonitor, and leader election.

> **Note:** The Bitnami Helm chart was deprecated in August 2025. This is the replacement first-party chart.

## Configuration

Configuration is done via a YAML file (ConfigMap when running in Kubernetes). The tool watches all events and lets you filter, route, and deliver them to various sinks.

### Global Options

```yaml
logLevel: info              # debug | info | warn | error
logFormat: json             # json | text
maxEventAgeSeconds: 60      # discard events older than this (default: 5)
kubeQPS: 100                # K8s API rate limit (QPS)
kubeBurst: 500              # K8s API rate limit (burst)
namespace: ""               # watch a specific namespace (default: all)
clusterName: "prod-us-1"    # optional, added to every event as clusterName
clusterEnvironment: "prod"  # optional, added to every event as clusterEnvironment
metricsNamePrefix: "event_exporter_"  # prefix for Prometheus metrics
omitLookup: false           # skip enriching events with object metadata
cacheSize: 1024             # LRU cache size for object metadata
leaderElection:
  enabled: true             # enable leader election for HA deployments
  leaderElectionID: "kubernetes-events-exporter"
```

### Routing

The routing configuration is a tree that allows flexible event filtering and delivery.

```yaml
route:
  routes:
    # Dump all events
    - match:
        - receiver: dump
    # Drop test namespaces and Normal events, send the rest to alerting
    - drop:
        - namespace: "test-.*"
        - type: "Normal"
      match:
        - receiver: "critical-events-queue"
    # Route specific kinds to owners via Slack
    - match:
        - kind: "Pod|Deployment|ReplicaSet"
          labels:
            version: "dev"
          receiver: "slack"
```

- A `match` rule is exclusive: all conditions must match.
- `drop` rules are evaluated first to filter out events.
- `match` rules in a route are independent. If an event matches a rule, it goes down its subtree.
- Routes can have sub-routes, forming a tree. Routing starts from the root.

### Using Secrets

Reference environment variables in config with `${VAR_NAME}`:

```yaml
receivers:
  - name: "es"
    elasticsearch:
      apiKey: ${ELASTIC_API_KEY}
```

## Receivers

### Elasticsearch

```yaml
receivers:
  - name: "dump"
    elasticsearch:
      hosts:
        - http://localhost:9200
      index: kube-events
      indexFormat: "kube-events-{2006-01-02}"  # time-based indices
      username: # optional
      password: # optional
      cloudID: # optional
      apiKey: # optional
      headers: # optional
      useEventID: true|false  # update same doc (useful for event count)
      deDot: true|false       # replace dots in label keys with underscores
      layout: # optional, custom payload
      tls:
        insecureSkipVerify: true|false
        serverName: # optional
        caFile: # optional
```

### OpenSearch

```yaml
receivers:
  - name: "dump"
    opensearch:
      hosts:
        - http://localhost:9200
      index: kube-events
      indexFormat: "kube-events-{2006-01-02}"
      username: # optional
      password: # optional
      useEventID: true|false
      deDot: true|false
      layout: # optional
      tls:
        insecureSkipVerify: true|false
        serverName: # optional
        caFile: # optional
```

### Loki

Supports basic auth, TLS, custom headers, and **Go template stream labels**.

```yaml
receivers:
  - name: "loki"
    loki:
      url: http://127.0.0.1:3100/loki/api/v1/push
      username: # optional, for basic auth
      password: # optional, for basic auth
      headers: # optional
        X-Scope-OrgID: tenantID
      streamLabels:
        app: kube-events
        # Go templates are supported in stream labels:
        namespace: "{{ .InvolvedObject.Namespace }}"
        kind: "{{ .InvolvedObject.Kind }}"
      layout: # optional
      tls:
        insecureSkipVerify: true|false  # for self-signed certs
        caFile: # optional
```

### Prometheus

Expose Kubernetes events as Prometheus gauge metrics. Useful for alerting on events via Prometheus/Grafana without needing a separate log pipeline.

```yaml
receivers:
  - name: "prometheus"
    prometheus:
      eventsMetricsNamePrefix: "event_exporter_"  # default
      reasonFilter:
        Pod:
          - "FailedMount"
          - "CrashLoopBackOff"
          - "OOMKilled"
        Node:
          - "NotReady"
          - "MemoryPressure"
```

This creates gauge metrics like `event_exporter_pod_event_count{pod="nginx-abc", namespace="default", reason="CrashLoopBackOff"}`.

### Opsgenie

```yaml
receivers:
  - name: "alerts"
    opsgenie:
      apiKey: xxx
      priority: "P3"
      message: "Event {{ .Reason }} for {{ .InvolvedObject.Namespace }}/{{ .InvolvedObject.Name }}"
      alias: "{{ .UID }}"
      description: "<pre>{{ toPrettyJson . }}</pre>"
      tags:
        - "event"
        - "{{ .Reason }}"
        - "{{ .InvolvedObject.Kind }}"
```

### Slack

```yaml
receivers:
  - name: "slack"
    slack:
      token: YOUR-API-TOKEN-HERE
      channel: "#alerts"
      message: "{{ .Message }}"
      color: # optional
      title: # optional
      fields:
        namespace: "{{ .Namespace }}"
        reason: "{{ .Reason }}"
```

### Webhook

```yaml
receivers:
  - name: "alerts"
    webhook:
      endpoint: "https://my-service.com/events"
      headers:
        X-API-KEY: "123"
        User-Agent: kubernetes-events-exporter 1.0
      layout: # optional
```

### Kafka

```yaml
receivers:
  - name: "kafka"
    kafka:
      clientId: "kubernetes"
      topic: "kube-event"
      brokers:
        - "localhost:9092"
      compressionCodec: "snappy"
      tls:
        enable: true
        certFile: "kafka-client.crt"
        keyFile: "kafka-client.key"
        caFile: "kafka-ca.crt"
      sasl:
        enable: true
        username: "producer"
        password: "password"
        mechanism: "sha512"  # plain, sha256, sha512
      layout: # optional
```

### Kinesis

```yaml
receivers:
  - name: "kinesis"
    kinesis:
      streamName: "events-pipeline"
      region: us-west-2
      layout: # optional
```

### Firehose

```yaml
receivers:
  - name: "firehose"
    firehose:
      deliveryStreamName: "events-pipeline"
      region: us-west-2
      layout: # optional
```

### SNS

Supports standard and FIFO topics.

```yaml
receivers:
  - name: "sns"
    sns:
      topicARN: "arn:aws:sns:us-east-1:123456789:mytopic"
      region: "us-west-2"
      messageGroupId: "k8s-events"  # optional, for FIFO topics
      layout: # optional
```

### SQS

```yaml
receivers:
  - name: "sqs"
    sqs:
      queueName: "kube-events"
      region: us-west-2
      layout: # optional
```

### Pub/Sub

```yaml
receivers:
  - name: "pubsub"
    pubsub:
      gcloud_project_id: "my-project"
      topic: "kube-event"
      create_topic: false
```

### Teams

```yaml
receivers:
  - name: "ms_teams"
    teams:
      endpoint: "https://outlook.office.com/webhook/..."
      layout: # optional
```

### EventBridge

```yaml
receivers:
  - name: "eventbridge"
    eventbridge:
      detailType: "deployment"
      source: "cd"
      eventBusName: "default"
      region: "ap-southeast-1"
      details:
        message: "{{ .Message }}"
        namespace: "{{ .Namespace }}"
```

### BigQuery

```yaml
receivers:
  - name: "bigquery"
    bigquery:
      location: "US"
      project: "my-project"
      dataset: "kube_events"
      table: "events"
      credentials_path: "/secrets/gcp.json"
      batch_size: 100
      max_retries: 3
      interval_seconds: 30
      timeout_seconds: 60
```

### File

```yaml
receivers:
  - name: "file"
    file:
      path: "/tmp/events.log"
      layout: # optional
```

### Stdout

```yaml
receivers:
  - name: "dump"
    stdout:
      deDot: true|false
```

### Pipe

```yaml
receivers:
  - name: "pipe"
    pipe:
      path: "/dev/stdout"
      deDot: true|false
```

### Syslog

```yaml
receivers:
  - name: "syslog"
    syslog:
      network: "tcp"
      address: "127.0.0.1:11514"
      tag: "k8s.event"
```

### OpsCenter

```yaml
receivers:
  - name: "alerts"
    opscenter:
      title: "{{ .Message }}"
      category: "{{ .Reason }}"
      description: "Event in {{ .InvolvedObject.Namespace }}/{{ .InvolvedObject.Name }}"
      region: "us-east-1"
      source: "production"
      severity: "6"           # optional
      priority: "6"           # optional
      notifications:          # optional, SNS ARNs
        - "arn:aws:sns:..."
```

## Customizing Payload

Receivers that support `layout` allow custom payloads using Go templates with [Sprig](https://masterminds.github.io/sprig/) functions. This supports recursive map definitions for building arbitrary JSON:

```yaml
layout:
  eventType: "kubernetes-event"
  createdAt: "{{ .GetTimestampMs }}"
  details:
    message: "{{ .Message }}"
    reason: "{{ .Reason }}"
    type: "{{ .Type }}"
    count: "{{ .Count }}"
    kind: "{{ .InvolvedObject.Kind }}"
    name: "{{ .InvolvedObject.Name }}"
    namespace: "{{ .Namespace }}"
    labels: "{{ toJson .InvolvedObject.Labels }}"
```

## Troubleshooting

### "Events Discarded" warnings

1. If you see `client-side throttling` warnings, increase API rate limits:
   ```yaml
   kubeQPS: 100    # ~1/5 of burst
   kubeBurst: 500  # roughly match your events per minute
   ```

2. If events are still dropped without throttling, increase the age cutoff:
   ```yaml
   maxEventAgeSeconds: 60
   ```

### "Object not found" errors

This happens when the involved object is deleted before metadata can be enriched. These events are still processed but without labels/annotations. If you see this constantly, consider setting `omitLookup: true` to skip metadata enrichment entirely.

### Missing events

If you notice events are missing, ensure the watcher is processing updates (this fork fixes this by default). For high-volume clusters, also check:
- `kubeQPS` and `kubeBurst` are high enough
- `maxEventAgeSeconds` is set appropriately (too low = events dropped, too high = stale events on restart)
- Leader election is configured for HA deployments to avoid duplicate/missed events

## Building

```bash
make build        # go build (runs tidy + vet first)
make test         # go test -cover -v ./...
make fmt          # gofmt -s -l -w .
make build-image  # docker build
```

## License

This project is licensed under the Apache License 2.0 - see the original repository for details.
