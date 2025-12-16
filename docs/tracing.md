# OpenTelemetry Tracing Configuration

The LFX v2 Meeting Service supports distributed tracing via OpenTelemetry (OTEL). Traces can be exported to any OTLP-compatible collector such as Jaeger, Grafana Tempo, or the OpenTelemetry Collector.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_SERVICE_NAME` | Service name for resource identification | `lfx-v2-meeting-service` |
| `OTEL_SERVICE_VERSION` | Service version for resource identification | `""` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP protocol: `grpc` or `http` | `grpc` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | (none) |
| `OTEL_EXPORTER_OTLP_INSECURE` | Disable TLS for OTLP connection | `false` |
| `OTEL_TRACES_EXPORTER` | Traces exporter: `otlp` or `none` | `none` |
| `OTEL_TRACES_SAMPLE_RATIO` | Sampling ratio for traces (0.0 to 1.0) | `1.0` |
| `OTEL_METRICS_EXPORTER` | Metrics exporter: `otlp` or `none` | `none` |
| `OTEL_LOGS_EXPORTER` | Logs exporter: `otlp` or `none` | `none` |

## Enabling Tracing

Tracing is disabled by default. To enable it, set `OTEL_TRACES_EXPORTER=otlp` and configure the endpoint.

### Local Development with Jaeger

1. Start Jaeger with OTLP support:

   ```bash
   docker run -d --name jaeger \
     -p 16686:16686 \
     -p 4317:4317 \
     -p 4318:4318 \
     jaegertracing/all-in-one:latest
   ```

2. Configure the service:

   ```bash
   export OTEL_TRACES_EXPORTER=otlp
   export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
   export OTEL_EXPORTER_OTLP_INSECURE=true
   ```

3. Run the service:

   ```bash
   make run
   ```
   <!-- markdown-link-check-disable-next-line -->
4. View traces at [http://localhost:16686](http://localhost:16686)

### Kubernetes Deployment

The Helm chart includes commented OTEL configuration in `values.yaml`. To enable tracing, uncomment and configure the OTEL environment variables:

```yaml
app:
  environment:
    OTEL_SERVICE_NAME:
      value: lfx-v2-meeting-service
    OTEL_EXPORTER_OTLP_PROTOCOL:
      value: grpc
    OTEL_EXPORTER_OTLP_ENDPOINT:
      value: jaeger-collector.observability.svc.cluster.local:4317
    OTEL_EXPORTER_OTLP_INSECURE:
      value: "true"
    OTEL_TRACES_EXPORTER:
      value: otlp
```

Or override via Helm install:

```bash
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --set app.environment.OTEL_TRACES_EXPORTER.value=otlp \
  --set app.environment.OTEL_EXPORTER_OTLP_ENDPOINT.value=jaeger-collector.observability.svc.cluster.local:4317 \
  --set app.environment.OTEL_EXPORTER_OTLP_INSECURE.value=true
```

## Protocol Selection

The service supports both gRPC and HTTP protocols for OTLP export:

| Protocol | Default Port | Use Case |
|----------|--------------|----------|
| `grpc` | 4317 | Recommended for most deployments |
| `http` | 4318 | Use when gRPC is blocked or unavailable |

## Metrics and Logs

In addition to traces, the service can export metrics and logs via OTLP:

```bash
# Enable metrics export
export OTEL_METRICS_EXPORTER=otlp

# Enable logs export
export OTEL_LOGS_EXPORTER=otlp
```

These use the same endpoint and protocol configuration as traces.

## Troubleshooting

### No traces appearing

1. Verify `OTEL_TRACES_EXPORTER` is set to `otlp`
2. Check the endpoint is reachable from the service
3. For in-cluster communication, ensure `OTEL_EXPORTER_OTLP_INSECURE=true` if not using TLS
4. Check service logs for OTLP connection errors

### Connection refused errors

- Verify the collector is running and accessible
- Check firewall rules allow traffic on the OTLP port (4317 for gRPC, 4318 for HTTP)
- For Kubernetes, verify the service DNS name resolves correctly

## Trace Sampling

By default, all traces are sampled (ratio of 1.0). In production environments with high traffic, you may want to reduce the sampling ratio to decrease costs and storage requirements while still maintaining visibility.

```bash
# Sample 10% of traces
export OTEL_TRACES_SAMPLE_RATIO=0.1

# Sample 50% of traces
export OTEL_TRACES_SAMPLE_RATIO=0.5

# Sample all traces (default)
export OTEL_TRACES_SAMPLE_RATIO=1.0
```

The sampling ratio must be a value between 0.0 and 1.0. Invalid values will be ignored and the default of 1.0 will be used.

## Database Tracing

All NATS KV store operations are automatically traced when tracing is enabled. The following database operations create spans:

| Operation | Span Name | Description |
|-----------|-----------|-------------|
| Get | `nats.kv.get` | Retrieve an entity from the store |
| Put/Create | `nats.kv.put` | Create a new entity in the store |
| Update | `nats.kv.update` | Update an existing entity |
| Delete | `nats.kv.delete` | Remove an entity from the store |
| List Keys | `nats.kv.list_keys` | List all keys in a bucket |

Each span includes the following attributes:
- `db.system`: Always "nats"
- `db.operation`: The operation type (get, put, update, delete, list_keys)
- `db.nats.key`: The key being accessed (when applicable)
- `db.nats.entity`: The entity type (meeting, registrant, etc.)
- `db.nats.revision`: The revision number (for update/delete operations)
- `db.nats.keys_count`: Number of keys returned (for list operations)
