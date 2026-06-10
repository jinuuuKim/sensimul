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

## 3-1) Weather Integration (기상청 / KMA)

Outdoor sites can be driven by real observations from the KMA API Hub
(지상관측 ASOS 시간자료, `kma_sfctm3.php`).

### Configuration

```yaml
weather:
  mode: kma                                                        # synthetic | kma
  api_key: "${SENSIMUL_WEATHER_API_KEY}"                           # 기상청 API Hub authKey
  base_url: https://apihub.kma.go.kr/api/typ01/url/kma_sfctm2.php
  station: "108"                                                   # ASOS 지점번호 (108=서울)
  ttl: 3600s                                                       # 요청/캐시 주기 (ASOS는 매시간 갱신)
  timeout: 10s
```

Get a free authKey at <https://apihub.kma.go.kr/>.

### Behaviour and scope

- **Both site types.** Weather is fetched for indoor and outdoor sites.
  - **Outdoor:** runs directly on the observation; the first fetch also seeds the
    current value so the site starts from real outdoor conditions.
  - **Indoor:** keeps its configured baseline as the starting point. When a
    controller on an axis (cooling/heating/humidify/dehumidify) is **off**, the
    value relaxes toward the KMA observation (조절기 off → 기상청 값 수렴); when
    **on**, the controller biases the equilibrium away from it.
- **Evidence values, not stored telemetry.** The KMA response is not persisted.
  Each observation only seeds or adjusts the simulation engines' base values:
  - **First fetch** → initializes temperature/humidity *current and ambient*, and
    sets pressure. A freshly configured site starts from real conditions
    ("초기 기반 값").
  - **Later fetches** → adjust only the *ambient* targets; simulated values drift
    toward the new observation via the physics model ("주기적으로 값 조정").
- **Fields:** ASOS provides temperature (TA), humidity (HM), local pressure
  (PA, 현지기압), and wind speed (WS). Temperature, humidity, and pressure are
  wired into the simulation. **Wind is captured into the snapshot but not yet
  consumed** by any engine/sensor channel. **PM2.5/PM10 are NOT provided by this
  endpoint** and remain fully simulated (a separate 황사/PM API would be needed).
- **Request cycle:** ASOS hourly data is requested for the *last completed hour*
  in KST. `ttl` (default `3600s`) controls how often the upstream is actually
  called; ticks in between are served from cache.
- **Failure handling:** if a fetch fails, the last good observation is retained
  (the simulation does not jump back to synthetic defaults); synthetic baseline
  is used only when no successful fetch has ever happened.
- **Station selection:** ASOS is queried by `stn`, not lat/lon. Set
  `weather.station` (or `SENSIMUL_WEATHER_STATION`). Run one site per config with
  `run --site <id>` to use a station per site.

> **Verification status: verified.** The parser column indices
> (`internal/weather/kma.go`: TA=11, HM=13, PA=7, WS=3) and the single-`tm`
> request form against `kma_sfctm2.php` are confirmed against a real station-108
> response (the fixture in `kma_test.go` is that response). Missing values use the
> `-9 / -9.0` sentinel (also confirmed live) and are skipped per field. Pressure
> uses **PA (현지기압, local/station pressure)**, e.g. ~999 hPa at Seoul's
> elevation — a site barometer reading, not sea-level PS.

### Particulate (PM2.5 / PM10) and air-quality devices

PM is modelled with the same ambient-relaxation engine as temperature/humidity.

- **PM10 source:** opt-in `weather.pm_mode: kma` fetches KMA 황사 PM10
  (`kma_pm10.php`). **PM2.5 is not provided by KMA 황사** and stays simulated.
- **PM10 evidence (outdoor reference):** the KMA PM10 value when `pm_mode: kma`,
  otherwise the simulated baseline.
- **Air-quality controllers** drive the indoor PM target (physical opposites):
  - **Air purifier ON** → PM relaxes toward a clean baseline (fast).
  - **Ventilation (환풍기) ON** → PM relaxes toward the **outdoor** value (fast) —
    during 황사 this raises indoor PM, by design.
  - **All off (indoor)** → PM slowly relaxes toward `outdoor − offset` (indoor is
    a bit cleaner than outside).
  - **Outdoor site** → PM tracks the outdoor/KMA value directly.
- **Wind:** outdoor wind speed (WS) speeds the humidity (evaporation) and PM
  (ventilation) convergence *rate* toward ambient. It does not move the
  equilibrium (the KMA reading already embeds wind), so the effect is a subtle
  transient, not a large swing.

> **PM10 verification status (pending).** Unlike the ASOS endpoint, the
> `kma_pm10.php` column layout has **not** been verified against a live response,
> so `pm_mode` defaults to **off**. `weather.pm_column` (default 2) is the 0-based
> PM10 column index — call `kma_pm10.php?tm1=YYYYMMDDHH00&tm2=YYYYMMDDHH00&stn=108&help=1`,
> confirm the column, adjust `pm_column` if needed, then set `pm_mode: kma`.

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

## StockOps Admin-Web MQTT Subscription

`stockops-admin-web` subscribes to live sensor telemetry directly through MQTT over WebSocket.

Required broker listener:

```text
listener 9001 0.0.0.0
protocol websockets
listener_allow_anonymous false
password_file /mosquitto/config/passwords
```

Admin-web topic filter:

```text
sensimul/sites/+/sensors/+
```

The API server should not persist live sensor measurements when this browser subscription path is enabled.
