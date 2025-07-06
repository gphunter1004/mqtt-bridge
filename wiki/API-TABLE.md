# API 실행 시 연관 테이블 매핑

## 🏥 Health Check API

### `GET /api/v1/health`
**연관 테이블:** 없음
**동작:** 서비스 상태만 반환

---

## 🤖 Robot Management API

### `GET /api/v1/robots`
**연관 테이블:**
- **READ**: `connection_states` - ONLINE 상태인 로봇 조회
- **READ**: Redis 캐시 - 연결 상태 검증

**쿼리:**
```sql
SELECT DISTINCT serial_number 
FROM connection_states 
WHERE connection_state = 'ONLINE'
```

---

### `GET /api/v1/robots/{serialNumber}/state`
**연관 테이블:**
- **READ**: Redis 캐시 - 로봇 실시간 상태 조회

**동작:** Redis에서 `robot:state:{serial_number}` 키 조회

---

### `GET /api/v1/robots/{serialNumber}/health`
**연관 테이블:**
- **READ**: Redis 캐시 - 로봇 상태 정보
- **READ**: `connection_states` - 연결 상태 확인

**동작:** 
1. Redis에서 상태 정보 조회
2. 연결 상태와 배터리, 위치 등 종합 분석

---

### `GET /api/v1/robots/{serialNumber}/capabilities`
**연관 테이블:**
- **READ**: `physical_parameters` - 물리적 특성
- **READ**: `type_specifications` - 타입 사양
- **READ**: `agv_actions` + `agv_action_parameters` - 사용 가능한 액션

**쿼리:**
```sql
-- 물리적 파라미터
SELECT * FROM physical_parameters WHERE serial_number = ?

-- 타입 사양
SELECT * FROM type_specifications WHERE serial_number = ?

-- 액션 정보
SELECT a.*, p.* FROM agv_actions a
LEFT JOIN agv_action_parameters p ON a.id = p.agv_action_id
WHERE a.serial_number = ?
```

---

### `GET /api/v1/robots/{serialNumber}/history`
**연관 테이블:**
- **READ**: `connection_state_histories` - 연결 이력

**쿼리:**
```sql
SELECT * FROM connection_state_histories 
WHERE serial_number = ? 
ORDER BY created_at DESC 
LIMIT ?
```

---

## 🎯 Robot Control API

### `POST /api/v1/robots/{serialNumber}/order`
**연관 테이블:**
- **READ**: Redis - 로봇 온라인 상태 확인
- **INSERT**: `order_executions` - 주문 실행 기록 생성
- **UPDATE**: `order_executions` - 상태 업데이트

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **Payload**: OrderMessage (nodes, edges 포함)

**처리 순서:**
1. Redis에서 로봇 ONLINE 상태 확인
2. `order_executions` INSERT (status: 'CREATED')
3. MQTT 전송 → `meili/v2/Roboligent/{serialNumber}/order`
4. `order_executions` UPDATE (status: 'SENT')

---

### `POST /api/v1/robots/{serialNumber}/action`
**연관 테이블:**
- **READ**: Redis - 로봇 온라인 상태 확인

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/instantActions`
- **Payload**: InstantActionMessage (actions 배열)

**동작:** MQTT 메시지 전송만, DB 저장 없음

---

### `POST /api/v1/robots/{serialNumber}/inference`
**연관 테이블:**
- **READ**: Redis - 로봇 온라인 상태 확인
- **INSERT**: `order_executions` - 추론 주문 기록

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **Payload**: 자동 생성된 OrderMessage (inference 액션이 포함된 노드)

**동작:** 추론 액션이 포함된 주문을 자동 생성하여 전송

---

### `POST /api/v1/robots/{serialNumber}/trajectory`
**연관 테이블:**
- **READ**: Redis - 로봇 온라인 상태 확인
- **INSERT**: `order_executions` - 궤적 주문 기록

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **Payload**: 자동 생성된 OrderMessage (trajectory 액션이 포함된 노드)

**동작:** 궤적 액션이 포함된 주문을 자동 생성하여 전송

---

## 📋 Order Template Management API

### `POST /api/v1/order-templates`
**연관 테이블:**
- **INSERT**: `order_templates` - 새 템플릿 생성
- **READ**: `node_templates` - nodeIds 존재 확인
- **READ**: `edge_templates` - edgeIds 존재 확인
- **INSERT**: `order_template_nodes` - 노드 연결
- **INSERT**: `order_template_edges` - 엣지 연결

**트랜잭션 처리:**
```sql
BEGIN;
INSERT INTO order_templates (name, description) VALUES (?, ?);
-- nodeIds 검증
SELECT id FROM node_templates WHERE node_id IN (?);
-- edgeIds 검증  
SELECT id FROM edge_templates WHERE edge_id IN (?);
-- 연결 생성
INSERT INTO order_template_nodes (order_template_id, node_template_id) VALUES (?, ?);
INSERT INTO order_template_edges (order_template_id, edge_template_id) VALUES (?, ?);
COMMIT;
```

---

### `GET /api/v1/order-templates`
**연관 테이블:**
- **READ**: `order_templates` - 템플릿 목록

**쿼리:**
```sql
SELECT * FROM order_templates 
ORDER BY created_at DESC 
LIMIT ? OFFSET ?
```

---

### `GET /api/v1/order-templates/{id}`
**연관 테이블:**
- **READ**: `order_templates` - 특정 템플릿

---

### `GET /api/v1/order-templates/{id}/details`
**연관 테이블:**
- **READ**: `order_templates` - 기본 템플릿 정보
- **READ**: `order_template_nodes` + `node_templates` - 연결된 노드들
- **READ**: `order_template_edges` + `edge_templates` - 연결된 엣지들
- **READ**: `action_templates` + `action_parameter_templates` - 노드/엣지의 액션들

**복잡한 JOIN 쿼리:**
```sql
-- 노드와 액션 정보
SELECT ot.*, nt.*, at.*, apt.*
FROM order_templates ot
JOIN order_template_nodes otn ON ot.id = otn.order_template_id
JOIN node_templates nt ON otn.node_template_id = nt.id
LEFT JOIN action_templates at ON at.id = ANY(nt.action_template_ids)
LEFT JOIN action_parameter_templates apt ON at.id = apt.action_template_id
WHERE ot.id = ?
```

---

### `PUT /api/v1/order-templates/{id}`
**연관 테이블:**
- **UPDATE**: `order_templates` - 기본 정보 수정
- **DELETE**: `order_template_nodes` - 기존 노드 연결 삭제
- **DELETE**: `order_template_edges` - 기존 엣지 연결 삭제
- **INSERT**: `order_template_nodes` - 새 노드 연결
- **INSERT**: `order_template_edges` - 새 엣지 연결

---

### `DELETE /api/v1/order-templates/{id}`
**연관 테이블:**
- **DELETE**: `order_template_nodes` - 노드 연결 삭제
- **DELETE**: `order_template_edges` - 엣지 연결 삭제
- **DELETE**: `order_templates` - 템플릿 삭제

---

## 🔗 Template Association API

### `POST /api/v1/order-templates/{id}/associate-nodes`
**연관 테이블:**
- **READ**: `node_templates` - nodeIds 존재 확인
- **READ**: `order_template_nodes` - 기존 연결 중복 확인
- **INSERT**: `order_template_nodes` - 새 연결 생성

---

### `POST /api/v1/order-templates/{id}/associate-edges`
**연관 테이블:**
- **READ**: `edge_templates` - edgeIds 존재 확인
- **READ**: `order_template_edges` - 기존 연결 중복 확인
- **INSERT**: `order_template_edges` - 새 연결 생성

---

## ⚡ Order Execution API

### `POST /api/v1/orders/execute`
**연관 테이블:**
- **READ**: `order_templates` + 연관 테이블들 - 템플릿 상세 정보
- **READ**: `node_templates` + `action_templates` - 노드와 액션
- **READ**: `edge_templates` + `action_templates` - 엣지와 액션
- **READ**: Redis - 로봇 온라인 상태
- **INSERT**: `order_executions` - 실행 기록

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **Payload**: 템플릿에서 변환된 OrderMessage (nodes, edges 포함)

**복잡한 데이터 조회:**
```sql
-- 템플릿 상세 정보 조회 (여러 JOIN)
-- 노드별 액션 템플릿 조회
-- 엣지별 액션 템플릿 조회
-- MQTT 메시지 생성을 위한 데이터 변환
```

---

### `POST /api/v1/orders/execute/template/{id}/robot/{serialNumber}`
**연관 테이블:** `POST /api/v1/orders/execute`와 동일

**MQTT 전송:**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **Payload**: 템플릿에서 변환된 OrderMessage

---

### `GET /api/v1/orders`
**연관 테이블:**
- **READ**: `order_executions` - 주문 실행 목록

**쿼리:**
```sql
SELECT * FROM order_executions 
WHERE serial_number = ? (optional)
ORDER BY created_at DESC 
LIMIT ? OFFSET ?
```

---

### `GET /api/v1/orders/{orderId}`
**연관 테이블:**
- **READ**: `order_executions` - 특정 주문 실행 정보

---

### `POST /api/v1/orders/{orderId}/cancel`
**연관 테이블:**
- **READ**: `order_executions` - 현재 상태 확인
- **UPDATE**: `order_executions` - 상태를 'CANCELLED'로 변경

---

### `GET /api/v1/robots/{serialNumber}/orders`
**연관 테이블:**
- **READ**: `order_executions` - 특정 로봇의 주문들

---

## 📍 Node Management API

### `POST /api/v1/nodes`
**연관 테이블:**
- **READ**: `node_templates` - nodeId 중복 확인
- **INSERT**: `action_templates` - 포함된 액션들 생성
- **INSERT**: `action_parameter_templates` - 액션 파라미터들 생성
- **INSERT**: `node_templates` - 노드 생성 (action_template_ids JSON 포함)

**트랜잭션 처리:**
```sql
BEGIN;
-- nodeId 중복 확인
SELECT id FROM node_templates WHERE node_id = ?;
-- 액션 템플릿들 생성
INSERT INTO action_templates (...) VALUES (...);
INSERT INTO action_parameter_templates (...) VALUES (...);
-- 노드 생성 (액션 ID 배열과 함께)
INSERT INTO node_templates (node_id, ..., action_template_ids) VALUES (?, ..., '[1,2,3]');
COMMIT;
```

---

### `GET /api/v1/nodes`
**연관 테이블:**
- **READ**: `node_templates` - 노드 목록

---

### `GET /api/v1/nodes/{nodeId}` (Database ID)
**연관 테이블:**
- **READ**: `node_templates` - 특정 노드

---

### `GET /api/v1/nodes/by-node-id/{nodeId}` (Node ID)
**연관 테이블:**
- **READ**: `node_templates` - nodeId로 조회

---

### `PUT /api/v1/nodes/{nodeId}`
**연관 테이블:**
- **READ**: `node_templates` - 기존 노드 정보 및 nodeId 중복 확인
- **DELETE**: `action_templates` + `action_parameter_templates` - 기존 액션들 삭제
- **INSERT**: `action_templates` + `action_parameter_templates` - 새 액션들 생성
- **UPDATE**: `node_templates` - 노드 정보 업데이트

---

### `DELETE /api/v1/nodes/{nodeId}`
**연관 테이블:**
- **READ**: `node_templates` - 삭제할 노드 정보
- **DELETE**: `action_templates` + `action_parameter_templates` - 연관 액션들 삭제
- **DELETE**: `order_template_nodes` - 템플릿 연결 삭제
- **DELETE**: `node_templates` - 노드 삭제

---

## 🔗 Edge Management API

### `POST /api/v1/edges`
**연관 테이블:**
- **READ**: `edge_templates` - edgeId 중복 확인
- **INSERT**: `action_templates` + `action_parameter_templates` - 액션들 생성
- **INSERT**: `edge_templates` - 엣지 생성

---

### `GET /api/v1/edges`
**연관 테이블:**
- **READ**: `edge_templates` - 엣지 목록

---

### `GET /api/v1/edges/{edgeId}` (Database ID)
**연관 테이블:**
- **READ**: `edge_templates` - 특정 엣지

---

### `GET /api/v1/edges/by-edge-id/{edgeId}` (Edge ID)
**연관 테이블:**
- **READ**: `edge_templates` - edgeId로 조회

---

### `PUT /api/v1/edges/{edgeId}`
**연관 테이블:**
- **READ**: `edge_templates` - 기존 엣지 정보
- **DELETE**: `action_templates` + `action_parameter_templates` - 기존 액션들 삭제
- **INSERT**: `action_templates` + `action_parameter_templates` - 새 액션들 생성
- **UPDATE**: `edge_templates` - 엣지 정보 업데이트

---

### `DELETE /api/v1/edges/{edgeId}`
**연관 테이블:**
- **READ**: `edge_templates` - 삭제할 엣지 정보
- **DELETE**: `action_templates` + `action_parameter_templates` - 연관 액션들 삭제
- **DELETE**: `order_template_edges` - 템플릿 연결 삭제
- **DELETE**: `edge_templates` - 엣지 삭제

---

## ⚙️ Action Template Management API

### `POST /api/v1/actions`
**연관 테이블:**
- **INSERT**: `action_templates` - 새 액션 템플릿
- **INSERT**: `action_parameter_templates` - 파라미터들

---

### `GET /api/v1/actions`
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 액션 목록

**필터링 쿼리:**
```sql
SELECT at.*, apt.* FROM action_templates at
LEFT JOIN action_parameter_templates apt ON at.id = apt.action_template_id
WHERE at.action_type LIKE ? (optional)
AND at.blocking_type = ? (optional)
AND (at.action_type ILIKE ? OR at.action_description ILIKE ?) (search)
ORDER BY at.created_at DESC
LIMIT ? OFFSET ?
```

---

### `GET /api/v1/actions/{actionId}` (Database ID)
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 특정 액션

---

### `GET /api/v1/actions/by-action-id/{actionId}` (Action ID)
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - actionId로 조회

---

### `PUT /api/v1/actions/{actionId}`
**연관 테이블:**
- **READ**: `action_templates` - 기존 액션 정보
- **UPDATE**: `action_templates` - 기본 정보 수정
- **DELETE**: `action_parameter_templates` - 기존 파라미터 삭제
- **INSERT**: `action_parameter_templates` - 새 파라미터 생성

---

### `DELETE /api/v1/actions/{actionId}`
**연관 테이블:**
- **READ**: `action_templates` - 삭제할 액션 정보
- **DELETE**: `action_parameter_templates` - 연관 파라미터 삭제
- **DELETE**: `action_templates` - 액션 삭제

---

### `POST /api/v1/actions/{actionId}/clone`
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 원본 액션
- **INSERT**: `action_templates` - 복제된 액션
- **INSERT**: `action_parameter_templates` - 복제된 파라미터들

---

## 📚 Action Library Management API

### `POST /api/v1/actions/library`
**연관 테이블:**
- **INSERT**: `action_templates` + `action_parameter_templates` - 라이브러리 액션

**동작:** 일반 액션 생성과 동일

---

### `GET /api/v1/actions/library`
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 모든 액션 (라이브러리로 취급)

---

## 🔍 Validation & Bulk Operations API

### `POST /api/v1/actions/validate`
**연관 테이블:**
- **READ**: `agv_actions` + `agv_action_parameters` - 로봇 능력 확인 (serialNumber가 있는 경우)

**동작:** 액션 유효성 검사 (현재는 기본 응답만 반환)

---

### `POST /api/v1/actions/bulk/delete`
**연관 테이블:**
- **DELETE**: `action_parameter_templates` - 여러 액션의 파라미터들
- **DELETE**: `action_templates` - 여러 액션들

**루프 처리:** 각 actionId에 대해 개별 삭제 실행

---

### `POST /api/v1/actions/bulk/clone`
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 원본 액션들
- **INSERT**: `action_templates` + `action_parameter_templates` - 복제된 액션들

**루프 처리:** 각 actionId에 대해 개별 복제 실행

---

## 📤 Import/Export API

### `POST /api/v1/actions/export`
**연관 테이블:**
- **READ**: `action_templates` + `action_parameter_templates` - 내보낼 액션들

**동작:** 메모리에서 JSON 변환 후 파일 다운로드

---

### `POST /api/v1/actions/import`
**연관 테이블:**
- **INSERT**: `action_templates` + `action_parameter_templates` - 가져온 액션들

**루프 처리:** 각 액션에 대해 개별 생성 실행

---

## 📡 MQTT 토픽 및 메시지 구조 상세

### 🤖 로봇으로 전송하는 MQTT 토픽 (Bridge → Robot)

#### 1. **주문 전송** 
- **Topic**: `meili/v2/Roboligent/{serialNumber}/order`
- **사용 API**: 
  - `POST /robots/{serialNumber}/order`
  - `POST /orders/execute`
  - `POST /orders/execute/template/{id}/robot/{serialNumber}`
  - `POST /robots/{serialNumber}/inference`
  - `POST /robots/{serialNumber}/trajectory`

**메시지 구조 (OrderMessage):**
```json
{
  "headerId": 1,
  "timestamp": "2025-07-06T10:30:45.000000000Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0001",
  "orderId": "order_19a2b3c4d5e6f",
  "orderUpdateId": 0,
  "nodes": [
    {
      "nodeId": "node_001",
      "description": "Pick position",
      "sequenceId": 0,
      "released": true,
      "nodePosition": {
        "x": 10.5,
        "y": 15.2,
        "theta": 1.57,
        "allowedDeviationXY": 0.1,
        "allowedDeviationTheta": 0.05,
        "mapId": "warehouse_map_001"
      },
      "actions": [
        {
          "actionType": "pick",
          "actionId": "pick_001",
          "blockingType": "HARD",
          "actionParameters": [
            {
              "key": "gripperForce",
              "value": 50
            }
          ]
        }
      ]
    }
  ],
  "edges": [
    {
      "edgeId": "edge_001_to_002",
      "sequenceId": 1,
      "released": true,
      "startNodeId": "node_001",
      "endNodeId": "node_002",
      "actions": []
    }
  ]
}
```

#### 2. **즉시 액션 전송**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/instantActions`
- **사용 API**: `POST /robots/{serialNumber}/action`

**메시지 구조 (InstantActionMessage):**
```json
{
  "headerId": 1,
  "timestamp": "2025-07-06T10:30:45.000000000Z",
  "version": "2.0.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0001",
  "actions": [
    {
      "actionType": "initPosition",
      "actionId": "init_001",
      "blockingType": "NONE",
      "actionParameters": [
        {
          "key": "pose",
          "value": {
            "x": 0.0,
            "y": 0.0,
            "theta": 0.0,
            "mapId": "map_001"
          }
        }
      ]
    }
  ]
}
```

### 🔄 로봇에서 수신하는 MQTT 토픽 (Robot → Bridge)

#### 1. **연결 상태 메시지**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/connection`
- **자동 처리**: `connection_states`, `connection_state_histories` 업데이트

**메시지 구조:**
```json
{
  "headerId": 1,
  "timestamp": "2025-07-06T10:30:45.000Z",
  "version": "2.0",
  "manufacturer": "Roboligent",
  "serialNumber": "DEX0001",
  "connectionState": "ONLINE"
}
```

#### 2. **로봇 상태 메시지**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/state`
- **자동 처리**: Redis 캐시 업데이트

**메시지 구조:**
```json
{
  "serialNumber": "DEX0001",
  "agvPosition": {
    "x": 10.5,
    "y": 15.2,
    "theta": 1.57,
    "positionInitialized": true,
    "mapId": "warehouse_map_001"
  },
  "batteryState": {
    "batteryCharge": 85.5,
    "batteryVoltage": 24.2,
    "charging": false
  },
  "driving": false,
  "paused": false,
  "operatingMode": "AUTOMATIC"
}
```

#### 3. **Factsheet 응답**
- **Topic**: `meili/v2/{manufacturer}/{serialNumber}/factsheet`
- **자동 처리**: `agv_actions`, `physical_parameters`, `type_specifications` 업데이트

**메시지 구조:**
```json
{
  "serialNumber": "DEX0001",
  "protocolFeatures": {
    "AgvActions": [
      {
        "ActionType": "pick",
        "ActionDescription": "Pick up items",
        "ActionParameters": [
          {
            "Key": "gripperForce",
            "Description": "Force applied by gripper",
            "IsOptional": false,
            "ValueDataType": "number"
          }
        ]
      }
    ]
  },
  "physicalParameters": {
    "speedMax": 2.0,
    "accelerationMax": 1.0,
    "length": 1.2,
    "width": 0.8
  },
  "typeSpecification": {
    "agvClass": "FORKLIFT",
    "seriesName": "Robin"
  }
}
```

#### 4. **주문 응답**
- **Topic**: `meili/v2/Roboligent/{serialNumber}/orderResponse`
- **자동 처리**: `order_executions` 상태 업데이트 (구현 예정)

### 🔄 MQTT 자동 처리 플로우

#### **로봇 연결 시:**
```
로봇 ONLINE 메시지 → connection 토픽 → 
Bridge 자동 factsheet 요청 → instantActions 토픽 → 
로봇 factsheet 응답 → factsheet 토픽 → 
Bridge DB 업데이트
```

#### **주문 실행 시:**
```
API 호출 → DB 조회 (템플릿/액션) → 
MQTT 메시지 생성 → order 토픽 → 
로봇 실행 → orderResponse 토픽 → 
Bridge 상태 업데이트
```

#### **즉시 액션 시:**
```
API 호출 → 온라인 상태 확인 → 
MQTT 메시지 생성 → instantActions 토픽 → 
로봇 즉시 실행
```

### 📊 MQTT 메시지 빈도 분석

**🔥 고빈도 토픽:**
- `meili/v2/Roboligent/+/state` (로봇 상태 - 초당 여러 번)
- `meili/v2/Roboligent/+/connection` (연결 상태 - 연결/해제 시)

**🔄 중빈도 토픽:**
- `meili/v2/Roboligent/+/order` (주문 전송 - 작업 시작 시)
- `meili/v2/Roboligent/+/instantActions` (즉시 액션 - 필요 시)

**📚 저빈도 토픽:**
- `meili/v2/+/+/factsheet` (능력 정보 - 연결 시 1회)
- `meili/v2/Roboligent/+/orderResponse` (주문 응답 - 작업 완료 시)

### ⚠️ MQTT 관련 주의사항

1. **QoS 설정**: 현재 QoS 1 사용 (최소 1회 전달 보장)
2. **메시지 크기**: 대용량 주문 시 MQTT 브로커 제한 고려
3. **연결 상태**: 로봇 오프라인 시 메시지 전송 실패 처리
4. **Header ID 관리**: 로봇별 Header ID 증가 관리
5. **타임스탬프**: UTC 기준 정확한 시간 동기화 필요

### 🔥 고빈도 사용 테이블
1. **`action_templates`** - 모든 템플릿 관련 API에서 사용
2. **`action_parameter_templates`** - 액션과 항상 함께 사용
3. **`order_executions`** - 모든 주문 실행에서 사용
4. **Redis** - 모든 로봇 제어 API에서 상태 확인

### 🔄 중간 빈도 사용 테이블
1. **`node_templates`**, **`edge_templates`** - 템플릿 관리 및 주문 실행
2. **`order_templates`** - 주문 템플릿 관련 API
3. **`connection_states`** - 로봇 상태 조회 API

### 📚 저빈도 사용 테이블
1. **`agv_actions`** - 로봇 능력 조회 시만 사용
2. **`physical_parameters`**, **`type_specifications`** - 로봇 능력 조회
3. **`connection_state_histories`** - 히스토리 조회 시만 사용

---

## ⚠️ 주의사항

### 트랜잭션 필수 API
- 모든 템플릿 생성/수정 API (여러 테이블 동시 작업)
- 주문 실행 API (일관성 보장 필요)

### 성능 고려사항
- `GET /api/v1/order-templates/{id}/details` - 복잡한 JOIN 쿼리
- Bulk 작업 API - 대량 데이터 처리
- 주문 실행 API - 여러 테이블 조회 및 데이터 변환

### 데이터 정합성
- Node/Edge 삭제 시 연관 액션 템플릿도 함께 삭제
- 주문 템플릿 수정 시 기존 연결 삭제 후 재생성
- 액션 템플릿 수정 시 파라미터 전체 교체