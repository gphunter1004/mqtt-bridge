# MQTT Bridge API 문서

## 기본 정보
- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`

---

## 🏥 Health Check

### GET /health
서비스 상태 확인

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/health
```

**응답:**
```json
{
  "status": "healthy",
  "service": "mqtt-bridge",
  "timestamp": "1720251157"
}
```

---

## 🤖 Robot Management

### GET /robots
연결된 로봇 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots
```

**응답:**
```json
{
  "connectedRobots": ["DEX0001", "DEX0002"],
  "count": 2
}
```

### GET /robots/{serialNumber}/state
로봇 상태 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/state
```

**응답:**
```json
{
  "agvPosition": {
    "x": 1.23,
    "y": 4.56,
    "theta": 0.0,
    "positionInitialized": true,
    "mapId": "map_001"
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

### GET /robots/{serialNumber}/health
로봇 헬스 상태 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/health
```

**응답:**
```json
{
  "serialNumber": "DEX0001",
  "isOnline": true,
  "batteryCharge": 85.5,
  "batteryVoltage": 24.2,
  "isCharging": false,
  "positionInitialized": true,
  "hasErrors": false,
  "errorCount": 0,
  "operatingMode": "AUTOMATIC",
  "isPaused": false,
  "isDriving": false,
  "lastUpdate": "2025-07-06T10:30:45Z"
}
```

### GET /robots/{serialNumber}/capabilities
로봇 기능 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/capabilities
```

**응답:**
```json
{
  "serialNumber": "DEX0001",
  "physicalParameters": {
    "speedMax": 2.0,
    "speedMin": 0.1,
    "accelerationMax": 1.0,
    "decelerationMax": 1.5,
    "length": 1.2,
    "width": 0.8,
    "heightMax": 2.0,
    "heightMin": 0.1
  },
  "typeSpecification": {
    "agvClass": "FORKLIFT",
    "agvKinematics": "DIFFERENTIAL",
    "seriesName": "Robin",
    "seriesDescription": "Autonomous Mobile Robot"
  },
  "availableActions": [
    {
      "actionType": "move",
      "actionDescription": "Move to position",
      "parameters": [
        {
          "key": "targetPosition",
          "description": "Target position coordinates",
          "isOptional": false,
          "valueDataType": "object"
        }
      ]
    }
  ]
}
```

### GET /robots/{serialNumber}/history
로봇 연결 히스토리 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/history?limit=10
```

**응답:**
```json
[
  {
    "serialNumber": "DEX0001",
    "connectionState": "ONLINE",
    "timestamp": "2025-07-06T10:30:45Z",
    "createdAt": "2025-07-06T10:30:45Z"
  },
  {
    "serialNumber": "DEX0001",
    "connectionState": "OFFLINE",
    "timestamp": "2025-07-06T09:15:30Z",
    "createdAt": "2025-07-06T09:15:30Z"
  }
]
```

---

## 🎯 Robot Control

### POST /robots/{serialNumber}/order
작업 명령 전송

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/order \
  -H "Content-Type: application/json" \
  -d '{
    "orderId": "order_123",
    "orderUpdateId": 0,
    "nodes": [
      {
        "nodeId": "node_001",
        "description": "Start position",
        "sequenceId": 0,
        "released": true,
        "nodePosition": {
          "x": 0.0,
          "y": 0.0,
          "theta": 0.0,
          "allowedDeviationXY": 0.1,
          "allowedDeviationTheta": 0.1,
          "mapId": "map_001"
        },
        "actions": [
          {
            "actionType": "move",
            "actionId": "action_001",
            "blockingType": "HARD",
            "actionParameters": [
              {
                "key": "speed",
                "value": 1.0
              }
            ]
          }
        ]
      }
    ],
    "edges": []
  }'
```

**응답:**
```json
{
  "status": "success",
  "message": "Order sent successfully to robot DEX0001"
}
```

### POST /robots/{serialNumber}/action
커스텀 액션 전송

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/action \
  -H "Content-Type: application/json" \
  -d '{
    "headerId": 1,
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
  }'
```

**응답:**
```json
{
  "status": "success",
  "message": "Custom action sent successfully to robot DEX0001"
}
```

### POST /robots/{serialNumber}/inference
추론 명령 전송

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/inference \
  -H "Content-Type: application/json" \
  -d '{
    "inferenceName": "object_detection"
  }'
```

**응답:**
```json
{
  "status": "success",
  "message": "Inference order sent successfully to robot DEX0001",
  "action": "inference",
  "inference_name": "object_detection"
}
```

### POST /robots/{serialNumber}/trajectory
궤적 명령 전송

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/robots/DEX0001/trajectory \
  -H "Content-Type: application/json" \
  -d '{
    "trajectoryName": "pick_trajectory_001",
    "arm": "left"
  }'
```

**응답:**
```json
{
  "status": "success",
  "message": "Trajectory order sent successfully to robot DEX0001",
  "action": "trajectory",
  "trajectory_name": "pick_trajectory_001",
  "arm": "left"
}
```

---

## 📋 Order Template Management

### POST /order-templates
주문 템플릿 생성

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/order-templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Pick and Place Template",
    "description": "Standard pick and place operation",
    "nodeIds": ["pick_node_001", "place_node_002"],
    "edgeIds": ["edge_001"]
  }'
```

**응답:**
```json
{
  "id": 1,
  "name": "Pick and Place Template",
  "description": "Standard pick and place operation",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:30:45Z"
}
```

### GET /order-templates
주문 템플릿 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/order-templates?limit=10&offset=0
```

**응답:**
```json
{
  "templates": [
    {
      "id": 1,
      "name": "Pick and Place Template",
      "description": "Standard pick and place operation",
      "createdAt": "2025-07-06T10:30:45Z",
      "updatedAt": "2025-07-06T10:30:45Z"
    }
  ],
  "count": 1
}
```

### GET /order-templates/{id}
특정 주문 템플릿 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/order-templates/1
```

**응답:**
```json
{
  "id": 1,
  "name": "Pick and Place Template",
  "description": "Standard pick and place operation",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:30:45Z"
}
```

### GET /order-templates/{id}/details
주문 템플릿 상세 정보 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/order-templates/1/details
```

**응답:**
```json
{
  "orderTemplate": {
    "id": 1,
    "name": "Pick and Place Template",
    "description": "Standard pick and place operation"
  },
  "nodesWithActions": [
    {
      "nodeTemplate": {
        "id": 1,
        "nodeId": "pick_node_001",
        "name": "Pick Position",
        "x": 1.0,
        "y": 2.0,
        "theta": 0.0
      },
      "actions": [
        {
          "id": 1,
          "actionType": "pick",
          "actionId": "pick_001",
          "parameters": []
        }
      ]
    }
  ],
  "edgesWithActions": []
}
```

### PUT /order-templates/{id}
주문 템플릿 수정

**요청:**
```bash
curl -X PUT http://localhost:8080/api/v1/order-templates/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Pick and Place Template",
    "description": "Updated description",
    "nodeIds": ["pick_node_001", "place_node_002"],
    "edgeIds": ["edge_001"]
  }'
```

### DELETE /order-templates/{id}
주문 템플릿 삭제

**요청:**
```bash
curl -X DELETE http://localhost:8080/api/v1/order-templates/1
```

**응답:**
```json
{
  "status": "success",
  "message": "Order template 1 deleted successfully"
}
```

---

## 🔗 Template Association

### POST /order-templates/{id}/associate-nodes
노드 연결

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/order-templates/1/associate-nodes \
  -H "Content-Type: application/json" \
  -d '{
    "nodeIds": ["node_003", "node_004"]
  }'
```

### POST /order-templates/{id}/associate-edges
엣지 연결

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/order-templates/1/associate-edges \
  -H "Content-Type: application/json" \
  -d '{
    "edgeIds": ["edge_002", "edge_003"]
  }'
```

---

## ⚡ Order Execution

### POST /orders/execute
주문 실행

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/orders/execute \
  -H "Content-Type: application/json" \
  -d '{
    "templateId": 1,
    "serialNumber": "DEX0001",
    "parameterOverrides": {
      "speed": 1.5,
      "timeout": 30
    }
  }'
```

**응답:**
```json
{
  "orderId": "order_19a2b3c4d5e6f",
  "status": "SENT",
  "serialNumber": "DEX0001",
  "orderTemplateId": 1,
  "createdAt": "2025-07-06T10:30:45Z"
}
```

### POST /orders/execute/template/{id}/robot/{serialNumber}
템플릿으로 주문 실행

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/orders/execute/template/1/robot/DEX0001 \
  -H "Content-Type: application/json" \
  -d '{
    "parameterOverrides": {
      "speed": 2.0
    }
  }'
```

### GET /orders
주문 실행 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/orders?serialNumber=DEX0001&limit=10&offset=0
```

**응답:**
```json
{
  "executions": [
    {
      "id": 1,
      "orderId": "order_19a2b3c4d5e6f",
      "serialNumber": "DEX0001",
      "status": "COMPLETED",
      "createdAt": "2025-07-06T10:30:45Z",
      "completedAt": "2025-07-06T10:35:45Z"
    }
  ],
  "count": 1
}
```

### GET /orders/{orderId}
특정 주문 실행 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/orders/order_19a2b3c4d5e6f
```

**응답:**
```json
{
  "id": 1,
  "orderId": "order_19a2b3c4d5e6f",
  "orderTemplateId": 1,
  "serialNumber": "DEX0001",
  "orderUpdateId": 0,
  "status": "COMPLETED",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:35:45Z",
  "startedAt": "2025-07-06T10:30:50Z",
  "completedAt": "2025-07-06T10:35:45Z"
}
```

### POST /orders/{orderId}/cancel
주문 취소

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/orders/order_19a2b3c4d5e6f/cancel
```

**응답:**
```json
{
  "status": "success",
  "message": "Order order_19a2b3c4d5e6f cancelled successfully"
}
```

### GET /robots/{serialNumber}/orders
로봇별 주문 실행 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/robots/DEX0001/orders?limit=10&offset=0
```

---

## 📍 Node Management

### POST /nodes
노드 생성

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "nodeId": "warehouse_pickup_001",
    "name": "Warehouse Pickup Point",
    "description": "Main pickup location in warehouse",
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
        "actionDescription": "Pick up items from shelf",
        "parameters": [
          {
            "key": "shelfLevel",
            "value": 2,
            "valueType": "number"
          },
          {
            "key": "itemType",
            "value": "box",
            "valueType": "string"
          }
        ]
      }
    ]
  }'
```

**응답:**
```json
{
  "id": 1,
  "nodeId": "warehouse_pickup_001",
  "name": "Warehouse Pickup Point",
  "description": "Main pickup location in warehouse",
  "sequenceId": 1,
  "released": true,
  "x": 10.5,
  "y": 15.2,
  "theta": 1.57,
  "allowedDeviationXY": 0.1,
  "allowedDeviationTheta": 0.05,
  "mapId": "warehouse_map_001",
  "actionTemplateIds": "[1]",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:30:45Z"
}
```

### GET /nodes
노드 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/nodes?limit=10&offset=0
```

**응답:**
```json
{
  "nodes": [
    {
      "id": 1,
      "nodeId": "warehouse_pickup_001",
      "name": "Warehouse Pickup Point",
      "x": 10.5,
      "y": 15.2,
      "createdAt": "2025-07-06T10:30:45Z"
    }
  ],
  "count": 1
}
```

### GET /nodes/{nodeId}
특정 노드 조회 (Database ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/nodes/1
```

### GET /nodes/by-node-id/{nodeId}
특정 노드 조회 (Node ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/nodes/by-node-id/warehouse_pickup_001
```

### PUT /nodes/{nodeId}
노드 수정

**요청:**
```bash
curl -X PUT http://localhost:8080/api/v1/nodes/1 \
  -H "Content-Type: application/json" \
  -d '{
    "nodeId": "warehouse_pickup_001",
    "name": "Updated Pickup Point",
    "description": "Updated description",
    "sequenceId": 1,
    "released": true,
    "position": {
      "x": 11.0,
      "y": 15.5,
      "theta": 1.57,
      "allowedDeviationXY": 0.15,
      "allowedDeviationTheta": 0.05,
      "mapId": "warehouse_map_001"
    },
    "actions": []
  }'
```

### DELETE /nodes/{nodeId}
노드 삭제

**요청:**
```bash
curl -X DELETE http://localhost:8080/api/v1/nodes/1
```

**응답:**
```json
{
  "status": "success",
  "message": "Node 1 deleted successfully"
}
```

---

## 🔗 Edge Management

### POST /edges
엣지 생성

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/edges \
  -H "Content-Type: application/json" \
  -d '{
    "edgeId": "path_001_to_002",
    "name": "Pickup to Dropoff Path",
    "description": "Main corridor path from pickup to dropoff",
    "sequenceId": 1,
    "released": true,
    "startNodeId": "warehouse_pickup_001",
    "endNodeId": "warehouse_dropoff_002",
    "actions": [
      {
        "actionType": "navigate",
        "actionId": "nav_001",
        "blockingType": "SOFT",
        "actionDescription": "Navigate along corridor",
        "parameters": [
          {
            "key": "maxSpeed",
            "value": 1.5,
            "valueType": "number"
          },
          {
            "key": "safetyMargin",
            "value": 0.3,
            "valueType": "number"
          }
        ]
      }
    ]
  }'
```

**응답:**
```json
{
  "id": 1,
  "edgeId": "path_001_to_002",
  "name": "Pickup to Dropoff Path",
  "description": "Main corridor path from pickup to dropoff",
  "sequenceId": 1,
  "released": true,
  "startNodeId": "warehouse_pickup_001",
  "endNodeId": "warehouse_dropoff_002",
  "actionTemplateIds": "[1]",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:30:45Z"
}
```

### GET /edges
엣지 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/edges?limit=10&offset=0
```

### GET /edges/{edgeId}
특정 엣지 조회 (Database ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/edges/1
```

### GET /edges/by-edge-id/{edgeId}
특정 엣지 조회 (Edge ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/edges/by-edge-id/path_001_to_002
```

### PUT /edges/{edgeId}
엣지 수정

**요청:**
```bash
curl -X PUT http://localhost:8080/api/v1/edges/1 \
  -H "Content-Type: application/json" \
  -d '{
    "edgeId": "path_001_to_002",
    "name": "Updated Path",
    "description": "Updated description",
    "sequenceId": 1,
    "released": true,
    "startNodeId": "warehouse_pickup_001",
    "endNodeId": "warehouse_dropoff_002",
    "actions": []
  }'
```

### DELETE /edges/{edgeId}
엣지 삭제

**요청:**
```bash
curl -X DELETE http://localhost:8080/api/v1/edges/1
```

---

## ⚙️ Action Template Management

### POST /actions
액션 템플릿 생성

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions \
  -H "Content-Type: application/json" \
  -d '{
    "actionType": "custom_pick",
    "actionId": "pick_template_001",
    "blockingType": "HARD",
    "actionDescription": "Custom picking action with configurable parameters",
    "parameters": [
      {
        "key": "gripperForce",
        "value": 50,
        "valueType": "number"
      },
      {
        "key": "pickHeight",
        "value": 1.2,
        "valueType": "number"
      },
      {
        "key": "itemType",
        "value": "box",
        "valueType": "string"
      },
      {
        "key": "safetyConfig",
        "value": {
          "checkCollision": true,
          "maxRetries": 3
        },
        "valueType": "object"
      }
    ]
  }'
```

**응답:**
```json
{
  "id": 1,
  "actionType": "custom_pick",
  "actionId": "pick_template_001",
  "blockingType": "HARD",
  "actionDescription": "Custom picking action with configurable parameters",
  "createdAt": "2025-07-06T10:30:45Z",
  "updatedAt": "2025-07-06T10:30:45Z",
  "parameters": [
    {
      "id": 1,
      "actionTemplateId": 1,
      "key": "gripperForce",
      "value": "50",
      "valueType": "number"
    },
    {
      "id": 2,
      "actionTemplateId": 1,
      "key": "pickHeight",
      "value": "1.2",
      "valueType": "number"
    }
  ]
}
```

### GET /actions
액션 템플릿 목록 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/actions?limit=10&offset=0&actionType=pick&blockingType=HARD
```

**응답:**
```json
{
  "actions": [
    {
      "id": 1,
      "actionType": "custom_pick",
      "actionId": "pick_template_001",
      "blockingType": "HARD",
      "actionDescription": "Custom picking action with configurable parameters",
      "createdAt": "2025-07-06T10:30:45Z",
      "parameters": []
    }
  ],
  "count": 1
}
```

### GET /actions/{actionId}
특정 액션 템플릿 조회 (Database ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/actions/1
```

### GET /actions/by-action-id/{actionId}
특정 액션 템플릿 조회 (Action ID)

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/actions/by-action-id/pick_template_001
```

### PUT /actions/{actionId}
액션 템플릿 수정

**요청:**
```bash
curl -X PUT http://localhost:8080/api/v1/actions/1 \
  -H "Content-Type: application/json" \
  -d '{
    "actionType": "updated_pick",
    "actionId": "pick_template_001",
    "blockingType": "SOFT",
    "actionDescription": "Updated picking action",
    "parameters": [
      {
        "key": "gripperForce",
        "value": 60,
        "valueType": "number"
      }
    ]
  }'
```

### DELETE /actions/{actionId}
액션 템플릿 삭제

**요청:**
```bash
curl -X DELETE http://localhost:8080/api/v1/actions/1
```

### POST /actions/{actionId}/clone
액션 템플릿 복제

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/1/clone \
  -H "Content-Type: application/json" \
  -d '{
    "newActionId": "pick_template_002"
  }'
```

**응답:**
```json
{
  "status": "success",
  "message": "Action template cloned successfully",
  "clonedAction": {
    "id": 2,
    "actionType": "custom_pick",
    "actionId": "pick_template_002",
    "actionDescription": "Custom picking action with configurable parameters (cloned)"
  }
}
```

---

## 📚 Action Library Management

### POST /actions/library
액션 라이브러리 생성

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/library \
  -H "Content-Type: application/json" \
  -d '{
    "actionType": "standardMove",
    "actionId": "lib_move_001",
    "blockingType": "HARD",
    "actionDescription": "Standard movement action for library",
    "parameters": [
      {
        "key": "speed",
        "value": 1.0,
        "valueType": "number"
      }
    ],
    "category": "movement",
    "tags": ["basic", "movement", "navigation"],
    "isReusable": true
  }'
```

### GET /actions/library
액션 라이브러리 조회

**요청:**
```bash
curl -X GET http://localhost:8080/api/v1/actions/library?limit=50&offset=0
```

**응답:**
```json
{
  "library": [
    {
      "id": 1,
      "actionType": "standardMove",
      "actionId": "lib_move_001",
      "actionDescription": "Standard movement action for library",
      "createdAt": "2025-07-06T10:30:45Z"
    }
  ],
  "count": 1
}
```

---

## 🔍 Action Validation & Bulk Operations

### POST /actions/validate
액션 템플릿 검증

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/validate \
  -H "Content-Type: application/json" \
  -d '{
    "actionType": "move",
    "parameters": [
      {
        "key": "speed",
        "value": 1.0,
        "valueType": "number"
      }
    ],
    "serialNumber": "DEX0001"
  }'
```

**응답:**
```json
{
  "isValid": true,
  "errors": [],
  "warnings": [],
  "suggestions": [],
  "canExecute": true,
  "missingParams": []
}
```

### POST /actions/bulk/delete
액션 템플릿 일괄 삭제

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/bulk/delete \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "delete",
    "actionIds": [1, 2, 3]
  }'
```

**응답:**
```json
{
  "successCount": 2,
  "errorCount": 1,
  "results": [
    {
      "actionId": 1,
      "status": "success",
      "message": ""
    },
    {
      "actionId": 2,
      "status": "success", 
      "message": ""
    },
    {
      "actionId": 3,
      "status": "error",
      "message": "Action template not found"
    }
  ]
}
```

### POST /actions/bulk/clone
액션 템플릿 일괄 복제

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/bulk/clone \
  -H "Content-Type: application/json" \
  -d '{
    "actionIds": [1, 2],
    "prefix": "cloned"
  }'
```

**응답:**
```json
{
  "successCount": 2,
  "errorCount": 0,
  "results": [
    {
      "actionId": 1,
      "status": "success",
      "message": "Cloned to action ID: 4"
    },
    {
      "actionId": 2,
      "status": "success",
      "message": "Cloned to action ID: 5"
    }
  ]
}
```

---

## 📤 Action Import/Export

### POST /actions/export
액션 템플릿 내보내기

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/export \
  -H "Content-Type: application/json" \
  -d '{
    "actionIds": [1, 2, 3],
    "format": "json"
  }' \
  --output actions_export.json
```

**응답:** 파일 다운로드 (JSON 형식)

### POST /actions/import
액션 템플릿 가져오기

**요청:**
```bash
curl -X POST http://localhost:8080/api/v1/actions/import \
  -H "Content-Type: application/json" \
  -d '{
    "actions": [
      {
        "actionType": "imported_action",
        "actionId": "import_001",
        "blockingType": "HARD",
        "actionDescription": "Imported action template",
        "parameters": [
          {
            "key": "param1",
            "value": "value1",
            "valueType": "string"
          }
        ]
      }
    ],
    "options": {
      "overwriteExisting": false,
      "skipDuplicates": true,
      "validateOnly": false
    }
  }'
```

**응답:**
```json
{
  "importedCount": 1,
  "errorCount": 0,
  "results": [
    {
      "actionType": "imported_action",
      "actionId": "import_001",
      "status": "imported",
      "message": "",
      "databaseId": 6
    }
  ],
  "summary": {
    "totalActions": 1,
    "successActions": 1,
    "failedActions": 0,
    "skippedActions": 0,
    "duplicateActions": 0,
    "newActionTypes": ["imported_action"]
  }
}
```

---

## 📊 Common Query Parameters

### Pagination
대부분의 목록 API에서 사용 가능:
- `limit`: 한 번에 가져올 항목 수 (기본값: 10-50)
- `offset`: 건너뛸 항목 수 (기본값: 0)

**예시:**
```bash
curl -X GET "http://localhost:8080/api/v1/nodes?limit=20&offset=40"
```

### Filtering
특정 API에서 사용 가능한 필터:

**Actions API:**
- `actionType`: 액션 타입으로 필터링
- `blockingType`: 블로킹 타입으로 필터링  
- `search`: 텍스트 검색

**Orders API:**
- `serialNumber`: 특정 로봇의 주문만 조회

**예시:**
```bash
curl -X GET "http://localhost:8080/api/v1/actions?actionType=move&blockingType=HARD&search=navigation"
```

---

## 🚨 Error Responses

### 400 Bad Request
잘못된 요청 데이터

**응답:**
```json
{
  "error": "Invalid request body: missing required field 'actionType'"
}
```

### 404 Not Found
리소스를 찾을 수 없음

**응답:**
```json
{
  "error": "Robot DEX0003 not found or not online"
}
```

### 500 Internal Server Error
서버 내부 오류

**응답:**
```json
{
  "error": "Failed to connect to database"
}
```

---

## 🔧 Status Codes Summary

| Status Code | Description |
|-------------|-------------|
| 200 | OK - 성공적인 요청 |
| 201 | Created - 리소스 생성 성공 |
| 400 | Bad Request - 잘못된 요청 |
| 404 | Not Found - 리소스 없음 |
| 500 | Internal Server Error - 서버 오류 |

---

## 🎯 Postman Collection

위의 모든 API를 테스트하기 위한 Postman Collection을 만들 수 있습니다:

1. **Environment Variables 설정:**
   - `BASE_URL`: `http://localhost:8080/api/v1`
   - `ROBOT_SERIAL`: `DEX0001`

2. **Collection 구조:**
   ```
   MQTT Bridge API
   ├── Health Check
   ├── Robot Management
   │   ├── Get Connected Robots
   │   ├── Get Robot State
   │   ├── Get Robot Health
   │   └── Get Robot Capabilities
   ├── Robot Control
   │   ├── Send Order
   │   ├── Send Custom Action
   │   ├── Send Inference Order
   │   └── Send Trajectory Order
   ├── Template Management
   │   ├── Order Templates
   │   ├── Node Templates
   │   ├── Edge Templates
   │   └── Action Templates
   └── Order Execution
       ├── Execute Order
       ├── Get Orders
       └── Cancel Order
   ```

이 API 문서를 통해 MQTT Bridge 서비스의 모든 기능을 테스트하고 활용할 수 있습니다.