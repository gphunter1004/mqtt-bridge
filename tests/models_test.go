package tests

import (
	"encoding/json"
	"testing"
	"topic-data-converter/models"
)

func TestPLCDataParsing(t *testing.T) {
	t.Run("Parse PLC Status String Format", func(t *testing.T) {
		payload := []byte("device1:running:100,200,300")
		status, err := models.ParsePLCStatus(payload)

		if err != nil {
			t.Fatalf("Failed to parse PLC status: %v", err)
		}

		if status.DeviceID != "device1" {
			t.Errorf("Expected DeviceID 'device1', got '%s'", status.DeviceID)
		}

		if status.Status != "running" {
			t.Errorf("Expected Status 'running', got '%s'", status.Status)
		}

		expectedValues := []int{100, 200, 300}
		if len(status.Values) != len(expectedValues) {
			t.Errorf("Expected %d values, got %d", len(expectedValues), len(status.Values))
		}

		for i, expected := range expectedValues {
			if status.Values[i] != expected {
				t.Errorf("Expected value[%d] = %d, got %d", i, expected, status.Values[i])
			}
		}
	})

	t.Run("Parse PLC Command String Format", func(t *testing.T) {
		payload := []byte("cmd001:move:10.5,20.3,30.1")
		command, err := models.ParsePLCCommand(payload)

		if err != nil {
			t.Fatalf("Failed to parse PLC command: %v", err)
		}

		if command.CommandID != "cmd001" {
			t.Errorf("Expected CommandID 'cmd001', got '%s'", command.CommandID)
		}

		if command.Action != "move" {
			t.Errorf("Expected Action 'move', got '%s'", command.Action)
		}

		expectedParams := []string{"10.5", "20.3", "30.1"}
		if len(command.Params) != len(expectedParams) {
			t.Errorf("Expected %d params, got %d", len(expectedParams), len(command.Params))
		}

		for i, expected := range expectedParams {
			if command.Params[i] != expected {
				t.Errorf("Expected param[%d] = '%s', got '%s'", i, expected, command.Params[i])
			}
		}
	})

	t.Run("Parse Generic JSON Data", func(t *testing.T) {
		topic := "plc/data"
		payload := []byte(`{"temperature": 25.5, "humidity": 60.2}`)

		plcData, err := models.ParsePLCData(topic, payload)
		if err != nil {
			t.Fatalf("Failed to parse PLC data: %v", err)
		}

		if plcData.Topic != topic {
			t.Errorf("Expected topic '%s', got '%s'", topic, plcData.Topic)
		}

		// Check if data is parsed as map
		dataMap, ok := plcData.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected data to be map[string]interface{}, got %T", plcData.Data)
		}

		if temp, exists := dataMap["temperature"]; !exists || temp != 25.5 {
			t.Errorf("Expected temperature 25.5, got %v", temp)
		}

		if humidity, exists := dataMap["humidity"]; !exists || humidity != 60.2 {
			t.Errorf("Expected humidity 60.2, got %v", humidity)
		}
	})

	t.Run("Parse String Array Data", func(t *testing.T) {
		topic := "plc/data"
		payload := []byte("value1,value2,value3")

		plcData, err := models.ParsePLCData(topic, payload)
		if err != nil {
			t.Fatalf("Failed to parse PLC data: %v", err)
		}

		// Check if data is parsed as string array
		dataArray, ok := plcData.Data.([]string)
		if !ok {
			t.Fatalf("Expected data to be []string, got %T", plcData.Data)
		}

		expectedValues := []string{"value1", "value2", "value3"}
		if len(dataArray) != len(expectedValues) {
			t.Errorf("Expected %d values, got %d", len(expectedValues), len(dataArray))
		}

		for i, expected := range expectedValues {
			if dataArray[i] != expected {
				t.Errorf("Expected value[%d] = '%s', got '%s'", i, expected, dataArray[i])
			}
		}
	})
}

func TestRobotDataCreation(t *testing.T) {
	t.Run("Create Robot Status from PLC Status", func(t *testing.T) {
		plcStatus := &models.PLCStatus{
			DeviceID: "device1",
			Status:   "running",
			Values:   []int{100, 200, 300},
		}

		robotStatus := models.NewRobotStatus(plcStatus)

		if robotStatus.DeviceID != plcStatus.DeviceID {
			t.Errorf("Expected DeviceID '%s', got '%s'", plcStatus.DeviceID, robotStatus.DeviceID)
		}

		if robotStatus.Status != plcStatus.Status {
			t.Errorf("Expected Status '%s', got '%s'", plcStatus.Status, robotStatus.Status)
		}

		if len(robotStatus.Values) != len(plcStatus.Values) {
			t.Errorf("Expected %d values, got %d", len(plcStatus.Values), len(robotStatus.Values))
		}

		// Check metadata
		if robotStatus.Metadata["converted_from"] != "plc_status" {
			t.Errorf("Expected metadata converted_from = 'plc_status', got '%v'", robotStatus.Metadata["converted_from"])
		}
	})

	t.Run("Create Robot Data JSON", func(t *testing.T) {
		testData := map[string]interface{}{
			"test":   "value",
			"number": 123,
		}

		robotData := models.NewRobotData("plc/test", "test_command", testData)

		jsonData, err := robotData.ToJSON()
		if err != nil {
			t.Fatalf("Failed to convert to JSON: %v", err)
		}

		// Parse back to verify
		var parsed map[string]interface{}
		if err := json.Unmarshal(jsonData, &parsed); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if parsed["source"] != "plc/test" {
			t.Errorf("Expected source 'plc/test', got '%v'", parsed["source"])
		}

		if parsed["command"] != "test_command" {
			t.Errorf("Expected command 'test_command', got '%v'", parsed["command"])
		}
	})
}
