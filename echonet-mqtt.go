package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"echonet-mqtt/echonet"
)

type Config struct {
	Broker     string           `json:"broker"`
	ObjectList []echonet.Config `json:"list"`
}

func main() {
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

	err = echonet_mqtt(cfg)
	if err != nil {
		log.Fatalf("%s", err)
	}

	os.Exit(0)
}

func echonet_mqtt(cfg Config) error {
	enet, err := echonet.NewEchonet()
	if err != nil {
		return err
	}

	for _, c := range cfg.ObjectList {
		obj, err := enet.NewObject(c)
		if err != nil {
			return err
		}
		log.Printf("added %s:%06x %s %s\n", c.Addr,
			obj.GetEoj(), obj.GetType(), obj.GetName())
	}

	err = enet.Start()
	if err != nil {
		return err
	}

	if false {
		enet.SendAnnounce()
		time.Sleep(3 * time.Second)
	}

	mqtt, err := NewMqtt(cfg.Broker)
	if err != nil {
		return err
	}

	// status update
	go func() {
		for {
			err = enet.StateAll()
			if err != nil {
				log.Fatalf("%s", err)
			}
			time.Sleep(300 * time.Second)
		}
	}()

	for _, obj := range enet.List() {
		topic := fmt.Sprintf("%s/%s", obj.GetType(), obj.GetName())
		switch obj.GetType() {
		case "light":
			mqtt.Subscribe(topic + "/power/set")
		case "aircon":
			mqtt.Subscribe(topic + "/mode/set")
			mqtt.Subscribe(topic + "/temperature/set")
			mqtt.Subscribe(topic + "/humidity/set")
			mqtt.Subscribe(topic + "/fan/set")
			mqtt.Subscribe(topic + "/swing/set")
		}
	}

	var update_nodes []*echonet.EchonetObject

	for {
		select {

		case obj := <-enet.RecvChan:
			//log.Printf("recv_echonet: %+v\n", node)
			topic := fmt.Sprintf("%s/%s", obj.GetType(), obj.GetName())
			switch obj.GetType() {

			case "light":
				mqtt.Send(topic+"/power", obj.GetPower())

			case "aircon":
				mqtt.Send(topic+"/mode", obj.GetMode())
				mqtt.Send(topic+"/temperature",
					strconv.Itoa(obj.GetTargetTemp()))
				mqtt.Send("sensor/"+topic+"/temperature",
					strconv.Itoa(obj.GetRoomTemp()))
				mqtt.Send("sensor/"+topic+"/outtemp",
					strconv.Itoa(obj.GetOutdoorTemp()))
				mqtt.Send(topic+"/humidity",
					strconv.Itoa(obj.GetTargetHumidity()))
				mqtt.Send("sensor/"+topic+"/humidity",
					strconv.Itoa(obj.GetRoomHumidfy()))
				mqtt.Send(topic+"/fan", obj.GetFan())
				mqtt.Send(topic+"/swing", obj.GetSwing())
			}

		case msg := <-recv_mqtt:
			log.Printf("recv_mqtt: %+v\n", msg)
			topic := strings.Split(msg[0], "/")
			payload := msg[1]

			node := enet.FindObject(topic[0], topic[1])
			if node == nil {
				return fmt.Errorf("invalid topic: %s", msg[0])
			}

			switch topic[0] {

			case "light":
				switch topic[2] {
				case "power":
					node.SetPower(payload)
				default:
					return fmt.Errorf("invalid topic: %s", msg[0])
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
				case "fan":
					node.SetFan(payload)
				case "swing":
					node.SetSwing(payload)
				default:
					return fmt.Errorf("invalid topic: %s", msg[0])
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
