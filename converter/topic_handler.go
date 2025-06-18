package converter

import (
	"strings"

	"topic-data-converter/config"
	"topic-data-converter/models"
	"topic-data-converter/utils"
)

// StatusHandler handles PLC status topics
type StatusHandler struct {
	config *config.Config
	logger *utils.Logger
}

func NewStatusHandler(cfg *config.Config, logger *utils.Logger) *StatusHandler {
	return &StatusHandler{
		config: cfg,
		logger: logger,
	}
}

func (h *StatusHandler) CanHandle(topic string) bool {
	return strings.HasPrefix(topic, h.config.PLCTopicPrefix) && strings.HasSuffix(topic, "status")
}

func (h *StatusHandler) Convert(topic string, payload []byte) ([]byte, string, error) {
	h.logger.Infof("📊 STATUS HANDLER - Converting data from topic: %s", topic)
	h.logger.Debugf("📊 STATUS INPUT - %s", string(payload))

	// Parse PLC status
	plcStatus, err := models.ParsePLCStatus(payload)
	if err != nil {
		h.logger.Errorf("❌ STATUS PARSE FAILED - Error: %v", err)
		return nil, "", err
	}

	h.logger.Infof("📊 STATUS PARSED - Device: %s, Status: %s, Values: %v",
		plcStatus.DeviceID, plcStatus.Status, plcStatus.Values)

	// Convert to robot status
	robotStatus := models.NewRobotStatus(plcStatus)

	// Create robot data
	robotData := models.NewRobotData(topic, "status_update", robotStatus)
	h.logger.Debugf("📊 ROBOT DATA CREATED - MessageID: %s", robotData.MessageID)

	// Convert to JSON
	jsonData, err := robotData.ToJSON()
	if err != nil {
		h.logger.Errorf("❌ STATUS JSON FAILED - Error: %v", err)
		return nil, "", err
	}

	// Generate target topic
	targetTopic := h.config.GetRobotTopic(topic)

	h.logger.Infof("✅ STATUS CONVERSION SUCCESS - %s → %s", topic, targetTopic)
	h.logger.Debugf("✅ STATUS OUTPUT - %s", string(jsonData))

	return jsonData, targetTopic, nil
}

// CommandHandler handles PLC command topics
type CommandHandler struct {
	config *config.Config
	logger *utils.Logger
}

func NewCommandHandler(cfg *config.Config, logger *utils.Logger) *CommandHandler {
	return &CommandHandler{
		config: cfg,
		logger: logger,
	}
}

func (h *CommandHandler) CanHandle(topic string) bool {
	return strings.HasPrefix(topic, h.config.PLCTopicPrefix) && strings.HasSuffix(topic, "command")
}

func (h *CommandHandler) Convert(topic string, payload []byte) ([]byte, string, error) {
	h.logger.Infof("⚡ COMMAND HANDLER - Converting data from topic: %s", topic)
	h.logger.Debugf("⚡ COMMAND INPUT - %s", string(payload))

	// Parse PLC command
	plcCommand, err := models.ParsePLCCommand(payload)
	if err != nil {
		h.logger.Errorf("❌ COMMAND PARSE FAILED - Error: %v", err)
		return nil, "", err
	}

	h.logger.Infof("⚡ COMMAND PARSED - ID: %s, Action: %s, Params: %v",
		plcCommand.CommandID, plcCommand.Action, plcCommand.Params)

	// Convert to robot command
	robotCommand := models.NewRobotCommand(plcCommand)

	// Create robot data
	robotData := models.NewRobotData(topic, "command_execute", robotCommand)
	h.logger.Debugf("⚡ ROBOT DATA CREATED - MessageID: %s", robotData.MessageID)

	// Convert to JSON
	jsonData, err := robotData.ToJSON()
	if err != nil {
		h.logger.Errorf("❌ COMMAND JSON FAILED - Error: %v", err)
		return nil, "", err
	}

	// Generate target topic
	targetTopic := h.config.GetRobotTopic(topic)

	h.logger.Infof("✅ COMMAND CONVERSION SUCCESS - %s → %s", topic, targetTopic)
	h.logger.Debugf("✅ COMMAND OUTPUT - %s", string(jsonData))

	return jsonData, targetTopic, nil
}

// DataHandler handles generic PLC data topics
type DataHandler struct {
	config *config.Config
	logger *utils.Logger
}

func NewDataHandler(cfg *config.Config, logger *utils.Logger) *DataHandler {
	return &DataHandler{
		config: cfg,
		logger: logger,
	}
}

func (h *DataHandler) CanHandle(topic string) bool {
	return strings.HasPrefix(topic, h.config.PLCTopicPrefix) && strings.HasSuffix(topic, "data")
}

func (h *DataHandler) Convert(topic string, payload []byte) ([]byte, string, error) {
	h.logger.Infof("📦 DATA HANDLER - Converting generic data from topic: %s", topic)
	h.logger.Debugf("📦 DATA INPUT - %s", string(payload))

	// Parse PLC data
	plcData, err := models.ParsePLCData(topic, payload)
	if err != nil {
		h.logger.Errorf("❌ DATA PARSE FAILED - Error: %v", err)
		return nil, "", err
	}

	h.logger.Infof("📦 DATA PARSED - Type: %T", plcData.Data)
	h.logger.Debugf("📦 DATA CONTENT - %+v", plcData.Data)

	// Create robot data
	robotData := models.NewRobotData(topic, "data_update", plcData.Data)
	h.logger.Debugf("📦 ROBOT DATA CREATED - MessageID: %s", robotData.MessageID)

	// Convert to JSON
	jsonData, err := robotData.ToJSON()
	if err != nil {
		h.logger.Errorf("❌ DATA JSON FAILED - Error: %v", err)
		return nil, "", err
	}

	// Generate target topic
	targetTopic := h.config.GetRobotTopic(topic)

	h.logger.Infof("✅ DATA CONVERSION SUCCESS - %s → %s", topic, targetTopic)
	h.logger.Debugf("✅ DATA OUTPUT - %s", string(jsonData))

	return jsonData, targetTopic, nil
}
