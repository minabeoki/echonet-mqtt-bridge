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
		log.Printf("added %s:%06x %s %s\n", obj.Addr,
			node.GetEoj(), node.GetType(), node.GetName())
	}

	err = echonet.StartReceiver()
	if err != nil {
		log.Fatalf("%s", err)
	}

	if false {
		echonet.SendAnnounce()
		time.Sleep(3 * time.Second)
	}

	mqtt, err := NewMqtt(cfg.Broker)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// status update
	go func() {
		for {
			err = echonet.StateAll()
			if err != nil {
				log.Fatalf("%s", err)
			}
			time.Sleep(300 * time.Second)
		}
	}()

	for _, node := range echonet.NodeList() {
		topic := fmt.Sprintf("%s/%s", node.GetType(), node.GetName())
		switch node.GetType() {
		case "light":
			mqtt.Subscribe(topic + "/power/set")
		case "aircon":
			mqtt.Subscribe(topic + "/mode/set")
			mqtt.Subscribe(topic + "/temperature/set")
			mqtt.Subscribe(topic + "/humidity/set")
		}
	}

	var update_nodes []*EchonetNode

	for {
		select {

		case node := <-recv_echonet:
			//log.Printf("recv_echonet: %+v\n", node)
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
				mqtt.Send("sensor/"+topic+"/outtemp",
					strconv.Itoa(node.GetOutdoorTemp()))
				mqtt.Send(topic+"/humidity",
					strconv.Itoa(node.GetTargetHumidity()))
				mqtt.Send("sensor/"+topic+"/humidity",
					strconv.Itoa(node.GetRoomHumidfy()))

			}

		case msg := <-recv_mqtt:
			log.Printf("recv_mqtt: %+v\n", msg)
			topic := strings.Split(msg[0], "/")
			payload := msg[1]

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
				//update_nodes = append(update_nodes, node)

			case "aircon":
				switch topic[2] {
				case "mode":
					node.SetMode(payload)
				case "temperature":
					temp, _ := strconv.ParseFloat(payload, 32)
					node.SetTargetTemp(int(temp))
				case "humidity":
					humi, _ := strconv.ParseFloat(payload, 32)
					node.SetTargetHumidity(int(humi))
				default:
					log.Fatalf("invalid topic: %s", msg[0])
				}
				update_nodes = append(update_nodes, node)
			}

		case <-time.After(1 * time.Second):
			for _, node := range update_nodes {
				node.State()
			}
			update_nodes = nil
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
