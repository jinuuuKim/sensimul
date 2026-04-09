# SenSimul - 환경 센서 시뮬레이터

> 현장 기반 환경 데이터 시뮬레이션 시스템

## 개요

SenSimul은 물류창고, 냉동창고, 야외적치장 등 다양한 현장의 환경 데이터를 시뮬레이션하는 Python 기반 CLI 도구입니다.

## 특징

- **현장 유형 지원**: 실내(Indoor) / 실외(Outdoor)
- **물리 기반 모델**: 뉴턴 냉각 법칙, 증발/응결 모델 적용
- **환경 조절 시뮬레이션**: 냉방/난방/가습/제습 조절기 ON/OFF
- **날씨 API 연동**: 실외 현장은 실시간 기상 데이터 반영
- **장애 시뮬레이션**: 센서 오류, 조절기 고장, 정전 등
- **MQTT 통신**: 계층적 토픽 구조로 데이터 발행

## 설치

```bash
git clone https://gitlab.ithans.com/solcloud_team/sensimul.git
cd sensimul
pip install -r requirements.txt
```

## 사용법

### 현장 생성
```bash
sensimul site add "서울냉동창고" --type indoor --location "37.5665,126.9780"
```

### 센서 추가
```bash
sensimul sensor add "서울냉동창고" --type temperature --id TEMP_001
```

### 시뮬레이션 실행
```bash
sensimul run --site "서울냉동창고" --interval 10s
```

## 라이선스

Internal Use Only - Team Project
