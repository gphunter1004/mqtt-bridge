package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func NewClient(cfg *config.Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.MQTTBroker)
	opts.SetClientID(cfg.MQTTClientID)
	opts.SetUsername(cfg.MQTTUsername)
	opts.SetPassword(cfg.MQTTPassword)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)

	// 연결 상태 콜백
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		utils.Logger.Info("MQTT client connected")
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		utils.Logger.Errorf("MQTT connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)

	// 연결 시도
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	return client, nil
}
