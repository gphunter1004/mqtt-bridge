# MQTT Bridge Topic Information

MQTT 브릿지 시스템에서 사용되는 모든 토픽과 메시지 정보를 정리한 문서입니다.

## 📋 목차

1. [PLC ↔ Bridge 통신](#plc--bridge-통신)
2. [Bridge ↔ Robot 통신](#bridge--robot-통신)
3. [데이터베이스 테이블 정보](#데이터베이스-테이블-정보)

---

## PLC ↔ Bridge 통신

### 1. PLC → Bridge (명령 요청)

**Topic:** `bridge/command`

**Message Types:**
- `CR` - 백내장 적출
- `GR` - 적내장 적출 
- `GC` - 그리퍼 세정
- `CC` - 카메라 확인
- `CL` - 카메라 세정
- `KC` - 나이프 세정
- `OC` - 명령 취소 (Order Cancel)

**Message Format:**
```
단순 텍스트 (예: "CR", "GR", "OC")
```

**관련 DB Table:** `commands`

---

### 2. Bridge → PLC (응답)

**Topic:** `bridge/response`

**Response Codes:**
- `{CommandType}:S` - 성공 (Success)
- `{CommandType}:F` - 실패 (Failure)
- `{CommandType}:A` - 비정상 (Abnormal)
- `{CommandType}:N` - 정상 (Normal)
- `{CommandType}:R` - 거부 (Rejected)

**Message Format:**
```
예시:
- "CR:S" (백내장 적출 성공)
- "OC:S" (명령 취소 성공)
- "CC:A" (카메라 확인 비정상)
```

**관련 DB Table:** `commands`

---

## Bridge ↔ Robot 통신

### 1. Robot → Bridge (연결 상태)

**Topic:** `meili/v2/{manufacturer}/{serial_number}/connection`

**Message Format:**
```json
{
  "headerId": 123,
  "timestamp": "2025-06-16T15:19:27Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0002",
  "connectionState": "ONLINE"
}
```

**Connection States:**
- `ONLINE` - 온라인
- `OFFLINE` - 오프라인
- `CONNECTIONBROKEN` - 연결 끊김

**관련 DB Table:** `robot_statuses`

---

### 2. Robot → Bridge (운영 상태)

**Topic:** `meili/v2/{manufacturer}/{serial_number}/state`

**Message Format:**
```json
{
  "actionStates": [
    {
      "actionDescription": "This action will trigger the behavior tree for following a recorded trajectory.",
      "actionId": "11d1f7953ca14cd4b1e3266dbb882571",
      "actionStatus": "WAITING",
      "actionType": "Roboligent Robin - Follow Trajectory",
      "resultDescription": ""
    }
  ],
  "agvPosition": {
    "deviationRange": 0,
    "localizationScore": 1.0,
    "mapDescription": "",
    "mapId": "",
    "positionInitialized": true,
    "theta": 0.036346584612831405,
    "x": -0.698147540154784,
    "y": -0.45838658454549475
  },
  "batteryState": {
    "batteryCharge": 60.00000238418579,
    "batteryHealth": 0,
    "batteryVoltage": 40.0,
    "charging": false,
    "reach": 0
  },
  "distanceSinceLastNode": 0,
  "driving": true,
  "errors": [],
  "headerId": 46,
  "information": [],
  "lastNodeId": "intermediate_node_0_0",
  "lastNodeSequenceId": 0,
  "manufacturer": "Roboligent",
  "operatingMode": "AUTOMATIC",
  "orderId": "856009bd43f24945bd4f0b122a5dc8ed",
  "paused": false,
  "safetyState": {
    "eStop": "NONE",
    "fieldViolation": false
  },
  "serialNumber": "dex",
  "timestamp": "2025-06-16T15:19:27",
  "velocity": {
    "omega": 0.0,
    "vx": 0.0,
    "vy": 0.0
  },
  "version": "0.8.4"
}
```

**관련 DB Table:** `robot_states`

---

### 3. Bridge → Robot (InstantActions 요청)

**Topic:** `meili/v2/{manufacturer}/{serial_number}/instantActions`

#### 3.1 InitPosition 요청

**Message Format:**
```json
{
  "headerId": 286,
  "timestamp": "2025-06-16T19:52:47.663533397Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0002",
  "actions": [
    {
      "actionType": "initPosition",
      "actionId": "629410f320164a4da0e5d2c05ef10b14_1750103380361",
      "blockingType": "NONE",
      "actionParameters": [
        {
          "key": "pose",
          "value": {
            "lastNodeId": "",
            "mapId": "",
            "theta": 0.0,
            "x": 0.0,
            "y": 0.0
          }
        }
      ]
    }
  ]
}
```

**트리거 조건:** `agvPosition.positionInitialized: false`

**관련 DB Table:** 추적하지 않음 (로그만 기록)

#### 3.2 CancelOrder 요청

**Message Format:**
```json
{
  "headerId": 283,
  "timestamp": "2025-06-16T19:49:40.361223014Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0002",
  "actions": [
    {
      "actionType": "cancelOrder",
      "actionId": "629410f320164a4da0e5d2c05ef10b14_1750103380361",
      "blockingType": "HARD",
      "actionParameters": []
    }
  ]
}
```

**트리거 조건:** PLC에서 "OC" 명령 수신

**관련 DB Table:** `commands` (OC 명령으로 기록)

#### 3.3 Factsheet 요청

**Message Format:**
```json
{
  "headerId": 285,
  "timestamp": "2025-06-16T19:51:30.123456789Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0002",
  "actions": [
    {
      "actionType": "factsheetRequest",
      "actionId": "factsheet_req_1750103380361",
      "blockingType": "NONE",
      "actionParameters": []
    }
  ]
}
```

**트리거 조건:** 로봇이 온라인 상태가 될 때

**관련 DB Table:** `robot_factsheets`

---

### 4. Robot → Bridge (Factsheet 응답)

**Topic:** `meili/v2/{manufacturer}/{serial_number}/factsheet`

**Message Format:**
```json
{
  "headerId": 285,
  "timestamp": "2025-06-16T19:51:35.123456789Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0002",
  "agvGeometry": {},
  "physicalParameters": {
    "AccelerationMax": 1.0,
    "DecelerationMax": 1.0,
    "HeightMax": 2.0,
    "HeightMin": 1.5,
    "Length": 1.2,
    "SpeedMax": 2.0,
    "SpeedMin": 0.1,
    "Width": 0.8
  },
  "protocolFeatures": {
    "AgvActions": [],
    "OptionalParameters": []
  },
  "protocolLimits": {
    "VDA5050ProtocolLimits": []
  },
  "typeSpecification": {
    "AgvClass": "forklift",
    "AgvKinematics": "diff",
    "LocalizationTypes": ["natural"],
    "MaxLoadMass": 1000,
    "NavigationTypes": ["physical_line_guided"],
    "SeriesDescription": "Automated Guided Vehicle",
    "SeriesName": "Robin"
  }
}
```

**관련 DB Table:** `robot_factsheets`

---

## 데이터베이스 테이블 정보

### 1. commands
PLC에서 받은 명령 정보를 저장

**주요 필드:**
- `id` - 고유 ID
- `command_type` - 명령 타입 (CR, GR, GC, CC, CL, KC, OC)
- `status` - 상태 (PENDING, PROCESSING, SUCCESS, FAILURE, ABNORMAL, NORMAL, REJECTED)
- `request_time` - 요청 시간
- `response_time` - 응답 시간
- `error_message` - 에러 메시지

**관련 토픽:**
- `bridge/command` (입력)
- `bridge/response` (출력)

---

### 2. robot_statuses
로봇 연결 상태 정보를 저장

**주요 필드:**
- `id` - 고유 ID
- `manufacturer` - 제조사
- `serial_number` - 시리얼 번호
- `connection_state` - 연결 상태 (ONLINE, OFFLINE, CONNECTIONBROKEN)
- `last_header_id` - 마지막 헤더 ID
- `last_timestamp` - 마지막 타임스탬프
- `version` - 버전

**관련 토픽:**
- `meili/v2/{manufacturer}/{serial_number}/connection`

---

### 3. robot_states
로봇 실시간 운영 상태 정보를 저장

**주요 필드:**
- `id` - 고유 ID
- `serial_number` - 시리얼 번호
- `manufacturer` - 제조사
- `header_id` - 헤더 ID
- `timestamp` - 타임스탬프
- `position_x`, `position_y`, `position_theta` - 위치 정보
- `localization_score` - 로컬라이제이션 점수
- `position_initialized` - 위치 초기화 여부
- `battery_charge`, `battery_voltage` - 배터리 정보
- `operating_mode` - 운영 모드
- `driving`, `paused` - 주행 상태
- `e_stop`, `field_violation` - 안전 상태
- `error_count`, `action_count` - 에러 및 액션 수

**관련 토픽:**
- `meili/v2/{manufacturer}/{serial_number}/state`

---

### 4. robot_factsheets
로봇 팩트시트 정보를 저장

**주요 필드:**
- `id` - 고유 ID
- `serial_number` - 시리얼 번호
- `manufacturer` - 제조사
- `version` - 버전
- `series_name`, `series_description` - 시리즈 정보
- `agv_class` - AGV 클래스
- `max_load_mass` - 최대 적재 질량
- `speed_max`, `speed_min` - 속도 정보
- `acceleration_max`, `deceleration_max` - 가속도 정보
- `length`, `width`, `height_max`, `height_min` - 크기 정보
- `last_updated` - 마지막 업데이트

**관련 토픽:**
- `meili/v2/{manufacturer}/{serial_number}/instantActions` (factsheetRequest)
- `meili/v2/{manufacturer}/{serial_number}/factsheet` (응답)

---

## 자동 처리 로직

### 1. 위치 초기화 (InitPosition)
- **트리거:** `agvPosition.positionInitialized: false` 감지
- **조건:** 자동 모드 (`operatingMode: "AUTOMATIC"`)
- **동작:** `initPosition` instantAction 자동 전송
- **추적:** 로그만 기록, DB 추적 없음

### 2. 명령 취소 (CancelOrder)
- **트리거:** PLC에서 "OC" 명령 수신
- **동작:** `cancelOrder` instantAction 전송 → 즉시 "OC:S" 응답
- **모니터링:** state 메시지의 actionStates에서 완료 상황 추적

### 3. 팩트시트 요청 (Factsheet)
- **트리거:** 로봇이 온라인 상태가 될 때
- **동작:** `factsheetRequest` instantAction 전송
- **저장:** factsheet 응답을 `robot_factsheets` 테이블에 저장

### 4. 크리티컬 상황 자동 처리
- **E-Stop 활성화:** 진행 중인 명령들을 자동 실패 처리
- **크리티컬 에러 발생:** FATAL/ERROR 레벨 에러 시 명령 실패 처리
- **극심한 저배터리:** 5% 미만 시 명령 실패 처리

---

## 메시지 흐름도

```
PLC                Bridge              Robot
 |                   |                   |
 |-- "CR" ---------->|                   |
 |                   |-- factsheetReq ->|
 |                   |<-- factsheet ----|
 |                   |                   |
 |                   |-- processing -----|
 |<-- "CR:S" --------|                   |
 |                   |                   |
 |-- "OC" ---------->|                   |
 |                   |-- cancelOrder --->|
 |<-- "OC:S" --------|                   |
 |                   |<-- state ---------|
 |                   |                   |
```

---

## 환경 설정

### MQTT 브로커 설정
- **Host:** localhost (또는 MQTT_BROKER 환경변수)
- **Port:** 1883 (또는 MQTT_PORT 환경변수)
- **Client ID:** DEX0002_PLC_BRIDGE

### 로봇 설정
- **Serial Number:** DEX0002 (또는 ROBOT_SERIAL_NUMBER 환경변수)
- **Manufacturer:** Roboligent (또는 ROBOT_MANUFACTURER 환경변수)

### 데이터베이스 설정
- **PostgreSQL:** 메인 데이터 저장
- **Redis:** 실시간 상태 캐시 (선택적)

---

## 로그 레벨별 출력

### INFO 레벨
- PLC 명령 수신/처리 완료
- 로봇 연결 상태 변경
- InstantAction 요청/응답

### DEBUG 레벨
- 상세한 메시지 페이로드
- 내부 처리 과정

### ERROR 레벨
- 크리티컬 에러 발생
- 통신 실패
- 데이터베이스 오류

### WARN 레벨
- E-Stop 활성화
- 저배터리 경고
- 명령 거부