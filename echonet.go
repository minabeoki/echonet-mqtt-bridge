package main

import (
	"fmt"
	"net"
)

const (
	ECHONET_PORT      = 3610
	ECHONET_NODE_SELF = 0x0ef001
)

type EchonetNode struct {
	addr         *net.UDPAddr
	conn         net.Conn
	power        bool
	mode         string
	target_temp  int
	room_temp    int
	outdoor_temp int
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
	}
	en.Nodes = append(en.Nodes, node)

	return &node, nil
}

func (node *EchonetNode) PowerOff() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_NODE_SELF)
	pkt.SetDeoj(0x013001)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty1(0x80, 0x31)

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
}

func (node *EchonetNode) PowerOnAuto() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_NODE_SELF)
	pkt.SetDeoj(0x013001)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty1(0x80, 0x30)
	pkt.AddProperty1(0xb0, 0x41)

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
}

func (node *EchonetNode) PowerOnCool() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_NODE_SELF)
	pkt.SetDeoj(0x013001)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty1(0x80, 0x30)
	pkt.AddProperty1(0xb0, 0x42)

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
}

func (node *EchonetNode) PowerOnHeat() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_NODE_SELF)
	pkt.SetDeoj(0x013001)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty1(0x80, 0x30)
	pkt.AddProperty1(0xb0, 0x43)

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
}

func (node *EchonetNode) State() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_NODE_SELF)
	pkt.SetDeoj(0x013001)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty0(0x80) // power
	pkt.AddProperty0(0xb0) // mode
	pkt.AddProperty0(0xb3) // target temp
	pkt.AddProperty0(0xbb) // room temp
	pkt.AddProperty0(0xbe) // outdoor temp

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
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
					node.mode = "dehumidify"
				case 0x45:
					node.mode = "fan"
				case 0x46:
					node.mode = "other"
				}
			case EPC_TARGET_TEMP:
				node.target_temp = int(prop.EDT[0])
			case EPC_ROOM_TEMP:
				node.room_temp = int(prop.EDT[0])
			case EPC_OUTDOOR_TEMP:
				node.outdoor_temp = int(prop.EDT[0])
			}
		}
		if node.power == false {
			node.mode = "off"
			node.target_temp = node.room_temp
		}

		fmt.Printf("Handler: %+v\n", node)
	}
}
