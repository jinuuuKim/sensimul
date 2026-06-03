# SenSimul Usage Guide

## 1) Prerequisites

- Go 1.23+
- Docker + Docker Compose (for container run)

## 2) Local Build

```bash
make build
```

Binary output:

```text
build/sensimul
```

## 3) Core Commands

All commands support `--config` (default: `config/sensimul.yaml`).

### Health Check

```bash
./build/sensimul health --config config/sensimul.yaml
```

### Site Management

```bash
./build/sensimul site add --id SEOUL_WH --name "Seoul Warehouse" --type indoor --lat 37.5665 --lon 126.9780
./build/sensimul site list
```

### Sensor Management

```bash
./build/sensimul sensor add --site SEOUL_WH --id TEMP_001 --type temperature
./build/sensimul sensor add --site SEOUL_WH --id HUM_001 --type humidity
./build/sensimul sensor list --site SEOUL_WH
```

### Controller Management

```bash
./build/sensimul controller add --site SEOUL_WH --id COOL_001 --type cooling
./build/sensimul controller list --site SEOUL_WH
```

### Run Simulation

```bash
./build/sensimul run --config config/sensimul.yaml
```

MQTT topic format:

```text
sensimul/sites/{site_id}/sensors/{sensor_id}
```

## 4) Docker Run

### MQTT only

```bash
cp .env.example .env
# Set MQTT_WS_USERNAME and MQTT_WS_PASSWORD in .env before starting.
docker compose -f docker-compose.mqtt.yml up -d
```

The MQTT broker exposes:

| Protocol | Port | Authentication |
|---|---:|---|
| MQTT TCP | `1883` | anonymous, for local simulator/web services |
| MQTT over WebSocket | `9001` | username/password from `.env` |

MQTT over WebSocket clients can subscribe to live telemetry with:

```text
sensimul/sites/{site_id}/sensors/+
```

### Full stack (MQTT + SenSimul)

```bash
cp .env.example .env
# Set MQTT_WS_USERNAME and MQTT_WS_PASSWORD in .env before starting.
docker compose up -d --build
docker compose logs -f
```

### Optional web profile (separate web service)

```bash
docker compose --profile web up -d --build
curl -f http://localhost:18080/healthz
```

Web UI routes:

- `/sites` site/sensor/controller CRUD entry
- `/live` real-time sensor overview
- `/docs/manual` user manual page
- `/docs/mqtt` MQTT integration manual page

## 5) DietPi Deployment (192.168.0.11)

1. Copy repository to server.
2. Ensure Docker and Docker Compose plugin are installed.
3. Run:

```bash
docker compose up -d --build
```

4. Verify service:

```bash
docker compose ps
docker compose logs --tail=100 sensimul-app
```
