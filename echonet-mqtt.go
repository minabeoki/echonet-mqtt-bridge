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
	Broker string `json:"broker"`
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
		log.Fatalf("%s", err)
	}
	log.Printf("config: %+v\n", cfg)

	mqtt, err := NewMqtt(cfg)
	if err != nil {
		log.Fatalf("%s", err)
	}

	echonet := NewEchonet()
	err = echonet.StartReceiver()
	if err != nil {
		panic(err)
	}

	node, err := echonet.NewNodeAircon("aircon-living")
	if err != nil {
		panic(err)
	}

	fmt.Println("get state")
	//node.Property()
	node.State()
	time.Sleep(3 * time.Second)

	if false {
		fmt.Println("power on")
		//node.SetMode("auto")
		node.SetMode("cool")

		time.Sleep(60 * time.Second)

		fmt.Println("get state")
		node.State()

		time.Sleep(60 * time.Second)

		fmt.Println("power off")
		node.SetMode("off")
		time.Sleep(3 * time.Second)

		fmt.Println("get state")
		node.State()
		time.Sleep(3 * time.Second)
	}

	for {
		select {

		case dev := <-recv_echonet:
			fmt.Printf("recv: %+v\n", dev)
			mqtt.Send("aircon/livingroom/mode", dev.GetMode())
			mqtt.Send("aircon/livingroom/temperature",
				strconv.Itoa(dev.GetTargetTemp()))
			mqtt.Send("sensor/aircon/livingroom/temperature",
				strconv.Itoa(dev.GetRoomTemp()))

		case msg := <-recv_mqtt:
			topic := strings.Split(msg[0], "/")
			payload := msg[1]
			log.Printf("recv: %+v: %s\n", topic, payload)

			switch topic[2] {
			case "mode":
				node.SetMode(payload)
			case "temperature":
				temp, _ := strconv.Atoi(payload)
				node.SetTargetTemp(temp)
			}

		case <-time.After(60 * time.Second):
		}

		node.State()
		time.Sleep(1 * time.Second)
	}
}

func readConfig(fn string) (Config, error) {
	var cfg Config

	fp, err := os.Open(fn)
	if err != nil {
		return cfg, err
	}

	jsondec := json.NewDecoder(fp)
	jsondec.Decode(&cfg)

	return cfg, nil
}
