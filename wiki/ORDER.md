# Order Template을 사용한 주문 실행 가이드 + Enhanced Robot Control APIs

## 🎯 주문 실행 방법 개요

로봇에게 작업을 지시하는 방법은 **크게 3가지 카테고리**로 나뉩니다:

### 📋 **1. Template 기반 주문 (추천)**
- 미리 정의된 템플릿을 사용
- 재사용 가능하고 관리하기 쉬움
- 파라미터만 조정해서 다양한 상황에 적용

### ⚡ **2. Enhanced Robot Control APIs (NEW)**
- 빠르고 간편한 작업 실행
- 기본 → 위치 지정 → 완전 커스터마이징 3단계 지원
- 템플릿 없이도 즉시 실행 가능

### 🛠️ **3. Direct Order/Action APIs**
- 완전한 제어를 위한 저수준 API
- 복잡한 커스터마이징 필요 시 사용

---

## 📋 방법 1: Order Template 실행 ⭐ **추천**

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

### 2. Enhanced APIs → MQTT 변환 과정

```
1. API 요청 수신 (Enhanced Robot Control)
   ↓
2. 요청 타입별 처리 (Basic/WithPosition/Custom/Dynamic)
   ↓
3. 동적 노드/엣지/액션 생성
   ↓
4. 커스텀 파라미터 적용
   ↓
5. MQTT OrderMessage 생성
   ↓
6. 로봇으로 전송 (meili/v2/Roboligent/{serialNumber}/order)
```

### 3. 데이터베이스 영향

**Template 기반 실행:**
- **READ**: `order_templates`, `node_templates`, `edge_templates`, `action_templates`
- **WRITE**: `order_executions` INSERT/UPDATE

**Enhanced APIs 실행:**
- **READ**: Redis (로봇 상태)
- **WRITE**: `order_executions` INSERT/UPDATE

---

## 🎯 Enhanced APIs 사용 시나리오별 가이드

### 🔰 **시나리오 1: 빠른 테스트 (Basic APIs)**

**상황**: 로봇이 제대로 동작하는지 빠르게 테스트하고 싶을 때

```bash
# 간단한 추론 테스트
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference \
  -H "Content-Type: application/json" \
  -d '{"inferenceName": "connectivity_test"}'

# 간단한 궤적 테스트
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory \
  -H "Content-Type: application/json" \
  -d '{"trajectoryName": "home_position", "arm": "both"}'
```

**특징:**
- 최소한의 파라미터
- 빠른 실행
- 개발/테스트 단계에 적합

---

### 📍 **시나리오 2: 특정 위치에서 작업 (With Position APIs)**

**상황**: 정확한 위치에서 품질 검사나 픽업 작업을 수행해야 할 때

```bash
# 특정 위치에서 품질 검사
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference/with-position \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "quality_inspection",
    "position": {
      "x": 12.34,
      "y": 56.78,
      "theta": 1.5708,
      "allowedDeviationXY": 0.05,
      "allowedDeviationTheta": 0.01,
      "mapId": "production_line_A"
    }
  }'

# 정밀한 위치에서 궤적 실행
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory/with-position \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "precision_assembly",
    "arm": "left",
    "position": {
      "x": 8.90,
      "y": 12.34,
      "theta": 0.0,
      "allowedDeviationXY": 0.02,
      "allowedDeviationTheta": 0.005,
      "mapId": "assembly_station_B"
    }
  }'
```

**특징:**
- 정밀한 위치 제어
- 맵 기반 좌표 시스템
- 허용 편차 설정 가능
- 생산라인/조립 작업에 적합

---

### 🎛️ **시나리오 3: 고급 커스터마이징 (Custom APIs)**

**상황**: 복잡한 파라미터와 설정이 필요한 고급 작업

```bash
# 고급 AI 추론 작업
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference/custom \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "multi_model_analysis",
    "description": "다중 모델 기반 종합 분석",
    "sequenceId": 1,
    "released": true,
    "position": {
      "x": 15.67,
      "y": 23.45,
      "theta": 2.356,
      "allowedDeviationXY": 0.03,
      "allowedDeviationTheta": 0.01,
      "mapId": "research_lab"
    },
    "actionType": "Multi-Model AI Analysis",
    "actionDescription": "여러 AI 모델을 동시 실행하여 종합 분석",
    "blockingType": "HARD",
    "customParameters": {
      "models": ["yolo_v8", "efficientnet", "transformer"],
      "confidence_threshold": 0.92,
      "batch_processing": true,
      "max_processing_time": 300,
      "output_format": "detailed_json",
      "gpu_allocation": {
        "primary": "cuda:0",
        "secondary": "cuda:1"
      },
      "post_processing": {
        "noise_reduction": true,
        "edge_enhancement": true,
        "color_correction": "auto"
      }
    }
  }'

# 고급 듀얼 암 조립 작업
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory/custom \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "complex_dual_arm_assembly",
    "arm": "dual",
    "description": "복잡한 듀얼 암 동기화 조립",
    "sequenceId": 1,
    "released": true,
    "position": {
      "x": 10.11,
      "y": 20.22,
      "theta": 1.5708,
      "allowedDeviationXY": 0.01,
      "allowedDeviationTheta": 0.005,
      "mapId": "precision_assembly"
    },
    "actionType": "Advanced Dual Arm Coordination",
    "actionDescription": "고정밀 듀얼 암 협업 조립 작업",
    "blockingType": "HARD",
    "customParameters": {
      "coordination_mode": "master_slave",
      "master_arm": "right",
      "slave_arm": "left",
      "sync_tolerance": 0.001,
      "force_feedback": {
        "enabled": true,
        "sensitivity": "high",
        "max_force": 20.0
      },
      "assembly_steps": [
        {
          "step": 1,
          "action": "align_components",
          "timeout": 30,
          "precision": "ultra_high"
        },
        {
          "step": 2,
          "action": "apply_fastener",
          "torque": 2.5,
          "verification": true
        }
      ],
      "safety_parameters": {
        "collision_detection": "active",
        "emergency_stop_distance": 0.05,
        "workspace_monitoring": true
      }
    }
  }'
```

**특징:**
- 모든 파라미터 완전 제어
- 복잡한 설정 지원
- 고급 AI/로봇 기능 활용
- 연구개발/프로토타입에 적합

---

### 🌐 **시나리오 4: 복합 워크플로우 (Dynamic Order)**

**상황**: 여러 단계의 복잡한 작업 프로세스를 하나의 주문으로 실행

```bash
# 완전한 Pick & Place 워크플로우
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/order/dynamic \
  -H "Content-Type: application/json" \
  -d '{
    "orderUpdateId": 0,
    "nodes": [
      {
        "nodeId": "barcode_scan_station",
        "description": "바코드 스캔 스테이션",
        "sequenceId": 0,
        "released": true,
        "nodePosition": {
          "x": 1.0,
          "y": 2.0,
          "theta": 0.0,
          "allowedDeviationXY": 0.1,
          "allowedDeviationTheta": 0.05,
          "mapId": "warehouse_floor_1"
        },
        "actions": [
          {
            "actionType": "barcode_scan",
            "actionId": "scan_incoming_item",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "scan_attempts", "value": 5 },
              { "key": "scan_timeout", "value": 30 },
              { "key": "verification_required", "value": true }
            ]
          }
        ]
      },
      {
        "nodeId": "quality_inspection_point",
        "description": "품질 검사 포인트",
        "sequenceId": 1,
        "released": true,
        "nodePosition": {
          "x": 5.5,
          "y": 8.2,
          "theta": 1.5708,
          "allowedDeviationXY": 0.05,
          "allowedDeviationTheta": 0.02,
          "mapId": "warehouse_floor_1"
        },
        "actions": [
          {
            "actionType": "visual_inspection",
            "actionId": "quality_check",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "inspection_model", "value": "defect_detection_v3" },
              { "key": "confidence_threshold", "value": 0.85 },
              { "key": "save_images", "value": true },
              { "key": "detailed_report", "value": true }
            ]
          }
        ]
      },
      {
        "nodeId": "precision_pick_location",
        "description": "정밀 픽업 위치",
        "sequenceId": 2,
        "released": true,
        "nodePosition": {
          "x": 12.8,
          "y": 15.3,
          "theta": 3.14159,
          "allowedDeviationXY": 0.02,
          "allowedDeviationTheta": 0.01,
          "mapId": "warehouse_floor_1"
        },
        "actions": [
          {
            "actionType": "precision_pick",
            "actionId": "careful_pickup",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "grip_force", "value": 65 },
              { "key": "approach_speed", "value": 0.2 },
              { "key": "lift_height", "value": 0.15 },
              { "key": "stability_check", "value": true }
            ]
          }
        ]
      },
      {
        "nodeId": "final_placement_zone",
        "description": "최종 배치 구역",
        "sequenceId": 3,
        "released": true,
        "nodePosition": {
          "x": 20.1,
          "y": 10.7,
          "theta": 0.0,
          "allowedDeviationXY": 0.05,
          "allowedDeviationTheta": 0.02,
          "mapId": "warehouse_floor_1"
        },
        "actions": [
          {
            "actionType": "gentle_placement",
            "actionId": "final_place",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "placement_force", "value": 8 },
              { "key": "release_height", "value": 0.03 },
              { "key": "final_verification", "value": true },
              { "key": "completion_report", "value": true }
            ]
          }
        ]
      }
    ],
    "edges": [
      {
        "edgeId": "scan_to_inspection",
        "sequenceId": 0,
        "released": true,
        "startNodeId": "barcode_scan_station",
        "endNodeId": "quality_inspection_point",
        "actions": [
          {
            "actionType": "standard_navigate",
            "actionId": "move_to_inspection",
            "blockingType": "SOFT",
            "actionParameters": [
              { "key": "max_speed", "value": 1.8 },
              { "key": "acceleration", "value": 1.2 }
            ]
          }
        ]
      },
      {
        "edgeId": "inspection_to_pick",
        "sequenceId": 1,
        "released": true,
        "startNodeId": "quality_inspection_point",
        "endNodeId": "precision_pick_location",
        "actions": [
          {
            "actionType": "careful_navigate",
            "actionId": "approach_pick_zone",
            "blockingType": "SOFT",
            "actionParameters": [
              { "key": "max_speed", "value": 1.2 },
              { "key": "deceleration_zone", "value": 2.0 },
              { "key": "obstacle_avoidance", "value": "enhanced" }
            ]
          }
        ]
      },
      {
        "edgeId": "pick_to_place",
        "sequenceId": 2,
        "released": true,
        "startNodeId": "precision_pick_location",
        "endNodeId": "final_placement_zone",
        "actions": [
          {
            "actionType": "cargo_navigate",
            "actionId": "transport_with_cargo",
            "blockingType": "SOFT",
            "actionParameters": [
              { "key": "max_speed", "value": 1.0 },
              { "key": "smooth_acceleration", "value": true },
              { "key": "vibration_dampening", "value": "active" },
              { "key": "cargo_monitoring", "value": true }
            ]
          }
        ]
      }
    ]
  }'
```

**특징:**
- 완전한 워크플로우 정의
- 다단계 순차 실행
- 각 단계별 세밀한 제어
- 복잡한 물류/제조 프로세스

---

## ❌ 자주 발생하는 오류 및 해결책

### 1. 로봇 오프라인 오류
```json
{
  "error": "robot DEX0001 is not online"
}
```
**해결책:**
```bash
# 로봇 연결 상태 확인
curl -X GET http://localhost:8080/api/v1/robots
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/health
```

### 2. Template 관련 오류
```json
{
  "error": "failed to get order template: record not found"
}
```
**해결책:**
```bash
# Template 존재 확인
curl -X GET http://localhost:8080/api/v1/order-templates
curl -X GET http://localhost:8080/api/v1/order-templates/1/details
```

### 3. Enhanced API 파라미터 오류
```json
{
  "error": "inferenceName is required"
}
```
**해결책:**
- 필수 파라미터 확인
- 요청 JSON 구조 검증
- API 문서의 예시와 비교

### 4. 위치 좌표 오류
```json
{
  "error": "Invalid position coordinates"
}
```
**해결책:**
- 맵 범위 내 좌표인지 확인
- 허용 편차 값이 적절한지 검증
- mapId가 올바른지 확인

---

## 🎯 Best Practices

### 1. API 선택 가이드

```
📊 작업 복잡도에 따른 API 선택:

간단한 테스트        → Basic APIs (/inference, /trajectory)
위치 지정 필요       → With Position APIs (/inference/with-position)
고급 설정 필요       → Custom APIs (/inference/custom)
복합 워크플로우      → Dynamic Order API (/order/dynamic)
재사용 가능한 패턴   → Template 기반 (/orders/execute/template)
```

### 2. 개발 단계별 접근

```
🔄 개발 프로세스:

1. 프로토타입 단계  → Basic APIs로 빠른 테스트
2. 기능 검증 단계   → With Position APIs로 정밀도 확인
3. 고도화 단계     → Custom APIs로 최적화
4. 운영 단계       → Template으로 표준화
5. 복합 작업       → Dynamic Order로 워크플로우 구현
```

### 3. 파라미터 관리

```bash
# 환경별 설정 분리
export ROBOT_SERIAL="DEX0001"
export API_BASE="http://localhost:8080/api/v1"

# 개발 환경 설정
export INFERENCE_MODEL="dev_model_v1"
export MAX_SPEED="1.0"

# 운영 환경 설정
export INFERENCE_MODEL="prod_model_v2"
export MAX_SPEED="2.0"
```

### 4. 오류 처리 및 모니터링

```bash
# 주문 실행 후 상태 모니터링
ORDER_ID=$(curl -s -X POST $API_BASE/robots/$ROBOT_SERIAL/inference \
  -H "Content-Type: application/json" \
  -d '{"inferenceName": "object_detection"}' | jq -r '.orderId')

# 주문 상태 추적
while true; do
  STATUS=$(curl -s $API_BASE/orders/$ORDER_ID | jq -r '.status')
  echo "Order Status: $STATUS"
  
  if [[ "$STATUS" == "COMPLETED" || "$STATUS" == "FAILED" ]]; then
    break
  fi
  
  sleep 5
done
```

### 5. 성능 최적화 팁

1. **배치 처리**: 여러 작업을 Dynamic Order로 묶어서 실행
2. **캐싱**: 자주 사용하는 Template은 미리 생성해두기
3. **비동기 처리**: 여러 로봇에 동시 작업 할당
4. **모니터링**: 주문 실행 시간과 성공률 추적

---

## 📈 활용 시나리오 요약

| 시나리오 | 추천 API | 복잡도 | 설정 시간 | 재사용성 |
|---------|---------|--------|-----------|----------|
| 빠른 테스트 | Basic APIs | ⭐ | 1분 | ⭐ |
| 정밀 작업 | With Position | ⭐⭐ | 5분 | ⭐⭐ |
| 고급 기능 | Custom APIs | ⭐⭐⭐ | 15분 | ⭐⭐ |
| 복합 워크플로우 | Dynamic Order | ⭐⭐⭐⭐ | 30분 | ⭐⭐⭐ |
| 표준화된 작업 | Template 기반 | ⭐⭐ | 60분 | ⭐⭐⭐⭐⭐ |

이렇게 다양한 접근 방식을 통해 **단순한 테스트부터 복잡한 워크플로우까지** 모든 시나리오를 효율적으로 처리할 수 있습니다!://localhost:8080/api/v1/orders/execute/template/1/robot/DEX0001 \
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

## ⚡ 방법 2: Enhanced Robot Control APIs ⭐ **NEW**

새롭게 추가된 Enhanced APIs는 **3단계 복잡성**을 제공합니다:

### 🔰 **1단계: 기본 실행 (Simple)**
가장 간단한 형태의 작업 실행

#### **추론 실행:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "object_detection"
  }'
```

#### **궤적 실행:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "pick_sequence_A",
    "arm": "left"
  }'
```

**특징:**
- 기본 위치 (0, 0, 0) 사용
- 최소한의 파라미터만 필요
- 빠른 테스트 및 프로토타이핑에 적합

---

### 📍 **2단계: 위치 지정 실행 (With Position)**
특정 위치에서 작업을 실행

#### **위치 지정 추론:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference/with-position \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "quality_inspection",
    "position": {
      "x": 10.5,
      "y": 15.2,
      "theta": 1.57,
      "allowedDeviationXY": 0.1,
      "allowedDeviationTheta": 0.05,
      "mapId": "warehouse_map_001"
    }
  }'
```

#### **위치 지정 궤적:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory/with-position \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "precision_pick",
    "arm": "right",
    "position": {
      "x": 5.2,
      "y": 8.7,
      "theta": 3.14,
      "allowedDeviationXY": 0.05,
      "allowedDeviationTheta": 0.02,
      "mapId": "production_floor"
    }
  }'
```

**특징:**
- 정확한 위치 좌표 지정
- 허용 편차 설정 가능
- 맵 기반 위치 지정
- 정밀한 위치 제어가 필요한 작업에 적합

---

### 🎛️ **3단계: 완전 커스터마이징 (Custom)**
모든 파라미터를 완전히 제어

#### **커스텀 추론:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference/custom \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "advanced_inspection",
    "description": "고급 품질 검사",
    "sequenceId": 1,
    "released": true,
    "position": {
      "x": 12.0,
      "y": 18.5,
      "theta": 3.14,
      "allowedDeviationXY": 0.05,
      "allowedDeviationTheta": 0.01,
      "mapId": "quality_control_zone"
    },
    "actionType": "Advanced Quality Inspection",
    "actionDescription": "상세 품질 검사 수행",
    "blockingType": "HARD",
    "customParameters": {
      "inspection_level": "detailed",
      "timeout": 120,
      "retry_count": 3,
      "camera_settings": {
        "resolution": "4K",
        "lighting": "auto",
        "focus_mode": "continuous"
      },
      "ai_model": "quality_v2.1",
      "confidence_threshold": 0.95
    },
    "edges": []
  }'
```

#### **커스텀 궤적:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory/custom \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "dual_arm_assembly",
    "arm": "dual",
    "description": "듀얼 암 조립 시퀀스",
    "sequenceId": 2,
    "released": true,
    "position": {
      "x": 7.8,
      "y": 11.2,
      "theta": 1.57,
      "allowedDeviationXY": 0.02,
      "allowedDeviationTheta": 0.01,
      "mapId": "assembly_line_B"
    },
    "actionType": "Dual Arm Assembly",
    "actionDescription": "동기화된 듀얼 암 조립 작업",
    "blockingType": "HARD",
    "customParameters": {
      "sync_mode": "coordinated",
      "speed_multiplier": 0.8,
      "force_limit": 15.0,
      "collision_avoidance": true,
      "assembly_sequence": ["component_A", "component_B", "fastener"],
      "quality_check": {
        "enabled": true,
        "check_points": [0.5, 0.8, 1.0],
        "tolerance": 0.1
      }
    }
  }'
```

**특징:**
- 모든 파라미터 완전 제어
- 복잡한 커스텀 파라미터 지원
- 액션 타입 및 블로킹 타입 지정
- 고급 설정 및 워크플로우 구성

---

### 🌐 **3단계+: 동적 다중 작업 (Dynamic Order)**
완전히 자유로운 다중 노드/엣지 워크플로우

```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/order/dynamic \
  -H "Content-Type: application/json" \
  -d '{
    "orderUpdateId": 0,
    "nodes": [
      {
        "nodeId": "dynamic_scan_001",
        "description": "스캔 포인트",
        "sequenceId": 0,
        "released": true,
        "nodePosition": {
          "x": 2.0,
          "y": 3.0,
          "theta": 0.0,
          "allowedDeviationXY": 0.1,
          "allowedDeviationTheta": 0.05,
          "mapId": "warehouse_A"
        },
        "actions": [
          {
            "actionType": "barcode_scan",
            "actionId": "scan_001",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "scan_mode", "value": "comprehensive" },
              { "key": "retry_limit", "value": 5 }
            ]
          }
        ]
      },
      {
        "nodeId": "dynamic_pick_002",
        "description": "픽업 포인트",
        "sequenceId": 1,
        "released": true,
        "nodePosition": {
          "x": 8.5,
          "y": 12.0,
          "theta": 1.57,
          "allowedDeviationXY": 0.05,
          "allowedDeviationTheta": 0.02,
          "mapId": "warehouse_A"
        },
        "actions": [
          {
            "actionType": "precision_pick",
            "actionId": "pick_002",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "grip_force", "value": 80 },
              { "key": "approach_speed", "value": 0.3 },
              { "key": "lift_height", "value": 0.2 }
            ]
          }
        ]
      },
      {
        "nodeId": "dynamic_place_003",
        "description": "배치 포인트",
        "sequenceId": 2,
        "released": true,
        "nodePosition": {
          "x": 15.0,
          "y": 8.0,
          "theta": 3.14,
          "allowedDeviationXY": 0.1,
          "allowedDeviationTheta": 0.05,
          "mapId": "warehouse_A"
        },
        "actions": [
          {
            "actionType": "gentle_place",
            "actionId": "place_003",
            "blockingType": "HARD",
            "actionParameters": [
              { "key": "release_height", "value": 0.05 },
              { "key": "placement_force", "value": 5 },
              { "key": "confirm_placement", "value": true }
            ]
          }
        ]
      }
    ],
    "edges": [
      {
        "edgeId": "path_scan_to_pick",
        "sequenceId": 0,
        "released": true,
        "startNodeId": "dynamic_scan_001",
        "endNodeId": "dynamic_pick_002",
        "actions": [
          {
            "actionType": "fast_navigate",
            "actionId": "nav_001",
            "blockingType": "SOFT",
            "actionParameters": [
              { "key": "max_speed", "value": 2.5 },
              { "key": "acceleration", "value": 1.5 }
            ]
          }
        ]
      },
      {
        "edgeId": "path_pick_to_place",
        "sequenceId": 1,
        "released": true,
        "startNodeId": "dynamic_pick_002",
        "endNodeId": "dynamic_place_003",
        "actions": [
          {
            "actionType": "careful_navigate",
            "actionId": "nav_002",
            "blockingType": "SOFT",
            "actionParameters": [
              { "key": "max_speed", "value": 1.5 },
              { "key": "safety_margin", "value": 0.5 },
              { "key": "vibration_damping", "value": true }
            ]
          }
        ]
      }
    ]
  }'
```

**특징:**
- 다중 노드/엣지 워크플로우
- 순차적 작업 실행
- 각 단계별 세밀한 제어
- 복잡한 물류/제조 프로세스 구현 가능

---

## 🛠️ 방법 3: Direct Order/Action APIs

### 직접 Order API
```
POST /api/v1/robots/{serialNumber}/order
```

### 직접 Action API
```
POST /api/v1/robots/{serialNumber}/action
```

**특징:**
- 완전한 저수준 제어
- MQTT 메시지 구조 직접 정의
- 고급 사용자용

---

## 🔧 전체 Template 실행 프로세스 (방법 1)

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
curl -X POST http