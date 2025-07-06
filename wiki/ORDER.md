# Order Template을 사용한 주문 실행 가이드

## 🎯 Order Template 실행 방법

Order Template을 사용해서 실제 로봇에게 주문을 전송하는 방법은 **2가지**가 있습니다.

---

## 방법 1: Template ID로 직접 실행 ⭐ **추천**

### API 엔드포인트
```
POST /api/v1/orders/execute/template/{templateId}/robot/{serialNumber}
```

### 사용 예시
```bash
curl -X POST http://localhost:8080/api/v1/orders/execute/template/1/robot/DEX0001 \
  -H "Content-Type: application/json" \
  -d '{
    "parameterOverrides": {
      "speed": 1.5,
      "gripperForce": 60,
      "timeout": 30
    }
  }'
```

### 파라미터 설명
- **`templateId`**: 실행할 주문 템플릿의 Database ID
- **`serialNumber`**: 주문을 실행할 로봇의 시리얼 번호
- **`parameterOverrides`**: 템플릿의 액션 파라미터를 덮어쓸 값들 (선택사항)

---

## 방법 2: 일반 실행 API 사용

### API 엔드포인트
```
POST /api/v1/orders/execute
```

### 사용 예시
```bash
curl -X POST http://localhost:8080/api/v1/orders/execute \
  -H "Content-Type: application/json" \
  -d '{
    "templateId": 1,
    "serialNumber": "DEX0001",
    "parameterOverrides": {
      "speed": 1.5,
      "gripperForce": 60,
      "timeout": 30
    }
  }'
```

---

## 🔧 전체 실행 프로세스

### 1단계: Order Template 생성 (사전 준비)

먼저 Order Template이 존재해야 합니다. 없다면 생성:

```bash
# 1. Node 생성
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "nodeId": "warehouse_pickup_001",
    "name": "Pickup Point",
    "description": "Main pickup location",
    "sequenceId": 1,
    "released": true,
    "position": {
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
        "actionId": "pick_action_001",
        "blockingType": "HARD",
        "actionDescription": "Pick up items",
        "parameters": [
          {
            "key": "gripperForce",
            "value": 50,
            "valueType": "number"
          },
          {
            "key": "speed",
            "value": 1.0,
            "valueType": "number"
          }
        ]
      }
    ]
  }'

# 2. Edge 생성 (필요시)
curl -X POST http://localhost:8080/api/v1/edges \
  -H "Content-Type: application/json" \
  -d '{
    "edgeId": "path_pickup_to_dropoff",
    "name": "Main Transport Path",
    "description": "Path from pickup to dropoff",
    "sequenceId": 1,
    "released": true,
    "startNodeId": "warehouse_pickup_001",
    "endNodeId": "warehouse_dropoff_002",
    "actions": [
      {
        "actionType": "navigate",
        "actionId": "nav_001",
        "blockingType": "SOFT",
        "actionDescription": "Navigate along path",
        "parameters": [
          {
            "key": "maxSpeed",
            "value": 1.5,
            "valueType": "number"
          }
        ]
      }
    ]
  }'

# 3. Order Template 생성
curl -X POST http://localhost:8080/api/v1/order-templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Standard Pick and Transport",
    "description": "Standard warehouse pick and transport operation",
    "nodeIds": ["warehouse_pickup_001", "warehouse_dropoff_002"],
    "edgeIds": ["path_pickup_to_dropoff"]
  }'
```

### 2단계: Template 확인

생성된 템플릿을 확인:

```bash
# 템플릿 목록 조회
curl -X GET http://localhost:8080/api/v1/order-templates

# 특정 템플릿 상세 조회 (templateId = 1)
curl -X GET http://localhost:8080/api/v1/order-templates/1/details
```

### 3단계: 로봇 상태 확인

주문을 보낼 로봇이 온라인 상태인지 확인:

```bash
# 연결된 로봇 목록
curl -X GET http://localhost:8080/api/v1/robots

# 특정 로봇 상태
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/state
```

### 4단계: Order 실행

Template을 사용해서 주문 실행:

```bash
curl -X POST http://localhost:8080/api/v1/orders/execute/template/1/robot/DEX0001 \
  -H "Content-Type: application/json" \
  -d '{
    "parameterOverrides": {
      "gripperForce": 60,
      "speed": 1.2,
      "maxSpeed": 1.8
    }
  }'
```

---

## 📊 실행 결과 확인

### 성공 응답
```json
{
  "orderId": "order_19a2b3c4d5e6f",
  "status": "SENT",
  "serialNumber": "DEX0001",
  "orderTemplateId": 1,
  "createdAt": "2025-07-06T10:30:45Z"
}
```

### 주문 상태 추적
```bash
# 특정 주문 상태 조회
curl -X GET http://localhost:8080/api/v1/orders/order_19a2b3c4d5e6f

# 로봇별 주문 목록
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/orders

# 전체 주문 목록
curl -X GET http://localhost:8080/api/v1/orders
```

---

## ⚙️ Parameter Override 상세

### Parameter Override 작동 방식

Template의 액션에 정의된 파라미터를 실행 시점에 덮어쓸 수 있습니다:

**Template의 원본 파라미터:**
```json
{
  "actionType": "pick",
  "parameters": [
    {
      "key": "gripperForce",
      "value": 50,
      "valueType": "number"
    },
    {
      "key": "speed", 
      "value": 1.0,
      "valueType": "number"
    }
  ]
}
```

**Override 적용:**
```json
{
  "parameterOverrides": {
    "gripperForce": 60,  // 50 → 60으로 변경
    "speed": 1.5         // 1.0 → 1.5로 변경
  }
}
```

**실제 로봇으로 전송되는 파라미터:**
```json
{
  "actionType": "pick",
  "actionParameters": [
    {
      "key": "gripperForce",
      "value": 60      // Override된 값
    },
    {
      "key": "speed",
      "value": 1.5     // Override된 값
    }
  ]
}
```

### Override 규칙
1. **키 매칭**: `parameterOverrides`의 키와 액션 파라미터의 `key`가 일치해야 함
2. **타입 무관**: Override 시 타입 변환은 자동으로 처리
3. **선택적**: Override하지 않은 파라미터는 원본 값 유지
4. **전역 적용**: Template 내 모든 액션의 동일한 키에 적용

---

## 🔄 내부 처리 과정

### 1. Template → MQTT 변환 과정

```
1. Template 조회 (order_templates + 연관 테이블들)
   ↓
2. Node/Edge 정보 수집 (node_templates, edge_templates)
   ↓
3. 각 Node/Edge의 Action Template 조회 (action_templates)
   ↓
4. Action Parameter 수집 (action_parameter_templates)
   ↓
5. Parameter Override 적용
   ↓
6. MQTT OrderMessage 생성
   ↓
7. 로봇으로 전송 (meili/v2/Roboligent/{serialNumber}/order)
```

### 2. 데이터베이스 영향

**READ 작업:**
- `order_templates`
- `order_template_nodes`, `order_template_edges`
- `node_templates`, `edge_templates`
- `action_templates`, `action_parameter_templates`

**WRITE 작업:**
- `order_executions` INSERT (주문 실행 기록)
- `order_executions` UPDATE (상태 변경)

---

## ❌ 자주 발생하는 오류

### 1. 로봇 오프라인
```json
{
  "error": "robot DEX0001 is not online"
}
```
**해결**: 로봇 연결 상태 확인

### 2. Template 없음
```json
{
  "error": "failed to get order template: record not found"
}
```
**해결**: 올바른 Template ID 확인

### 3. Node/Edge 참조 오류
```json
{
  "error": "node 'invalid_node_001' not found"
}
```
**해결**: Template에 연결된 Node/Edge가 존재하는지 확인

---

## 🎯 Best Practices

### 1. Template 설계
- **재사용 가능한 구조**로 설계
- **적절한 기본값** 설정
- **명확한 naming convention** 사용

### 2. Parameter Override 활용
- **동적 값**만 Override 사용
- **타입 안전성** 고려
- **필수 파라미터** 누락 방지

### 3. 오류 처리
- **로봇 상태** 사전 확인
- **Template 유효성** 검증
- **주문 실행 결과** 모니터링

### 4. 모니터링
```bash
# 주기적으로 주문 상태 확인
curl -X GET http://localhost:8080/api/v1/orders/order_19a2b3c4d5e6f

# 로봇 상태 모니터링
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/state
```

이렇게 Order Template을 활용하면 **재사용 가능한 작업 패턴**을 정의하고, **파라미터만 조정**해서 다양한 상황에 맞는 주문을 쉽게 실행할 수 있습니다!