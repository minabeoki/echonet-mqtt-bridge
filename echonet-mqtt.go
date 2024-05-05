package main

import (
	"fmt"
	"os"
	"time"
)

const (
	ECHONE_PORT = 3610
)

func main() {
	echonet_mqtt()
	os.Exit(0)
}

func echonet_mqtt() {
	fmt.Println("echonet_mqtt")

	echonet := NewEchonet()
	err := echonet.StartReceiver()
	if err != nil {
		panic(err)
	}

	node, err := echonet.NewNodeAircon("aircon-living")
	if err != nil {
		panic(err)
	}

	fmt.Println("get state")
	node.Property()
	node.State()
	time.Sleep(3 * time.Second)

	if true {
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
}
