package main

import (
	"fmt"
	"net"
	"time"
)

const (
	ECHONET_PORT       = 3610
	ECHONET_EOJ_NODE   = 0x0ef001
	ECHONET_EOJ_AIRCON = 0x013001
)

var (
	recv_echonet = make(chan *EchonetNode, 8)
)

type EchonetNode struct {
	addr            *net.UDPAddr
	conn            net.Conn
	power           bool
	mode            string
	target_temp     int
	target_humidity int
	room_temp       int
	room_humidity   int
	outdoor_temp    int
	fan             int
	swing           int
	tid             uint16
	eoj             uint32
}

type Echonet struct {
	Nodes []EchonetNode
}

func NewEchonet() *Echonet {
	return &Echonet{}
}

func (en *Echonet) StartReceiver() error {
	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP("localhost"),
		Port: ECHONET_PORT,
	}
	conn_recv, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	go func() {
		defer conn_recv.Close()

		for {
			buf := make([]byte, 1500)
			length, addr, err := conn_recv.ReadFromUDP(buf)
			if err != nil {
				panic(err)
			}

			recv_pkt := NewEchonetPacket()
			recv_pkt.Parse(buf[:length])
			fmt.Printf("Received: %+v %s\n", addr, recv_pkt.String())

			for _, node := range en.Nodes {
				fmt.Printf("checking %+v %+v\n", node.addr.IP, addr.IP)
				if node.addr.IP.Equal(addr.IP) {
					fmt.Printf("call %+v %+v\n", node.addr, addr)
					node.Handler(recv_pkt)
				}
			}
		}
	}()

	return nil
}

func (en *Echonet) NewNodeAircon(addr string) (*EchonetNode, error) {
	addr_port := fmt.Sprintf("%s:%d", addr, ECHONET_PORT)

	conn, err := net.Dial("udp4", addr_port)
	if err != nil {
		return nil, err
	}

	udpaddr, err := net.ResolveUDPAddr("udp4", addr_port)
	if err != nil {
		return nil, err
	}

	node := EchonetNode{
		addr: udpaddr,
		conn: conn,
		tid:  1,
		eoj:  ECHONET_EOJ_AIRCON,
	}
	en.Nodes = append(en.Nodes, node)

	return &node, nil
}

func (node *EchonetNode) sendPacket(pkt *EchonetPacket) {
	pkt.SetTid(node.tid)
	node.tid += 1

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
	time.Sleep(500 * time.Millisecond)
}

func (node *EchonetNode) SetMode(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)

	switch mode {
	case "off":
		pkt.AddProperty1(EPC_POWER, 0x31) // power off
	case "auto":
		pkt.AddProperty1(EPC_POWER, 0x30) // power on
		pkt.AddProperty1(EPC_MODE, 0x41)  // auto mode
	case "cool":
		pkt.AddProperty1(EPC_POWER, 0x30) // power on
		pkt.AddProperty1(EPC_MODE, 0x42)  // cool mode
	case "heat":
		pkt.AddProperty1(EPC_POWER, 0x30) // power on
		pkt.AddProperty1(EPC_MODE, 0x43)  // heat mode
		pkt.AddProperty1(0xc1, 0x41)      // humidification on
		pkt.AddProperty1(0xc4, 0x41)      // humidification auto
	case "dry":
		pkt.AddProperty1(EPC_POWER, 0x30) // power on
		pkt.AddProperty1(EPC_MODE, 0x44)  // dry mode
	case "fan":
		pkt.AddProperty1(EPC_POWER, 0x30) // power on
		pkt.AddProperty1(EPC_MODE, 0x45)  // fan mode
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	node.sendPacket(pkt)
	return nil
}

func (node *EchonetNode) GetMode() (mode string) {
	if node.power {
		return node.mode
	}
	return "off"
}

func (node *EchonetNode) SetTargetTemp(temp int) error {
	if node.target_temp == 0xfd {
		// In case of 0xfd, the target temperature is auto.
		return nil // ignore setting
	}
	if temp < 0 || temp > 50 {
		return fmt.Errorf("invalid temperature %d", temp)
	}

	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty1(EPC_TARGET_TEMP, byte(temp))
	node.sendPacket(pkt)
	return nil
}

func (node *EchonetNode) GetTargetTemp() (temp int) {
	if node.target_temp == 0xfd {
		// In case of 0xfd, the target temperature is auto.
		return node.room_temp
	}
	return node.target_temp
}

func (node *EchonetNode) GetRoomTemp() int {
	return node.room_temp
}

func (node *EchonetNode) GetOutdoorTemp() int {
	return node.outdoor_temp
}

func (node *EchonetNode) GetRoomHumidfy() int {
	return node.room_humidity
}

func (node *EchonetNode) Property() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty0(0x83) // device identify
	pkt.AddProperty0(0x9d) // announce map
	pkt.AddProperty0(0x9e) // set map
	pkt.AddProperty0(0x9f) // get map
	node.sendPacket(pkt)
}

func (node *EchonetNode) State() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty0(0x80) // power
	pkt.AddProperty0(0xb0) // mode
	pkt.AddProperty0(0xb3) // target temp
	pkt.AddProperty0(0xb4) // target humidity
	pkt.AddProperty0(0xba) // room humidity
	pkt.AddProperty0(0xbb) // room temp
	pkt.AddProperty0(0xbe) // outdoor temp
	pkt.AddProperty0(0xa0) // fan
	pkt.AddProperty0(0xa3) // swing
	node.sendPacket(pkt)
}

func (node *EchonetNode) Handler(pkt *EchonetPacket) {
	if pkt.ESV == ESV_GET_RES {
		for _, prop := range pkt.Props {
			switch prop.EPC {
			case EPC_POWER:
				node.power = prop.EDT[0] == 0x30
			case EPC_MODE:
				switch prop.EDT[0] {
				case 0x41:
					node.mode = "auto"
				case 0x42:
					node.mode = "cool"
				case 0x43:
					node.mode = "heat"
				case 0x44:
					node.mode = "dry"
				case 0x45:
					node.mode = "fan"
				case 0x46:
					node.mode = "other"
				}
			case EPC_TARGET_TEMP:
				node.target_temp = int(prop.EDT[0])
			case EPC_ROOM_TEMP:
				node.room_temp = int(int8(prop.EDT[0]))
			case EPC_OUTDOOR_TEMP:
				node.outdoor_temp = int(int8(prop.EDT[0]))
			case EPC_ROOM_HUMIDITY:
				node.room_humidity = int(prop.EDT[0])
			case EPC_TARGET_HUMIDITY:
				node.target_humidity = int(prop.EDT[0])
			case EPC_FAN:
				node.fan = int(prop.EDT[0])
			case EPC_SWING:
				node.swing = int(prop.EDT[0])
			}
		}
		if node.power == false {
			node.mode = "off"
		}
		if node.target_temp == 0xfd {
			// In case of 0xfd, the target temperature is auto.
			node.target_temp = node.room_temp
		}

		recv_echonet <- node
		fmt.Printf("Handler: %+v\n", node)
	}
}
