# SenSimul - 환경 센서 시뮬레이터

> 현장 기반 환경 데이터 시뮬레이션 시스템

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 📋 목차

- [개요](#개요)
- [특징](#특징)
- [아키텍처](#아키텍처)
- [설치 및 빌드](#설치-및-빌드)
- [설정](#설정)
- [사용법](#사용법)
- [배포](#배포)
- [MQTT 토픽 구조](#mqtt-토픽-구조)
- [환경변수](#환경변수)
- [라이선스](#라이선스)

## 개요

SenSimul은 물류창고, 냉동창고, 야외적치장 등 다양한 현장의 환경 데이터를 시뮬레이션하는 Go 기반 CLI 도구입니다. 실제 센서 없이도 MQTT를 통해 실시간 환경 데이터를 발행하여 IoT 시스템 테스트 및 개발 환경을 제공합니다.

### 주요 용도

- 🏭 **물류창고 환경 모니터링** 시뮬레이션
- 🧊 **냉동/냉장 시설** 온습도 데이터 생성
- 🌤️ **야외 적치장** 기상 연동 시뮬레이션
- 🔧 **IoT 시스템 테스트** 환경 구축
- 📊 **센서 데이터 파이프라인** 개발/검증

## 특징

### 핵심 기능

| 기능 | 설명 |
|------|------|
| **현장 유형 지원** | 실내(Indoor) / 실외(Outdoor) 환경 분리 시뮬레이션 |
| **물리 기반 모델** | 뉴턴 냉각 법칙, 증발/응결 모델 적용 |
| **환경 조절 시뮬레이션** | 냉방/난방/가습/제습 조절기 ON/OFF 상태 반영 |
| **날씨 API 연동** | 실외 현장은 OpenWeatherMap 실시간 기상 데이터 반영 |
| **장애 시뮬레이션** | 센서 오류, 조절기 고장, 정전 등 다양한 장애 상황 재현 |
| **MQTT 통신** | 계층적 토픽 구조로 데이터 발행 (QoS 0/1/2 지원) |
| **웹 클라이언트** | 사이트/센서/조절기 관리 및 실시간 데이터 모니터링 |

### 기술 스택

- **언어**: Go 1.23+
- **CLI**: Cobra
- **설정**: Viper (YAML + 환경변수)
- **MQTT**: Eclipse Paho
- **DB**: SQLite (메타데이터 저장)
- **로깅**: Zerolog

## 아키텍처

```
┌─────────────────────────────────────────────────────────────┐
│                        SenSimul                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Site A    │  │   Site B    │  │      Site C         │  │
│  │  (Indoor)   │  │  (Indoor)   │  │     (Outdoor)       │  │
│  │             │  │             │  │                     │  │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────────────┐ │  │
│  │ │ Sensors │ │  │ │ Sensors │ │  │ │ Sensors +       │ │  │
│  │ │- Temp   │ │  │ │- Temp   │ │  │ │  Weather API    │ │  │
│  │ │- Humi   │ │  │ │- Humi   │ │  │ │- Temp/Humi/PM   │ │  │
│  │ │- PM2.5  │ │  │ │- PM2.5  │ │  │ │                 │ │  │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────────────┘ │  │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────────────┐ │  │
│  │ │Controllers│  │ │Controllers│  │ │  Controllers    │ │  │
│  │ │- Cooler │ │  │ │- Heater │ │  │ │   (Limited)     │ │  │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────────────┘ │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              MQTT Publisher                         │    │
│  │     sensimul/sites/{site_id}/sensors/{sensor_id}    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  MQTT Broker    │
                    │ (Mosquitto)     │
                    └─────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌────────────┐  ┌────────────┐  ┌────────────┐
       │  Web UI    │  │  External  │  │   Logger   │
       │ (Optional) │  │  Systems   │  │            │
       └────────────┘  └────────────┘  └────────────┘
```

## 설치 및 빌드

### 요구사항

- Go 1.23 or higher
- Docker & Docker Compose (선택사항)
- MQTT Broker (Docker로 자동 제공)

### 소스에서 빌드

```bash
# 저장소 클론
git clone https://gitlab.ithans.com/solcloud_team/sensimul.git
cd sensimul

# 빌드
make build

# 바이너리 확인
./build/sensimul --help
```

### Makefile 명령어

```bash
make build          # 메인 바이너리 빌드
make build-web      # 웹 서버 바이너리 빌드
make test           # 테스트 실행
make test-race      # 레이스 디텍터 테스트
make clean          # 빌드 아티팩트 정리
make docker-build   # Docker 이미지 빌드
make docker-up      # Docker Compose 전체 실행
make lint           # 린터 실행
make fmt            # 코드 포맷팅
```

## 설정

### 설정 파일

기본 설정 파일: `config/sensimul.yaml`

```yaml
mode: dev                    # 실행 모드: dev, test, prodlike
seed: 42                     # 랜덤 시드 (재현 가능한 결과)
tick_interval: 5s            # 시뮬레이션 틱/센서 송신 기본 간격 (웹 Live 화면에서도 변경할 수 있습니다)
site_id: ""                  # 특정 사이트만 실행 (빈값=전체)

sqlite:
  path: data/sensimul.db     # SQLite 데이터베이스 경로

mqtt:
  broker_url: tcp://mqtt:1883    # MQTT 브로커 URL
  client_id: sensimul-local      # MQTT 클라이언트 ID
  qos: 1                         # QoS 레벨 (0/1/2)
  retain: false                  # Retain 메시지 여부

weather:
  mode: synthetic            # 날씨 모드: synthetic, openweathermap
  api_key: ""                # OpenWeatherMap API 키 (선택)
  ttl: 300s                  # 날씨 캐시 TTL

logging:
  level: info                # 로그 레벨: debug, info, warn, error
  format: json               # 로그 포맷: json, console

web:
  listen_addr: ":8080"       # 웹 서버 주소
  stale_after: 10s           # 센서 데이터 stale 판정 시간
  sse_buffer: 256            # SSE 버퍼 크기
```

### 환경변수

Viper를 통해 환경변수로 모든 설정을 오버라이드할 수 있습니다.

```bash
# .env 파일 생성
cp .env.example .env

# 환경변수 예시
export SENSIMUL_WEATHER_API_KEY="your_api_key"
export SENSIMUL_MQTT_BROKER_URL="tcp://localhost:1883"
export SENSIMUL_MODE="dev"
```

자세한 환경변수 목록은 `.env.example` 파일을 참조하세요.

## 사용법

### 기본 워크플로우

```bash
# 1. 현장(Site) 생성
./build/sensimul site add \
  --id SEOUL_COLD \
  --name "서울냉동창고" \
  --type indoor \
  --lat 37.5665 \
  --lon 126.9780

# 2. 센서(Sensor) 추가
./build/sensimul sensor add \
  --site SEOUL_COLD \
  --type temperature \
  --id TEMP_001 \
  --name "온도센서1"

./build/sensimul sensor add \
  --site SEOUL_COLD \
  --type humidity \
  --id HUMI_001 \
  --name "습도센서1"

# 3. 조절기(Controller) 추가 (선택)
./build/sensimul controller add \
  --site SEOUL_COLD \
  --type cooler \
  --id COOL_001 \
  --name "냉방기1"

# 4. 시뮬레이션 실행
./build/sensimul run --config config/sensimul.yaml
```

### CLI 명령어 상세

#### 사이트 관리

```bash
# 사이트 생성
./build/sensimul site add \
  --id <site_id> \
  --name <name> \
  --type <indoor|outdoor> \
  --lat <latitude> \
  --lon <longitude>

# 사이트 목록 조회
./build/sensimul site list

# 사이트 삭제
./build/sensimul site delete <site_id>
```

#### 센서 관리

```bash
# 센서 생성
./build/sensimul sensor add \
  --site <site_id> \
  --type <sensor_type> \
  --id <sensor_id> \
  --name <name>

# 센서 목록 조회
./build/sensimul sensor list --site <site_id>

# 센서 삭제
./build/sensimul sensor delete <sensor_id>
```

지원 센서 타입:
- `temperature` - 온도 (°C)
- `humidity` - 습도 (%)
- `pm25` - 미세먼지 PM2.5 (μg/m³)
- `pm10` - 초미세먼지 PM10 (μg/m³)
- `pressure` - 기압 (hPa)

#### 조절기 관리

```bash
# 조절기 생성
./build/sensimul controller add \
  --site <site_id> \
  --type <controller_type> \
  --id <controller_id> \
  --name <name>

# 조절기 목록 조회
./build/sensimul controller list --site <site_id>

# 조절기 삭제
./build/sensimul controller delete <controller_id>
```

지원 조절기 타입:
- `cooler` - 냉방기 (온도 감소)
- `heater` - 난방기 (온도 상승)
- `humidifier` - 가습기 (습도 증가)
- `dehumidifier` - 제습기 (습도 감소)

#### 시뮬레이션 실행

```bash
# 기본 실행
./build/sensimul run --config config/sensimul.yaml

# 특정 사이트만 실행
./build/sensimul run --config config/sensimul.yaml --site SEOUL_COLD

# 로그 레벨 조정
SENSIMUL_LOGGING_LEVEL=debug ./build/sensimul run
```

#### 상태 확인

```bash
# 종속성 상태 확인 (MQTT, DB 등)
./build/sensimul health --config config/sensimul.yaml
```

## 배포

### Docker Compose 배포 (권장)

```bash
# 전체 스택 실행 (MQTT + Simulator + Web)
docker compose --profile web up -d --build

# MQTT + Simulator만 실행 (Web 없음)
docker compose up -d

# MQTT 브로커만 실행
make docker-up-mqtt

# 로그 확인
docker compose logs -f

# 중지
docker compose down
```

### 수동 배포

```bash
# 1. MQTT 브로커 설치 (선택)
docker run -d -p 1883:1883 eclipse-mosquitto:2

# 2. 설정 파일 수정
vim config/sensimul.yaml
# mqtt.broker_url: tcp://localhost:1883

# 3. 실행
./build/sensimul run --config config/sensimul.yaml
```

### 웹 클라이언트 배포

```bash
# Web 프로필로 실행
docker compose --profile web up -d --build

# 웹 UI 접속
open http://localhost:18080
```

### 프로덕션 고려사항

```bash
# 1. 실제 날씨 데이터 사용 (선택)
# OpenWeatherMap API 키 발급 후 설정
export SENSIMUL_WEATHER_API_KEY="your_api_key"

# config/sensimul.yaml 수정
# weather:
#   mode: openweathermap
#   api_key: "${SENSIMUL_WEATHER_API_KEY}"

# 2. 프로덕션 모드 설정
export SENSIMUL_MODE=prodlike
export SENSIMUL_LOGGING_LEVEL=warn

# 3. 실행
./build/sensimul run --config config/sensimul.yaml
```

## MQTT 토픽 구조

### 토픽 패턴

```
sensimul/sites/{site_id}/sensors/{sensor_id}
```

### 메시지 형식

```json
{
  "site_id": "SEOUL_COLD",
  "sensor_id": "TEMP_001",
  "sensor_type": "temperature",
  "value": 18.5,
  "unit": "°C",
  "timestamp": "2026-04-13T10:30:00Z",
  "source": "simulated"
}
```

### 테스트 토픽

```
sensimul/sites/{site_id}/test/request   # 테스트 요청
sensimul/sites/{site_id}/test/result    # 테스트 결과
```

## 환경변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `SENSIMUL_WEATHER_API_KEY` | OpenWeatherMap API 키 | "" |
| `SENSIMUL_MQTT_BROKER_URL` | MQTT 브로커 URL | tcp://localhost:1883 |
| `SENSIMUL_MQTT_CLIENT_ID` | MQTT 클라이언트 ID | 자동생성 |
| `SENSIMUL_MQTT_QOS` | MQTT QoS 레벨 | 1 |
| `SENSIMUL_MODE` | 실행 모드 | dev |
| `SENSIMUL_SQLITE_PATH` | SQLite DB 경로 | data/sensimul.db |
| `SENSIMUL_TICK_INTERVAL` | 시뮬레이션 간격 | 5s |
| `SENSIMUL_WEB_LISTEN_ADDR` | 웹 서버 주소 | :8080 |
| `SENSIMUL_LOGGING_LEVEL` | 로그 레벨 | info |
| `SENSIMUL_LOGGING_FORMAT` | 로그 포맷 | json |

## 라이선스

MIT License - 자세한 내용은 [LICENSE](LICENSE) 파일을 참조하세요.

---

## 🤝 기여

버그 리포트, 기능 요청, PR은 언제나 환영합니다!
