# MQTT Bridge Service

PLC와 Robin Robot 간의 통신을 중재하는 MQTT 브릿지 서비스입니다.

## 기능

- **연결 상태 관리**: 로봇의 연결 상태를 실시간으로 모니터링하고 데이터베이스에 저장
- **Factsheet 관리**: 로봇이 온라인 상태가 되면 자동으로 factsheet를 요청하여 로봇 정보를 업데이트
- **상태 모니터링**: 로봇의 실시간 상태를 Redis에 저장하고 관리
- **위치 초기화**: 로봇의 위치가 초기화되지 않은 경우 자동으로 initPosition 명령 전송
- **Order 관리**: 로봇에게 작업 명령을 전송
- **REST API**: 로봇 관리를 위한 HTTP API 제공

## 아키텍처

```
PLC <-> MQTT Bridge Server <-> Robin Robot
                |
        +--------------+
        |              |
   PostgreSQL       Redis
```

## 프로젝트 구조

```
mqtt-bridge/
├── config/              # 설정 관리
│   └── config.go
├── models/              # 데이터 모델
│   └── models.go
├── database/            # 데이터베이스 연결 및 관리
│   └── database.go
├── redis/               # Redis 클라이언트
│   └── redis.go
├── mqtt/                # MQTT 클라이언트
│   └── client.go
├── services/            # 비즈니스 로직
│   └── bridge_service.go
├── handlers/            # HTTP 핸들러
│   └── api_handlers.go
├── main.go              # 메인 애플리케이션
├── .env                 # 환경 변수
├── go.mod               # Go 모듈
├── Dockerfile           # Docker 빌드
├── docker-compose.yml   # Docker Compose 설정
├── mosquitto.conf       # MQTT 브로커 설정
└── README.md           # 프로젝트 문서
```

## 설치 및 실행

### 1. 환경 변수 설정

`.env` 파일을 생성하고 필요한 환경 변수를 설정합니다:

```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=mqtt_bridge

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# MQTT Configuration
MQTT_BROKER=tcp://localhost:1883
MQTT_PORT=1883
MQTT_CLIENT_ID=bridge-server
MQTT_USERNAME=
MQTT_PASSWORD=

# Application Configuration
LOG_LEVEL=info
TIMEOUT_SECONDS=30
```

### 2. Docker Compose로 실행

```bash
# 모든 서비스 시작
docker-compose up -d

# 로그 확인
docker-compose logs -f mqtt-bridge
```

### 3. 로컬 개발 환경

```bash
# 의존성 설치
go mod download

# 애플리케이션 실행
go run main.go
```

## API 엔드포인트

### 로봇 관리

- `GET /api/v1/health` - 서비스 상태 확인
- `GET /api/v1/robots` - 연결된 로봇 목록
- `GET /api/v1/robots/{serialNumber}/state` - 로봇 상태 조회
- `GET /api/v1/robots/{serialNumber}/health` - 로봇 헬스 상태
- `GET /api/v1/robots/{serialNumber}/capabilities` - 로봇 기능 조회
- `GET /api/v1/robots/{serialNumber}/history` - 연결 히스토리

### 로봇 제어

- `POST /api/v1/robots/{serialNumber}/order` - 작업 명령 전송
- `POST /api/v1/robots/{serialNumber}/action` - 커스텀 액션 전송

## MQTT 토픽 구조

### 구독 토픽 (Bridge Server가 구독)

- `meili/v2/Roboligent/{serial_number}/connection` - 연결 상태
- `meili/v2/Roboligent/{serial_number}/state` - 로봇 상태
- `meili/v2/{manufacturer}/{serial_number}/factsheet` - Factsheet 정보

### 발행 토픽 (Bridge Server가 발행)

- `meili/v2/Roboligent/{serial_number}/instantActions` - 즉시 액션
- `meili/v2/Roboligent/{serial_number}/order` - 작업 명령

## 데이터베이스 스키마

### ConnectionState
- 로봇의 연결 상태 히스토리 저장

### AgvAction / AgvActionParameter
- 로봇의 사용 가능한 액션과 파라미터 정보

### PhysicalParameter
- 로봇의 물리적 파라미터 (속도, 크기 등)

### TypeSpecification
- 로봇의 타입 및 사양 정보

## Redis 키 구조

- `robot:state:{serial_number}` - 로봇 상태 정보
- `robot:connection:{serial_number}` - 연결 상태 정보

## 로봇 통신 플로우

1. **로봇 연결**: 로봇이 `ONLINE` 상태로 연결
2. **Factsheet 요청**: Bridge 서버가 자동으로 factsheet 요청
3. **상태 모니터링**: 로봇이 지속적으로 상태 정보 전송
4. **위치 초기화**: 필요시 자동으로 initPosition 명령 전송
5. **명령 실행**: API를 통해 로봇에게 작업 명령 전송

## 개발 및 디버깅

### 로그 확인

```bash
# Docker 환경
docker-compose logs -f mqtt-bridge

# 로컬 환경
go run main.go
```

### MQTT 메시지 테스트

```bash
# mosquitto_pub를 사용한 테스트 메시지 발행
mosquitto_pub -h localhost -t "meili/v2/Roboligent/DEX0002/connection" -m '{"headerId":1,"timestamp":"2025-07-03T10:00:00.000Z","version":"2.0","manufacturer":"Roboligent","serialNumber":"DEX0002","connectionState":"ONLINE"}'
```

### 데이터베이스 접속

```bash
# PostgreSQL 접속
docker exec -it mqtt-bridge-postgres psql -U postgres -d mqtt_bridge

# Redis 접속
docker exec -it mqtt-bridge-redis redis-cli
```

## 보안 고려사항

1. **MQTT 인증**: 프로덕션 환경에서는 MQTT 브로커에 인증 설정
2. **데이터베이스 보안**: 강력한 패스워드 사용 및 접근 제한
3. **API 보안**: 필요시 JWT 토큰 인증 추가
4. **네트워크 보안**: 방화벽 및 VPN 설정

## 라이센스

이 프로젝트는 MIT 라이센스 하에 배포됩니다.