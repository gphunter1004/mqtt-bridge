# 데이터베이스 테이블 역할 및 데이터 흐름 분석

## 📊 테이블 분류

### 🔄 실시간 로봇 통신 테이블
자동으로 MQTT 메시지를 통해 업데이트되는 테이블들

### 📝 템플릿 관리 테이블
사용자가 수동으로 생성/관리하는 템플릿 테이블들

### 📋 주문 실행 테이블
주문 실행 과정에서 생성/업데이트되는 테이블들

---

## 🔄 실시간 로봇 통신 테이블

### 1. `connection_states` 📡
**역할:** 로봇의 현재 연결 상태 저장 (최신 상태만 유지)

**데이터 구조:**
```sql
id, serial_number, connection_state, header_id, timestamp, version, manufacturer, created_at, updated_at
```

**INSERT 시점:**
- 새로운 로봇이 처음 연결될 때

**UPDATE 시점:**
- 기존 로봇의 연결 상태가 변경될 때 (ONLINE ↔ OFFLINE)
- MQTT Topic: `meili/v2/Roboligent/{serial_number}/connection`

**트리거:**
```go
// MQTT 메시지 수신 시 자동 처리
mqtt.handleConnectionMessage() → database.SaveConnectionState()
```

**예시 시나리오:**
```
1. 로봇 DEX0001 최초 연결 → INSERT (ONLINE)
2. 로봇 DEX0001 연결 해제 → UPDATE (OFFLINE)  
3. 로봇 DEX0001 재연결 → UPDATE (ONLINE)
```

---

### 2. `connection_state_histories` 📈
**역할:** 로봇 연결 상태의 모든 이력 보관 (감사/분석용)

**데이터 구조:**
```sql
id, serial_number, connection_state, header_id, timestamp, version, manufacturer, created_at
```

**INSERT 시점:**
- 로봇의 연결 상태가 변경될 때마다 무조건 INSERT (삭제 없음)

**UPDATE 시점:**
- 없음 (이력 테이블이므로 UPDATE 하지 않음)

**트리거:**
```go
// connection_states와 동시에 실행
mqtt.handleConnectionMessage() → database.SaveConnectionState() → INSERT both tables
```

---

### 3. `agv_actions` 🛠️
**역할:** 로봇이 지원하는 액션 능력 정보 저장

**데이터 구조:**
```sql
id, serial_number, action_type, action_description, action_scopes, result_description, created_at, updated_at
```

**INSERT 시점:**
- 새로운 로봇의 factsheet를 받았을 때 새로운 액션 발견 시

**UPDATE 시점:**
- 기존 로봇의 factsheet에서 액션 정보가 변경되었을 때
- MQTT Topic: `meili/v2/{manufacturer}/{serial_number}/factsheet`

**트리거:**
```go
// Factsheet 메시지 수신 시 자동 처리
mqtt.handleFactsheetMessage() → database.SaveOrUpdateFactsheet()
```

**데이터 흐름:**
```
로봇 ONLINE → Bridge가 factsheet 요청 → 로봇이 factsheet 응답 → DB 저장/업데이트
```

---

### 4. `agv_action_parameters` ⚙️
**역할:** 로봇 액션의 파라미터 상세 정보

**데이터 구조:**
```sql
id, agv_action_id, key, description, is_optional, value_data_type
```

**INSERT/UPDATE 시점:**
- agv_actions와 연동하여 factsheet 처리 시 함께 처리
- 스마트 업데이트: 기존 파라미터는 업데이트, 새 파라미터는 추가, 없어진 파라미터는 삭제

---

### 5. `physical_parameters` 📏
**역할:** 로봇의 물리적 특성 (속도, 크기 등)

**데이터 구조:**
```sql
id, serial_number, acceleration_max, deceleration_max, height_max, height_min, length, speed_max, speed_min, width, created_at, updated_at
```

**INSERT/UPDATE 시점:**
- Factsheet 메시지 수신 시
- 로봇당 하나의 레코드만 유지 (UPSERT 방식)

---

### 6. `type_specifications` 🏷️
**역할:** 로봇의 타입 및 사양 정보

**데이터 구조:**
```sql
id, serial_number, agv_class, agv_kinematics, localization_types, max_load_mass, navigation_types, series_description, series_name, created_at, updated_at
```

**INSERT/UPDATE 시점:**
- Factsheet 메시지 수신 시
- 로봇당 하나의 레코드만 유지 (UPSERT 방식)

---

## 📝 템플릿 관리 테이블

### 7. `order_templates` 📋
**역할:** 주문 작업의 템플릿 정의

**데이터 구조:**
```sql
id, name, description, created_at, updated_at
```

**INSERT 시점:**
- 사용자가 새로운 주문 템플릿 생성 시
- API: `POST /api/v1/order-templates`

**UPDATE 시점:**
- 사용자가 기존 템플릿 수정 시
- API: `PUT /api/v1/order-templates/{id}`

**사용자 액션:**
```
작업자가 "픽업→이동→배치" 작업 패턴을 템플릿으로 생성
```

---

### 8. `node_templates` 📍
**역할:** 작업 위치점 템플릿 정의

**데이터 구조:**
```sql
id, node_id, name, description, sequence_id, released, x, y, theta, allowed_deviation_xy, allowed_deviation_theta, map_id, action_template_ids, created_at, updated_at
```

**INSERT 시점:**
- 사용자가 새로운 노드 생성 시
- API: `POST /api/v1/nodes`

**UPDATE 시점:**
- 사용자가 노드 정보 수정 시 (위치, 액션 등)
- API: `PUT /api/v1/nodes/{nodeId}`

**사용자 액션:**
```
작업자가 "창고 A-1 선반" 위치를 노드로 등록
좌표: (10.5, 15.2), 픽업 액션 포함
```

---

### 9. `edge_templates` 🔗
**역할:** 노드 간 이동 경로 템플릿 정의

**데이터 구조:**
```sql
id, edge_id, name, description, sequence_id, released, start_node_id, end_node_id, action_template_ids, created_at, updated_at
```

**INSERT 시점:**
- 사용자가 새로운 경로 생성 시
- API: `POST /api/v1/edges`

**UPDATE 시점:**
- 사용자가 경로 정보 수정 시
- API: `PUT /api/v1/edges/{edgeId}`

**사용자 액션:**
```
작업자가 "A-1 선반 → B-2 선반" 경로를 등록
네비게이션 액션, 최대속도 1.5m/s 설정
```

---

### 10. `action_templates` ⚙️
**역할:** 독립적인 액션 템플릿 (재사용 가능)

**데이터 구조:**
```sql
id, action_type, action_id, blocking_type, action_description, created_at, updated_at
```

**INSERT 시점:**
- 사용자가 독립적인 액션 템플릿 생성 시
- Node/Edge 생성 시 포함된 액션들도 자동 생성
- API: `POST /api/v1/actions`

**UPDATE 시점:**
- 사용자가 액션 템플릿 수정 시
- API: `PUT /api/v1/actions/{actionId}`

**사용자 액션:**
```
작업자가 "표준 픽업" 액션 템플릿 생성
그리퍼 힘: 50N, 픽업 높이: 1.2m 등 설정
```

---

### 11. `action_parameter_templates` 🔧
**역할:** 액션 템플릿의 파라미터 정의

**데이터 구조:**
```sql
id, action_template_id, key, value, value_type
```

**INSERT/UPDATE 시점:**
- action_templates와 함께 생성/수정됨
- 파라미터 변경 시 기존 파라미터 삭제 후 새로 생성

---

## 🔗 연결 테이블 (Many-to-Many)

### 12. `order_template_nodes` 🔗
**역할:** 주문 템플릿과 노드 템플릿 연결

**INSERT 시점:**
- 주문 템플릿 생성 시 노드 연결
- 기존 템플릿에 노드 추가 시

**UPDATE 시점:**
- 없음 (DELETE 후 INSERT 방식)

---

### 13. `order_template_edges` 🔗
**역할:** 주문 템플릿과 엣지 템플릿 연결

**INSERT 시점:**
- 주문 템플릿 생성 시 엣지 연결
- 기존 템플릿에 엣지 추가 시

**UPDATE 시점:**
- 없음 (DELETE 후 INSERT 방식)

---

## 📋 주문 실행 테이블

### 14. `order_executions` 🚀
**역할:** 실제 로봇에게 전송된 주문의 실행 상태 추적

**데이터 구조:**
```sql
id, order_id, order_template_id, serial_number, order_update_id, status, created_at, updated_at, started_at, completed_at, error_message
```

**INSERT 시점:**
- 주문 실행 시작 시
- API: `POST /api/v1/orders/execute`

**UPDATE 시점:**
- 주문 상태 변경 시 (CREATED → SENT → EXECUTING → COMPLETED/FAILED)
- MQTT 메시지로 로봇 상태 업데이트 수신 시

**상태 흐름:**
```
CREATED → SENT → ACKNOWLEDGED → EXECUTING → COMPLETED/FAILED/CANCELLED
```

**트리거:**
```go
// 주문 실행
API Request → OrderService.ExecuteOrder() → INSERT (CREATED)
MQTT Send → UPDATE (SENT)
Robot Response → UPDATE (ACKNOWLEDGED)
Robot Status → UPDATE (EXECUTING)
Task Complete → UPDATE (COMPLETED)
```

---

## 🕐 데이터 생명주기 요약

### 시스템 초기화 단계
1. **로봇 연결** → `connection_states`, `connection_state_histories` INSERT
2. **Factsheet 수신** → `agv_actions`, `physical_parameters`, `type_specifications` INSERT/UPDATE

### 작업 설정 단계 (사용자 작업)
1. **액션 템플릿 생성** → `action_templates`, `action_parameter_templates` INSERT
2. **노드 생성** → `node_templates` INSERT (액션 템플릿 ID 참조)
3. **엣지 생성** → `edge_templates` INSERT (액션 템플릿 ID 참조)
4. **주문 템플릿 생성** → `order_templates`, `order_template_nodes`, `order_template_edges` INSERT

### 작업 실행 단계
1. **주문 실행** → `order_executions` INSERT (CREATED)
2. **MQTT 전송** → `order_executions` UPDATE (SENT)
3. **로봇 응답** → `order_executions` UPDATE (상태 변경)

### 지속적 모니터링
- **로봇 상태 변경** → `connection_states` UPDATE, `connection_state_histories` INSERT
- **새로운 Factsheet** → 로봇 능력 테이블들 UPDATE

---

## 🔍 데이터 접근 패턴

### 읽기 주요 패턴
- **로봇 상태 조회**: `connection_states`, Redis 캐시
- **로봇 능력 조회**: `agv_actions`, `physical_parameters`, `type_specifications`
- **템플릿 목록**: `order_templates`, `node_templates`, `edge_templates`, `action_templates`
- **실행 히스토리**: `order_executions`, `connection_state_histories`

### 쓰기 주요 패턴
- **MQTT 자동 업데이트**: 연결 상태, 로봇 능력 정보
- **사용자 수동 생성**: 모든 템플릿 테이블
- **시스템 자동 추적**: 주문 실행 상태

이러한 구조를 통해 실시간 로봇 모니터링, 작업 템플릿 관리, 주문 실행 추적이 체계적으로 이루어집니다.