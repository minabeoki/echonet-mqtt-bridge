/// mqtt.go ---

package main

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type MqttClient struct {
	client MQTT.Client
}

var (
	recv_mqtt = make(chan [2]string, 8)
)

func makeTopics() (map[string]byte, error) {
	topics := make(map[string]byte)
	topics["aircon/livingroom/mode/set"] = byte(0)
	topics["aircon/livingroom/temperature/set"] = byte(0)
	return topics, nil
}

func NewMqtt(cfg Config) (*MqttClient, error) {
	mqtt := &MqttClient{}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetAutoReconnect(true)
	opts.SetDefaultPublishHandler(
		func(c MQTT.Client, msg MQTT.Message) {
			recv_mqtt <- [2]string{msg.Topic(), string(msg.Payload())}
		})

	mqtt.client = MQTT.NewClient(opts)
	if t := mqtt.client.Connect(); t.Wait() && t.Error() != nil {
		return nil, t.Error()
	}

	topics, err := makeTopics()
	if err != nil {
		return nil, err
	}

	if t := mqtt.client.SubscribeMultiple(topics, nil); t.Wait() && t.Error() != nil {
		return nil, t.Error()
	}

	return mqtt, nil
}

func (mqtt *MqttClient) Send(topic string, payload string) {
	//fmt.Printf("send %s: %s\n", topic, payload)
	token := mqtt.client.Publish(topic, 0, true, payload)
	token.Wait()
}
