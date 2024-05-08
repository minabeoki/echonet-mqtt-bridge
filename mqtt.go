/// mqtt.go ---

package main

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type MqttClient struct {
	client MQTT.Client
}

var (
	recv_mqtt = make(chan [2]string, 32)
)

func NewMqtt(broker string) (*MqttClient, error) {
	mqtt := &MqttClient{}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetAutoReconnect(true)
	opts.SetDefaultPublishHandler(
		func(c MQTT.Client, msg MQTT.Message) {
			recv_mqtt <- [2]string{msg.Topic(), string(msg.Payload())}
		})

	mqtt.client = MQTT.NewClient(opts)
	if t := mqtt.client.Connect(); t.Wait() && t.Error() != nil {
		return nil, t.Error()
	}

	return mqtt, nil
}

func (mqtt *MqttClient) Subscribe(topic string) error {
	if t := mqtt.client.Subscribe(topic, 0, nil); t.Wait() && t.Error() != nil {
		return t.Error()
	}
	return nil
}

func (mqtt *MqttClient) Send(topic string, payload string) {
	t := mqtt.client.Publish(topic, 0, true, payload)
	t.Wait()
}
