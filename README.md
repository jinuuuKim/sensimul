# SenSimul - 환경 센서 시뮬레이터

> 현장 기반 환경 데이터 시뮬레이션 시스템

## 개요

SenSimul은 물류창고, 냉동창고, 야외적치장 등 다양한 현장의 환경 데이터를 시뮬레이션하는 Go 기반 CLI 도구입니다.

## 특징

- **현장 유형 지원**: 실내(Indoor) / 실외(Outdoor)
- **물리 기반 모델**: 뉴턴 냉각 법칙, 증발/응결 모델 적용
- **환경 조절 시뮬레이션**: 냉방/난방/가습/제습 조절기 ON/OFF
- **날씨 API 연동**: 실외 현장은 실시간 기상 데이터 반영
- **장애 시뮬레이션**: 센서 오류, 조절기 고장, 정전 등
- **MQTT 통신**: 계층적 토픽 구조로 데이터 발행
- **웹 클라이언트(분리 서비스)**: 사이트/센서/조절기 관리 및 실시간 데이터 확인

## 설치

```bash
git clone https://gitlab.ithans.com/solcloud_team/sensimul.git
cd sensimul
make build
```

## 사용법

### 현장 생성
```bash
./build/sensimul site add --id SEOUL_COLD --name "서울냉동창고" --type indoor --lat 37.5665 --lon 126.9780
```

### 센서 추가
```bash
./build/sensimul sensor add --site SEOUL_COLD --type temperature --id TEMP_001
```

### 시뮬레이션 실행
```bash
./build/sensimul run --config config/sensimul.yaml
```

### 웹 클라이언트 실행 (선택)
```bash
docker compose --profile web up -d --build
```

## 라이선스

Internal Use Only - Team Project
