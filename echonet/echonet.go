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
	ECHONET_PORT      = 3610         // ECHONET port number
	ECHONET_EOJ_NODE  = 0x0ef001     // ECHONET node object code
	ECHONET_MULTICAST = "224.0.23.0" // ECHONET multicast address
)

var (
	send_mutex sync.Mutex
)

// Echonet object
type EchonetObject struct {
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
	watt            int
	tid             uint16
	eoj             uint32
	cfg             Config
}

// Echonet object config
type Config struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Eoj  string `json:"eoj"`
}

// Echonet
type Echonet struct {
	ObjectList []*EchonetObject
	RecvChan   chan *EchonetObject
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
		RecvChan:   make(chan *EchonetObject, 32),
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

		for _, obj := range en.ObjectList {
			if obj.addr.IP.String() == src &&
				obj.GetEoj() == recv_pkt.GetSeoj() {
				obj.Handler(recv_pkt)
			}
		}
	}
}

func (en *Echonet) Start() error {
	go en.receiver(en.mconn_recv)

	return nil
}

func (en *Echonet) NewObject(cfg Config) (*EchonetObject, error) {
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

	obj := EchonetObject{
		parent: en,
		addr:   udpAddr,
		conn:   conn_send,
		tid:    1,
		eoj:    uint32(eoj),
		cfg:    cfg,
	}
	en.ObjectList = append(en.ObjectList, &obj)

	return &obj, nil
}

func (en *Echonet) FindObject(objtype, objname string) *EchonetObject {
	for _, obj := range en.ObjectList {
		if obj.cfg.Type == objtype && obj.cfg.Name == objname {
			return obj
		}
	}
	return nil
}

func (en *Echonet) StateAll() error {
	for _, obj := range en.ObjectList {
		err := obj.State()
		if err != nil {
			return err
		}
	}
	return nil
}

func (en *Echonet) List() []*EchonetObject {
	return en.ObjectList
}

func (obj *EchonetObject) GetEoj() uint32 {
	return obj.eoj
}

func (obj *EchonetObject) GetType() string {
	return obj.cfg.Type
}

func (obj *EchonetObject) GetName() string {
	return obj.cfg.Name
}

func (obj *EchonetObject) sendPacket(pkt *EchonetPacket) error {
	pkt.SetTid(obj.tid)
	obj.tid += 1

	send_mutex.Lock()

	_, err := obj.conn.Write(pkt.Bytes())
	if err != nil {
		return fmt.Errorf("send failed: %s", err)
	}
	dst, _, _ := net.SplitHostPort(obj.conn.RemoteAddr().String())
	log.Printf("Send: %s %s\n", dst, pkt.String())
	time.Sleep(800 * time.Millisecond)

	send_mutex.Unlock()
	return nil
}

func (obj *EchonetObject) SetPower(pow string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
	pkt.SetEsv(ESV_SETI)
	if pow == "on" {
		pkt.AddProperty(EPC_POWER, EDT_ON) // power on
	} else {
		pkt.AddProperty(EPC_POWER, EDT_OFF) // power off
	}
	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetPower() string {
	if obj.power {
		return "on"
	}
	return "off"
}

func (obj *EchonetObject) SetMode(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
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

	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetMode() (mode string) {
	if obj.power {
		return obj.mode
	}
	return "off"
}

func (obj *EchonetObject) SetFan(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
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

	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetFan() (mode string) {
	switch obj.fan {
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

func (obj *EchonetObject) SetSwing(mode string) error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
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

	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetSwing() (mode string) {
	switch obj.fan {
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

func (obj *EchonetObject) SetTargetTemp(temp int) error {
	if obj.target_temp == 0xfd {
		// In case of 0xfd, the target temperature is auto.
		return nil // ignore setting
	}
	if temp < 0 || temp > 50 {
		return fmt.Errorf("invalid temperature %d", temp)
	}

	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty(EPC_TARGET_TEMP, byte(temp))
	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetTargetTemp() (temp int) {
	if obj.target_temp == 0xfd {
		// In case of 0xfd, the target temperature is auto.
		return obj.room_temp
	}
	return obj.target_temp
}

func (obj *EchonetObject) SetTargetHumidity(humi int) error {
	if humi < 0 || humi > 100 {
		return fmt.Errorf("invalid humidity %d", humi)
	}

	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
	pkt.SetEsv(ESV_SETI)
	pkt.AddProperty(EPC_TARGET_HUMIDITY, byte(humi))
	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) GetTargetHumidity() (temp int) {
	return obj.target_humidity
}

func (obj *EchonetObject) GetRoomTemp() int {
	return obj.room_temp
}

func (obj *EchonetObject) GetOutdoorTemp() int {
	return obj.outdoor_temp
}

func (obj *EchonetObject) GetRoomHumidfy() int {
	return obj.room_humidity
}

func (obj *EchonetObject) GetWatt() int {
	return obj.watt
}

func (obj *EchonetObject) Property() error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
	pkt.SetEsv(ESV_GET)
	pkt.AddProperty(EPC_INF_PROPMAP) // announce map
	pkt.AddProperty(EPC_SET_PROPMAP) // set map
	pkt.AddProperty(EPC_GET_PROPMAP) // get map
	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) State() error {
	pkt := NewEchonetPacket()
	pkt.SetSeoj(ECHONET_EOJ_NODE)
	pkt.SetDeoj(obj.eoj)
	pkt.SetEsv(ESV_GET)

	switch obj.cfg.Type {
	case "aircon":
		pkt.AddProperty(EPC_POWER)           // power
		pkt.AddProperty(EPC_MODE)            // mode
		pkt.AddProperty(EPC_TARGET_TEMP)     // target temp
		pkt.AddProperty(EPC_TARGET_HUMIDITY) // target humidity
		pkt.AddProperty(EPC_ROOM_TEMP)       // room temp
		pkt.AddProperty(EPC_ROOM_HUMIDITY)   // room humidity
		pkt.AddProperty(EPC_OUTDOOR_TEMP)    // outdoor temp
		pkt.AddProperty(EPC_FAN)             // fan
		pkt.AddProperty(EPC_SWING)           // swing
		pkt.AddProperty(EPC_WATT)            // watt
	case "light":
		pkt.AddProperty(EPC_POWER) // power
	default:
		return fmt.Errorf("invalid type: %s", obj.cfg.Type)
	}

	return obj.sendPacket(pkt)
}

func (obj *EchonetObject) Handler(pkt *EchonetPacket) {
	if pkt.ESV == ESV_GET_RES || pkt.ESV == ESV_SETGET_RES ||
		pkt.ESV == ESV_INF {
		for _, prop := range pkt.Props {
			switch prop.EPC {
			case EPC_POWER:
				obj.power = prop.EDT[0] == EDT_ON
				// While power is off, keep the mode.
			case EPC_MODE:
				switch prop.EDT[0] {
				case 0x41:
					obj.mode = "auto"
				case 0x42:
					obj.mode = "cool"
				case 0x43:
					obj.mode = "heat"
				case 0x44:
					obj.mode = "dry"
				case 0x45:
					obj.mode = "fan"
				case 0x46:
					obj.mode = "other"
				}
			case EPC_TARGET_TEMP:
				obj.target_temp = int(prop.EDT[0])
			case EPC_ROOM_TEMP:
				obj.room_temp = int(int8(prop.EDT[0]))
				if obj.room_temp < -127 || obj.room_temp > 125 {
					// error value
					obj.room_temp = 20 // tentative
				}
			case EPC_OUTDOOR_TEMP:
				obj.outdoor_temp = int(int8(prop.EDT[0]))
				if obj.outdoor_temp < -127 || obj.outdoor_temp > 125 {
					// error value
					obj.outdoor_temp = 20 // tentative
				}
			case EPC_ROOM_HUMIDITY:
				obj.room_humidity = int(prop.EDT[0])
			case EPC_TARGET_HUMIDITY:
				obj.target_humidity = int(prop.EDT[0])
			case EPC_FAN:
				obj.fan = int(prop.EDT[0])
			case EPC_SWING:
				obj.swing = int(prop.EDT[0])
			case EPC_WATT:
				obj.watt = int(prop.EDT[0])*0x100 + int(prop.EDT[1])
			}
		}

		obj.parent.RecvChan <- obj
		//log.Printf("Handler: %+v\n", node)
	}
}
