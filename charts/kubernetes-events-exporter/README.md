# kubernetes-events-exporter Helm Chart

Export Kubernetes events to multiple destinations with routing and filtering.

## Installation

```bash
helm install kubernetes-events-exporter ./charts/kubernetes-events-exporter \
  --namespace monitoring --create-namespace
```

With custom values:

```bash
helm install kubernetes-events-exporter ./charts/kubernetes-events-exporter \
  --namespace monitoring --create-namespace \
  -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall kubernetes-events-exporter --namespace monitoring
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `replicaCount` | int | `1` | Number of replicas |
| `image.repository` | string | `ghcr.io/ownkube/kubernetes-events-exporter` | Container image repository |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `image.tag` | string | `""` | Image tag (defaults to `appVersion`) |
| `imagePullSecrets` | list | `[]` | Image pull secrets |
| `nameOverride` | string | `""` | Override chart name |
| `fullnameOverride` | string | `""` | Override fully qualified app name |
| `serviceAccount.create` | bool | `true` | Create a ServiceAccount |
| `serviceAccount.annotations` | object | `{}` | ServiceAccount annotations (e.g. for IAM roles) |
| `serviceAccount.name` | string | `""` | ServiceAccount name (generated from fullname if empty) |
| `rbac.create` | bool | `true` | Create the RBAC objects below (see [Permissions](#permissions)) |
| `rbac.rules` | list | `get` on common kinds | Read access used to add the involved object's labels, annotations and owners to each event. Secrets and ConfigMaps are not included |
| `rbac.extraRules` | list | `[]` | Extra rules appended to `rbac.rules`, for example CRDs your events reference |
| `podAnnotations` | object | Prometheus scrape annotations | Annotations added to pods |
| `podLabels` | object | `{}` | Labels added to pods |
| `podSecurityContext` | object | `runAsNonRoot: true` | Pod-level security context |
| `securityContext` | object | Drop all capabilities, no privilege escalation | Container-level security context |
| `resources` | object | `{}` | CPU/memory requests and limits |
| `nodeSelector` | object | `{}` | Node selector |
| `tolerations` | list | `[]` | Tolerations |
| `affinity` | object | `{}` | Affinity rules |
| `service.enabled` | bool | `true` | Create a Service for metrics scraping |
| `service.port` | int | `2112` | Metrics service port |
| `serviceMonitor.enabled` | bool | `false` | Create a Prometheus ServiceMonitor |
| `serviceMonitor.interval` | string | `nil` | Scrape interval |
| `serviceMonitor.additionalLabels` | object | `nil` | Extra labels on the ServiceMonitor |
| `leaderElection.enabled` | bool | `false` | Enable leader election for HA deployments. Sets `config.leaderElection.enabled` and creates a Role for the lease |
| `extraEnv` | list | `[]` | Extra environment variables (for secrets, see example below) |
| `extraVolumes` | list | `[]` | Extra volumes (e.g. TLS certs) |
| `extraVolumeMounts` | list | `[]` | Extra volume mounts |
| `config` | object | See below | Event exporter configuration (becomes the ConfigMap) |

### Config

The `config` block maps directly to the exporter's YAML configuration:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `config.logLevel` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `config.logFormat` | string | `json` | Log format: `json`, `text` |
| `config.maxEventAgeSeconds` | int | `60` | Discard events older than this |
| `config.metricsNamePrefix` | string | `event_exporter_` | Prefix for Prometheus metrics |
| `config.clusterName` | string | `""` | Tag events with a cluster name |
| `config.clusterEnvironment` | string | `""` | Tag events with an environment label |
| `config.kubeQPS` | int | `nil` | K8s API rate limit (QPS) |
| `config.kubeBurst` | int | `nil` | K8s API rate limit (burst) |
| `config.namespace` | string | `""` | Watch one namespace only. RBAC becomes a Role in that namespace |
| `config.omitLookup` | bool | `false` | Skip metadata enrichment. RBAC is reduced to reading events |
| `config.route` | object | Match all to `dump` | Routing tree (see main README for details) |
| `config.receivers` | list | stdout dump | List of receiver/sink configurations |

## Permissions

The chart always grants `get`, `list` and `watch` on events. By default it also
grants `get` on the objects events are usually about (pods, nodes, services,
endpoints, PVCs, PVs, namespaces, deployments, replica sets, stateful sets,
daemon sets, jobs, cron jobs, HPAs, PDBs, ingresses, endpoint slices), so each
event can carry that object's labels, annotations and owner references.

It never grants access to Secrets or ConfigMaps. Events about a kind that is not
in the list are still exported, just without that metadata.

- To enrich events about CRDs, add them with `rbac.extraRules`.
- To grant events only, set `config.omitLookup: true`.
- To stay inside one namespace, set `config.namespace`. The chart then creates a
  Role and RoleBinding in that namespace instead of a ClusterRole.
- To manage RBAC yourself, set `rbac.create: false` and point
  `serviceAccount.name` at an existing ServiceAccount.

With leader election on, a separate Role in the release namespace allows `get`,
`create` and `update` on leases.

```yaml
rbac:
  extraRules:
    - apiGroups: ["cert-manager.io"]
      resources: [certificates]
      verbs: ["get"]
```

## Examples

### Send events to Loki

```yaml
config:
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

### Send events to Elasticsearch with secrets

```yaml
extraEnv:
  - name: ELASTIC_API_KEY
    valueFrom:
      secretKeyRef:
        name: elastic-secret
        key: api-key

config:
  route:
    routes:
      - match:
          - receiver: "es"
  receivers:
    - name: "es"
      elasticsearch:
        hosts:
          - https://elasticsearch.monitoring:9200
        index: kube-events
        indexFormat: "kube-events-{2006-01-02}"
        apiKey: ${ELASTIC_API_KEY}
```

### HA deployment with leader election

```yaml
replicaCount: 2
leaderElection:
  enabled: true

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

### With ServiceMonitor for Prometheus Operator

```yaml
serviceMonitor:
  enabled: true
  interval: 30s
  additionalLabels:
    release: kube-prometheus-stack
```
