package converter

import (
	"fmt"
	"time"

	"topic-data-converter/config"
	"topic-data-converter/models"
	"topic-data-converter/utils"
)

type Converter struct {
	config   *config.Config
	logger   *utils.Logger
	handlers map[string]TopicHandler
	metrics  *ConversionMetrics
}

type TopicHandler interface {
	Convert(topic string, payload []byte) ([]byte, string, error)
	CanHandle(topic string) bool
}

// ConversionMetrics tracks conversion statistics
type ConversionMetrics struct {
	TotalConversions int64
	TotalErrors      int64
	TotalDuration    time.Duration
	LastConversion   time.Time
}

func NewConverter(cfg *config.Config, logger *utils.Logger) *Converter {
	c := &Converter{
		config:   cfg,
		logger:   logger,
		handlers: make(map[string]TopicHandler),
		metrics:  &ConversionMetrics{},
	}

	// Register default handlers
	c.RegisterHandler("status", NewStatusHandler(cfg, logger))
	c.RegisterHandler("command", NewCommandHandler(cfg, logger))
	c.RegisterHandler("data", NewDataHandler(cfg, logger))

	return c
}

func (c *Converter) RegisterHandler(name string, handler TopicHandler) {
	c.handlers[name] = handler
	c.logger.Infof("Registered handler: %s", name)
}

func (c *Converter) Convert(topic string, payload []byte) ([]byte, string, error) {
	startTime := time.Now()

	c.logger.Infof("🔄 CONVERSION START - Topic: %s", topic)
	c.logger.Debugf("🔄 CONVERSION START - Payload size: %d bytes", len(payload))

	// Find appropriate handler
	for name, handler := range c.handlers {
		if handler.CanHandle(topic) {
			c.logger.Infof("🎯 HANDLER FOUND - Using handler: %s for topic: %s", name, topic)

			convertedData, targetTopic, err := handler.Convert(topic, payload)
			duration := time.Since(startTime)

			if err != nil {
				c.logger.Errorf("❌ HANDLER FAILED - Handler: %s, Error: %v", name, err)
				c.recordMetrics(duration, false)
				return nil, "", err
			}

			c.logger.Infof("✅ HANDLER SUCCESS - Handler: %s converted %s → %s (took %v)", name, topic, targetTopic, duration)
			c.recordMetrics(duration, true)
			return convertedData, targetTopic, nil
		}
	}

	// Default conversion if no specific handler found
	c.logger.Warnf("⚠️  NO HANDLER FOUND - Using default conversion for topic: %s", topic)

	convertedData, targetTopic, err := c.defaultConvert(topic, payload)
	duration := time.Since(startTime)

	if err != nil {
		c.recordMetrics(duration, false)
	} else {
		c.recordMetrics(duration, true)
	}

	return convertedData, targetTopic, err
}

func (c *Converter) defaultConvert(topic string, payload []byte) ([]byte, string, error) {
	c.logger.Infof("🔧 DEFAULT CONVERSION - Topic: %s", topic)
	c.logger.Debugf("🔧 DEFAULT CONVERSION - Input: %s", string(payload))

	// Parse PLC data
	plcData, err := models.ParsePLCData(topic, payload)
	if err != nil {
		c.logger.Errorf("❌ PARSE FAILED - Topic: %s, Error: %v", topic, err)
		return nil, "", fmt.Errorf("failed to parse PLC data: %w", err)
	}

	c.logger.Debugf("🔧 PARSED DATA - Type: %T, Content: %+v", plcData.Data, plcData.Data)

	// Create robot data
	robotData := models.NewRobotData(topic, "data_update", plcData.Data)
	c.logger.Debugf("🔧 ROBOT DATA CREATED - MessageID: %s", robotData.MessageID)

	// Convert to JSON
	jsonData, err := robotData.ToJSON()
	if err != nil {
		c.logger.Errorf("❌ JSON CONVERSION FAILED - Error: %v", err)
		return nil, "", fmt.Errorf("failed to convert to JSON: %w", err)
	}

	// Generate target topic
	targetTopic := c.config.GetRobotTopic(topic)

	c.logger.Infof("✅ DEFAULT CONVERSION SUCCESS - %s → %s", topic, targetTopic)
	c.logger.Debugf("✅ OUTPUT JSON - %s", string(jsonData))

	return jsonData, targetTopic, nil
}

func (c *Converter) recordMetrics(duration time.Duration, success bool) {
	if success {
		c.metrics.TotalConversions++
	} else {
		c.metrics.TotalErrors++
	}
	c.metrics.TotalDuration += duration
	c.metrics.LastConversion = time.Now()

	// Log metrics every 10 conversions
	total := c.metrics.TotalConversions + c.metrics.TotalErrors
	if total%10 == 0 {
		c.LogMetrics()
	}
}

func (c *Converter) LogMetrics() {
	total := c.metrics.TotalConversions + c.metrics.TotalErrors
	if total == 0 {
		return
	}

	avgDuration := time.Duration(int64(c.metrics.TotalDuration) / total)
	successRate := float64(c.metrics.TotalConversions) / float64(total) * 100

	c.logger.Infof("📈 CONVERSION METRICS")
	c.logger.Infof("├── Total Operations: %d", total)
	c.logger.Infof("├── Successful: %d (%.1f%%)", c.metrics.TotalConversions, successRate)
	c.logger.Infof("├── Failed: %d", c.metrics.TotalErrors)
	c.logger.Infof("├── Average Duration: %v", avgDuration)
	c.logger.Infof("└── Last Conversion: %s", c.metrics.LastConversion.Format("15:04:05"))
}

func (c *Converter) GetMetrics() *ConversionMetrics {
	return c.metrics
}
