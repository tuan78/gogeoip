# gogeoip

[![CI](https://github.com/tuan78/gogeoip/actions/workflows/ci.yml/badge.svg)](https://github.com/tuan78/gogeoip/actions/workflows/ci.yml)
[![Coverage](https://github.com/tuan78/gogeoip/actions/workflows/coverage.yml/badge.svg)](https://github.com/tuan78/gogeoip/actions/workflows/coverage.yml)
[![codecov](https://codecov.io/gh/tuan78/gogeoip/branch/main/graph/badge.svg)](https://codecov.io/gh/tuan78/gogeoip)
[![Docker Image](https://github.com/tuan78/gogeoip/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/tuan78/gogeoip/actions/workflows/docker-publish.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/tuan78/gogeoip)](https://goreportcard.com/report/github.com/tuan78/gogeoip)
[![Go Version](https://img.shields.io/github/go-mod/go-version/tuan78/gogeoip)](go.mod)
[![Latest Release](https://img.shields.io/github/v/release/tuan78/gogeoip?include_prereleases)](https://github.com/tuan78/gogeoip/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight, self-contained GeoIP lookup HTTP service powered by the [MaxMind GeoLite2-Country](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) database.

## Features

- IP to country/continent lookup via REST API
- Zero-downtime hot-swap of the MaxMind database on scheduled refresh
- Optional Redis response caching (disabled when `REDIS_ADDR` is not set)
- Automatic request logging with method, path, status code, and duration for observability
- Pure `net/http` standard library — no heavy framework dependencies
- Graceful shutdown on `SIGINT`/`SIGTERM`
- Docker multi-stage build with minimal distroless runtime image
- Docker Compose setup with Redis
- Kubernetes manifests with health probes, resource limits, and optional Ingress
- Comprehensive test suite with high code coverage

## Prerequisites

- Go 1.25+
- A free [MaxMind account](https://www.maxmind.com/en/geolite2/signup) with an account ID and license key
- Docker & Docker Compose (for containerised runs)
- `kubectl` + a cluster (for Kubernetes deployment)

## Getting Started

```bash
git clone https://github.com/tuan78/gogeoip.git
cd gogeoip

# Export your MaxMind credentials (see .env.sample for all available variables)
export MAXMIND_ACCOUNT_ID=your-account-id
export MAXMIND_LICENSE_KEY=your-license-key

# Install dependencies
go mod download

# Build and run locally
make run
```

## Environment Variables

| Variable                  | Default                      | Description                                                                                                                                                               |
| ------------------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `PORT`                    | `8080`                       | HTTP port                                                                                                                                                                 |
| `MAXMIND_ACCOUNT_ID`      | _(required)_                 | MaxMind account ID (used as Basic Auth username)                                                                                                                          |
| `MAXMIND_LICENSE_KEY`     | _(required)_                 | MaxMind license key (used as Basic Auth password)                                                                                                                         |
| `GEO_DB_PATH`             | `/tmp/geolite2-country.mmdb` | Local path for the cached `.mmdb` file                                                                                                                                    |
| `GEO_DB_REFRESH_INTERVAL` | `24h`                        | How often to re-download the database (e.g. `12h`, `48h`)                                                                                                                 |
| `REDIS_ADDR`              | _(empty)_                    | Redis address (e.g. `localhost:6379`). Leave empty to disable caching                                                                                                     |
| `REDIS_PASSWORD`          | _(empty)_                    | Redis password (optional)                                                                                                                                                 |
| `REDIS_LOOKUP_KEY_PREFIX` | `gogeoip:lookup:`            | Full key prefix for `/lookup` cache entries. The final Redis key is `{prefix}{ip}` — e.g. `gogeoip:lookup:8.8.8.8`. Useful when sharing a Redis instance across services. |
| `REDIS_LOOKUP_CACHE_TTL`  | `24h`                        | How long a `/lookup` result is cached in Redis (e.g. `1h`, `12h`, `48h`). Set to `0s` to disable expiry.                                                                  |

## API

### Health check

```
GET /ping
```

Returns `200 OK` when the MaxMind database is loaded and the service is ready. Returns `503` otherwise.

**Response**

```json
{ "status": "ok" }
```

### IP lookup

```
GET /lookup?ip=<address>
```

**Query Parameters**

| Parameter | Required | Description                     |
| --------- | -------- | ------------------------------- |
| `ip`      | Yes      | IPv4 or IPv6 address to look up |

**Success response (200)**

With IPv4

```json
{
  "ip": "142.251.32.110",
  "country_code": "US",
  "country_name": "United States",
  "continent_code": "NA",
  "continent_name": "North America"
}
```

With IPv6

```json
{
  "ip": "2001:4860:4860::8888",
  "country_code": "US",
  "country_name": "United States",
  "continent_code": "NA",
  "continent_name": "North America"
}
```

**Error responses**

| Status | Reason                            |
| ------ | --------------------------------- |
| `400`  | Missing or invalid `ip` parameter |
| `503`  | MaxMind database not yet loaded   |
| `500`  | Internal lookup failure           |

## Logging

All HTTP requests are automatically logged to stdout as one-line JSON entries. This format is easier to ingest and query in Datadog, Loki, and Grafana.

```
{"timestamp":"2026-03-27T16:00:00.123456Z","level":"info","message":"http_request","service":"gogeoip","method":"GET","path":"/lookup","query":"ip=8.8.8.8","status_code":200,"duration_ms":5.67,"request_id":"req-123","trace_id":"trace-456"}
```

**Examples**

```
{"timestamp":"2026-03-27T16:00:00.123456Z","level":"info","message":"http_request","service":"gogeoip","method":"GET","path":"/ping","status_code":200,"duration_ms":1.23}
{"timestamp":"2026-03-27T16:00:01.123456Z","level":"info","message":"http_request","service":"gogeoip","method":"GET","path":"/lookup","query":"ip=8.8.8.8","status_code":200,"duration_ms":5.67}
{"timestamp":"2026-03-27T16:00:02.123456Z","level":"info","message":"http_request","service":"gogeoip","method":"GET","path":"/lookup","query":"ip=invalid","status_code":400,"duration_ms":0.23}
{"timestamp":"2026-03-27T16:00:03.123456Z","level":"info","message":"http_request","service":"gogeoip","method":"GET","path":"/ping","status_code":503,"duration_ms":0.09}
```

Core fields include `method`, `path`, `status_code`, and numeric `duration_ms`, with optional `request_id` and `trace_id` for correlation.

## Project Structure

```text
gogeoip/
├── cmd/
│   └── gogeoip/
│       └── main.go          # Binary entry point: wires config, DB, cache, and server
├── internal/
│   ├── cache/
│   │   ├── cache.go         # Cache interface, Redis and no-op implementations
│   │   └── cache_test.go    # Cache tests
│   ├── config/
│   │   ├── config.go        # Environment variable loading
│   │   └── config_test.go   # Config tests
│   ├── geo/
│   │   ├── database.go      # MaxMind DB download, hot-swap, and refresh loop
│   │   ├── geo.go           # IP lookup logic
│   │   └── geo_test.go      # Geo tests
│   ├── handlers/
│   │   ├── handler.go       # /lookup endpoint
│   │   ├── ping.go          # /ping health-check endpoint
│   │   └── handler_test.go  # Handler tests
│   ├── server/
│   │   ├── server.go        # HTTP server setup, graceful shutdown, request logging middleware
│   │   └── server_test.go   # Server tests
│   └── utils/
│       ├── duration.go      # Duration parsing for refresh intervals
│       └── duration_test.go # Duration tests
├── Dockerfile               # Multi-stage build → distroless runtime image
├── docker-compose.yml       # gogeoip + Redis, with named volume for DB cache
├── k8s.yml                  # Namespace, Secret, ConfigMap, Deployment, Service, Ingress
├── Makefile                 # Developer shortcuts (build, run, test, docker-*)
├── .env.sample              # Environment variable reference
├── go.mod
└── README.md
```

All packages under `internal/` are private to this module and cannot be imported by external code.

## Makefile Targets

| Target              | Description                                |
| ------------------- | ------------------------------------------ |
| `make build`        | Compile binary to `./gogeoip`              |
| `make run`          | Build and run                              |
| `make test`         | Run all tests                              |
| `make fmt`          | Format source files with `gofmt`           |
| `make vet`          | Run `go vet`                               |
| `make lint`         | Run `golangci-lint` (must be installed)    |
| `make docker-build` | Build Docker image tagged `gogeoip:latest` |
| `make docker-run`   | Run container on port `8080`               |
| `make docker-stop`  | Stop the running container                 |
| `make clean`        | Remove compiled binary                     |

## Docker

```bash
# Build image
make docker-build

# Run container
make docker-run

# Or use Docker Compose (includes Redis)
docker compose up --build
```

The image is built in two stages: `golang:1.25-alpine` compiles the binary; `distroless/static-debian12:nonroot` runs it — no shell, no extra packages, runs as non-root.

## Kubernetes

`k8s.yml` contains a single-file manifest with:

- **Namespace** `gogeoip`
- **Secret** — MaxMind credentials (base64-encode your values before applying)
- **ConfigMap** — all other env vars
- **PersistentVolumeClaim** — 100 Mi volume for the cached `.mmdb` file
- **Deployment** — 2 replicas, readiness/liveness probes on `/ping`, resource limits, read-only root filesystem
- **Service** — ClusterIP on port 80
- **Ingress** — optional, targets `gogeoip.example.com` (update host before applying)

### Step 1 — Build and push the Docker image

```bash
# Replace <registry>/<repo> with your own image path, e.g. docker.io/myorg/gogeoip
IMAGE=<registry>/<repo>/gogeoip:latest

docker build -t "$IMAGE" .
docker push "$IMAGE"
```

If you are using a **local cluster** (kind / minikube) you can load the image directly instead of pushing:

```bash
# kind
kind load docker-image gogeoip:latest --name <your-cluster-name>

# minikube
minikube image load gogeoip:latest
```

Then set `image: gogeoip:latest` and `imagePullPolicy: Never` in `k8s.yml`.

### Step 2 — Encode your MaxMind credentials

The `Secret` in `k8s.yml` requires base64-encoded values:

```bash
echo -n '<your-account-id>'   | base64   # copy → MAXMIND_ACCOUNT_ID in k8s.yml
echo -n '<your-license-key>'  | base64   # copy → MAXMIND_LICENSE_KEY in k8s.yml
```

Open `k8s.yml` and replace the placeholder strings:

```yaml
data:
  MAXMIND_ACCOUNT_ID: "<base64-encoded-account-id>" # ← paste output here
  MAXMIND_LICENSE_KEY: "<base64-encoded-license-key>" # ← paste output here
```

### Step 3 — Update the image reference

In `k8s.yml`, find the Deployment container spec and set the image you pushed in Step 1:

```yaml
image: <registry>/<repo>/gogeoip:latest
```

### Step 4 — Deploy

```bash
kubectl apply -f k8s.yml
```

### Step 5 — Verify the rollout

```bash
# Watch pods come up (Ctrl-C when all are Running/Ready)
kubectl rollout status deployment/gogeoip -n gogeoip

# Check pod health
kubectl get pods -n gogeoip

# Tail logs from one pod
kubectl logs -n gogeoip -l app=gogeoip --follow
```

### Step 6 — Test the service

#### Option A — port-forward (quick local test)

```bash
kubectl port-forward svc/gogeoip-svc 8080:80 -n gogeoip
```

Then in another terminal:

```bash
curl "http://localhost:8080/ping"
curl "http://localhost:8080/lookup?ip=8.8.8.8"
```

#### Option B — Ingress

1. Make sure an ingress controller (e.g. `ingress-nginx`) is installed in your cluster.
2. Edit the `Ingress` resource in `k8s.yml` and set your real hostname:
   ```yaml
   host: gogeoip.example.com
   ```
3. Apply the manifest (or re-apply if already applied):
   ```bash
   kubectl apply -f k8s.yml
   ```
4. Add a DNS record (or `/etc/hosts` entry for local testing) pointing the hostname to your ingress controller's external IP.
5. Send a request:
   ```bash
   curl "http://gogeoip.example.com/lookup?ip=8.8.8.8"
   ```

To enable TLS, uncomment and populate the `tls` block in the Ingress resource and create the corresponding `Secret` (or use cert-manager).

### Teardown

```bash
# Remove all gogeoip resources (namespace and everything in it)
kubectl delete namespace gogeoip
```

## License

MIT
