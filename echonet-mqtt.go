package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Broker     string   `json:"broker"`
	ObjectList []Object `json:"list"`
}

type Object struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Eoj  string `json:"eoj"`
}

func main() {
	echonet_mqtt()
	os.Exit(0)
}

func echonet_mqtt() {
	log.SetFlags(log.Flags() | log.Lmicroseconds)

	if len(os.Args) != 2 {
		fmt.Println("usage: echonet2mqtt <CONFIG FILE>")
		os.Exit(0)
	}

	fn := os.Args[1]
	cfg, err := readConfig(fn)
	if err != nil {
		log.Fatalf("%s: %s", fn, err)
	}
	log.Printf("config: %+v\n", cfg)

	echonet, err := NewEchonet()
	if err != nil {
		log.Fatalf("%s", err)
	}

	for _, obj := range cfg.ObjectList {
		node, err := echonet.NewNode(obj)
		if err != nil {
			log.Fatalf("%s", err)
		}
		fmt.Printf("added: %+v\n", node)
	}

	err = echonet.StartReceiver()
	if err != nil {
		log.Fatalf("%s", err)
	}

	err = echonet.StateAll()
	if err != nil {
		log.Fatalf("%s", err)
	}
	time.Sleep(1 * time.Second)

	if false {
		//echonet.SendAnnounce()
		//time.Sleep(3 * time.Second)
	}

	mqtt, err := NewMqtt(cfg.Broker)
	if err != nil {
		log.Fatalf("%s", err)
	}

	for _, node := range echonet.NodeList() {
		topic := fmt.Sprintf("%s/%s", node.GetType(), node.GetName())
		switch node.GetType() {
		case "light":
			mqtt.Subscribe(topic + "/power/set")
		case "aircon":
			mqtt.Subscribe(topic + "/mode/set")
			mqtt.Subscribe(topic + "/temperature/set")
		}
	}

	for {
		select {

		case node := <-recv_echonet:
			fmt.Printf("recv: %+v\n", node)
			topic := fmt.Sprintf("%s/%s", node.GetType(), node.GetName())
			switch node.GetType() {

			case "light":
				mqtt.Send(topic+"/power", node.GetPower())

			case "aircon":
				mqtt.Send(topic+"/mode", node.GetMode())
				mqtt.Send(topic+"/temperature",
					strconv.Itoa(node.GetTargetTemp()))
				mqtt.Send("sensor/"+topic+"/temperature",
					strconv.Itoa(node.GetRoomTemp()))

			}

		case msg := <-recv_mqtt:
			topic := strings.Split(msg[0], "/")
			payload := msg[1]
			log.Printf("recv: %+v: %s\n", topic, payload)

			node := echonet.FindNode(topic[0], topic[1])
			if node == nil {
				log.Fatalf("invalid topic: %s", msg[0])
			}

			switch topic[0] {

			case "light":
				switch topic[2] {
				case "power":
					node.SetPower(payload)
				default:
					log.Fatalf("invalid topic: %s", msg[0])
				}
				node.State()

			case "aircon":
				switch topic[2] {
				case "mode":
					node.SetMode(payload)
				case "temperature":
					temp, _ := strconv.Atoi(payload)
					node.SetTargetTemp(temp)
				default:
					log.Fatalf("invalid topic: %s", msg[0])
				}
				node.State()
			}

		case <-time.After(60 * time.Second):
			echonet.StateAll()
		}

	}
}

func readConfig(fn string) (Config, error) {
	var cfg Config

	fp, err := os.Open(fn)
	if err != nil {
		return cfg, err
	}

	jsondec := json.NewDecoder(fp)
	if err := jsondec.Decode(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
