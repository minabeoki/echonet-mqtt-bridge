/*
ECHONET Lite
*/
package echonet

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	ECHONET_PORT      = 3610
	ECHONET_EOJ_NODE  = 0x0ef001
	ECHONET_MULTICAST = "224.0.23.0"
)

var (
	send_mutex sync.Mutex
)

// Echonet node
type EchonetNode struct {
	parent          *Echonet
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
	cfg             Object
}

// Echonet object informations
type Object struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Eoj  string `json:"eoj"`
}

// Echonet
type Echonet struct {
	Nodes      []*EchonetNode
	RecvChan   chan *EchonetNode
	mconn_send *net.UDPConn
	mconn_recv *ipv4.PacketConn
}

func NewEchonet() (*Echonet, error) {
	udpAddr, err := net.ResolveUDPAddr("udp",
		net.JoinHostPort(ECHONET_MULTICAST, strconv.Itoa(ECHONET_PORT)))
	if err != nil {
		return nil, err
	}

	conn_send, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	conn_recv, err := multicastSocket(ECHONET_MULTICAST, ECHONET_PORT)
	if err != nil {
		return nil, err
	}

	return &Echonet{
		RecvChan:   make(chan *EchonetNode, 32),
		mconn_send: conn_send,
		mconn_recv: conn_recv,
	}, nil
}

func (en *Echonet) SendAnnounce() {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(ECHONET_EOJ_NODE)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty(EPC_NODE_INS_LIST) // instalce list

	_, err := en.mconn_send.Write(pkt.Bytes())
	if err != nil {
		panic(err)
	}
	time.Sleep(250 * time.Millisecond)
}

func (en *Echonet) receiver(conn *ipv4.PacketConn) {
	defer conn.Close()

	for {
		buf := make([]byte, 1500)
		length, cm, addr, err := conn.ReadFrom(buf)
		if err != nil {
			panic(err)
		}

		recv_pkt := NewEchonetPacket()
		recv_pkt.Parse(buf[:length])
		src, _, _ := net.SplitHostPort(addr.String())
		log.Printf("Recv: %+v => %+v %s\n", src, cm.Dst, recv_pkt.String())

		for _, node := range en.Nodes {
			if node.addr.IP.String() == src &&
				node.GetEoj() == recv_pkt.GetSeoj() {
				node.Handler(recv_pkt)
			}
		}
	}
}

func (en *Echonet) Start() error {
	go en.receiver(en.mconn_recv)

	return nil
}

func (en *Echonet) NewNode(cfg Object) (*EchonetNode, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4",
		net.JoinHostPort(cfg.Addr, strconv.Itoa(ECHONET_PORT)))
	if err != nil {
		return nil, err
	}

	conn_send, err := net.DialUDP("udp4", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	eoj, err := strconv.ParseUint(cfg.Eoj, 16, 24)
	if err != nil {
		return nil, err
	}

	node := EchonetNode{
		parent: en,
		addr:   udpAddr,
		conn:   conn_send,
		tid:    1,
		eoj:    uint32(eoj),
		cfg:    cfg,
	}
	en.Nodes = append(en.Nodes, &node)

	return &node, nil
}

func (en *Echonet) FindNode(objtype, objname string) *EchonetNode {
	for _, node := range en.Nodes {
		if node.cfg.Type == objtype && node.cfg.Name == objname {
			return node
		}
	}
	return nil
}

func (en *Echonet) StateAll() error {
	for _, node := range en.Nodes {
		err := node.State()
		if err != nil {
			return err
		}
	}
	return nil
}

func (en *Echonet) NodeList() []*EchonetNode {
	return en.Nodes
}

func (node *EchonetNode) GetEoj() uint32 {
	return node.eoj
}

func (node *EchonetNode) GetType() string {
	return node.cfg.Type
}

func (node *EchonetNode) GetName() string {
	return node.cfg.Name
}

func (node *EchonetNode) sendPacket(pkt *EchonetPacket) error {
	pkt.SetTid(node.tid)
	node.tid += 1

	send_mutex.Lock()

	_, err := node.conn.Write(pkt.Bytes())
	if err != nil {
		return fmt.Errorf("send failed: %s", err)
	}
	dst, _, _ := net.SplitHostPort(node.conn.RemoteAddr().String())
	log.Printf("Send: %s %s\n", dst, pkt.String())
	time.Sleep(700 * time.Millisecond)

	send_mutex.Unlock()
	return nil
}

func (node *EchonetNode) SetPower(pow string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)
	if pow == "on" {
		pkt.AddProperty(EPC_POWER, EDT_ON) // power on
	} else {
		pkt.AddProperty(EPC_POWER, EDT_OFF) // power off
	}
	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetPower() string {
	if node.power {
		return "on"
	}
	return "off"
}

func (node *EchonetNode) SetMode(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)

	switch mode {
	case "off":
		pkt.AddProperty(EPC_POWER, EDT_OFF) // power off
	case "auto":
		pkt.AddProperty(EPC_POWER, EDT_ON)  // power on
		pkt.AddProperty(EPC_MODE, EDT_AUTO) // auto mode
	case "cool":
		pkt.AddProperty(EPC_POWER, EDT_ON) // power on
		pkt.AddProperty(EPC_MODE, 0x42)    // cool mode
	case "heat":
		pkt.AddProperty(EPC_POWER, EDT_ON)            // power on
		pkt.AddProperty(EPC_MODE, 0x43)               // heat mode
		pkt.AddProperty(EPC_HUMIDIFY, EDT_AUTO)       // humidification on
		pkt.AddProperty(EPC_HUMIDIFY_LEVEL, EDT_AUTO) // humidification auto
	case "dry":
		pkt.AddProperty(EPC_POWER, EDT_ON) // power on
		pkt.AddProperty(EPC_MODE, 0x44)    // dry mode
	case "fan":
		pkt.AddProperty(EPC_POWER, EDT_ON) // power on
		pkt.AddProperty(EPC_MODE, 0x45)    // fan mode
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetMode() (mode string) {
	if node.power {
		return node.mode
	}
	return "off"
}

func (node *EchonetNode) SetFan(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)

	switch mode {
	case "auto":
		pkt.AddProperty(EPC_FAN, EDT_AUTO)
	case "low":
		pkt.AddProperty(EPC_FAN, 0x31)
	case "medium":
		pkt.AddProperty(EPC_FAN, 0x33)
	case "high":
		pkt.AddProperty(EPC_FAN, 0x35)
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetFan() (mode string) {
	switch node.fan {
	case EDT_AUTO:
		return "auto"
	case 0x31, 0x32:
		return "low"
	case 0x33, 0x34:
		return "medium"
	case 0x35, 0x36, 0x37, 0x38:
		return "high"
	}
	return "auto"
}

func (node *EchonetNode) SetSwing(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)

	switch mode {
	case "off":
		pkt.AddProperty(EPC_SWING, EDT_OFF)
	case "ud":
		pkt.AddProperty(EPC_FAN, 0x41) // up and down
	case "lr":
		pkt.AddProperty(EPC_FAN, 0x42) // left and right
	case "on":
		pkt.AddProperty(EPC_FAN, 0x43)
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetSwing() (mode string) {
	switch node.fan {
	case EDT_OFF:
		return "off"
	case 0x41:
		return "ud"
	case 0x42:
		return "lr"
	case 0x43:
		return "on"
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
	pkt.AddProperty(EPC_TARGET_TEMP, byte(temp))
	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetTargetTemp() (temp int) {
	if node.target_temp == 0xfd {
		// In case of 0xfd, the target temperature is auto.
		return node.room_temp
	}
	return node.target_temp
}

func (node *EchonetNode) SetTargetHumidity(humi int) error {
	if humi < 0 || humi > 100 {
		return fmt.Errorf("invalid humidity %d", humi)
	}

	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty(EPC_TARGET_HUMIDITY, byte(humi))
	return node.sendPacket(pkt)
}

func (node *EchonetNode) GetTargetHumidity() (temp int) {
	return node.target_humidity
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

func (node *EchonetNode) Property() error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty(EPC_INF_PROPMAP) // announce map
	pkt.AddProperty(EPC_SET_PROPMAP) // set map
	pkt.AddProperty(EPC_GET_PROPMAP) // get map
	return node.sendPacket(pkt)
}

func (node *EchonetNode) State() error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(node.eoj)
	pkt.SetEsv(ESV_GET)

	switch node.cfg.Type {
	case "aircon":
		pkt.AddProperty(EPC_POWER)           // power
		pkt.AddProperty(EPC_MODE)            // mode
		pkt.AddProperty(EPC_TARGET_TEMP)     // target temp
		pkt.AddProperty(EPC_TARGET_HUMIDITY) // target humidity
		pkt.AddProperty(EPC_ROOM_HUMIDITY)   // room humidity
		pkt.AddProperty(EPC_ROOM_TEMP)       // room temp
		pkt.AddProperty(EPC_OUTDOOR_TEMP)    // outdoor temp
		pkt.AddProperty(EPC_FAN)             // fan
		pkt.AddProperty(EPC_SWING)           // swing
	case "light":
		pkt.AddProperty(EPC_POWER) // power
	default:
		return fmt.Errorf("invalid type: %s", node.cfg.Type)
	}

	return node.sendPacket(pkt)
}

func (node *EchonetNode) Handler(pkt *EchonetPacket) {
	if pkt.ESV == ESV_GET_RES || pkt.ESV == ESV_SETGET_RES ||
		pkt.ESV == ESV_INF {
		for _, prop := range pkt.Props {
			switch prop.EPC {
			case EPC_POWER:
				node.power = prop.EDT[0] == EDT_ON
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

		node.parent.RecvChan <- node
		//log.Printf("Handler: %+v\n", node)
	}
}
