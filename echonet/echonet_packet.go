package echonet

import "fmt"

const (
	ESV_SETI_SNA   = 0x50 // deny ESV=0x60
	ESV_SETC_SNA   = 0x51 // deny ESV=0x61
	ESV_GET_SNA    = 0x52 // deny ESV=0x62
	ESV_INF_SNA    = 0x53 // deny ESV=0x63
	ESV_SETGET_SNA = 0x5e // deny ESV=0x6e

	ESV_SETI       = 0x60 // set w/o response
	ESV_SETC       = 0x61 // set with response
	ESV_GET        = 0x62 // get
	ESV_INF_REQ    = 0x63
	ESV_SETGET     = 0x6e
	ESV_SET_RES    = 0x71 // response ESV=0x61
	ESV_GET_RES    = 0x72 // response ESV=0x62
	ESV_INF        = 0x73
	ESV_INFC       = 0x74
	ESV_INFC_RES   = 0x7a
	ESV_SETGET_RES = 0x7e

	EPC_POWER           = 0x80
	EPC_FAN             = 0xa0
	EPC_SWING           = 0xa3
	EPC_MODE            = 0xb0
	EPC_TARGET_TEMP     = 0xb3
	EPC_TARGET_HUMIDITY = 0xb4
	EPC_ROOM_HUMIDITY   = 0xba
	EPC_ROOM_TEMP       = 0xbb
	EPC_OUTDOOR_TEMP    = 0xbe
	EPC_HUMIDIFY        = 0xc1
	EPC_HUMIDIFY_LEVEL  = 0xc4

	EPC_INF_PROPMAP     = 0x9d
	EPC_SET_PROPMAP     = 0x9e
	EPC_GET_PROPMAP     = 0x9f
	EPC_NODE_INS_NUM    = 0xd3
	EPC_NODE_CLASS_NUM  = 0xd4
	EPC_NODE_INS_INF    = 0xd5
	EPC_NODE_INS_LIST   = 0xd6
	EPC_NODE_CLASS_LIST = 0xd7

	EDT_ON   = 0x30
	EDT_OFF  = 0x31
	EDT_AUTO = 0x41
)

type EchonetProperty struct {
	EPC byte
	PDC byte
	EDT []byte
}

type EchonetPacket struct {
	EHD   uint16
	TID   uint16
	SEOJ  uint32
	DEOJ  uint32
	ESV   byte
	OPC   byte
	Props []EchonetProperty
}

func NewEchonetPacket() *EchonetPacket {
	return &EchonetPacket{
		EHD: 0x1081,
		TID: 0x0001,
		ESV: 0x00,
		OPC: 0x00,
	}
}

func (pkt *EchonetPacket) Bytes() []byte {
	b := []byte{
		byte(pkt.EHD >> 8),
		byte(pkt.EHD & 0xff),
		byte(pkt.TID >> 8),
		byte(pkt.TID & 0xff),
		byte((pkt.SEOJ >> 16) & 0xff),
		byte((pkt.SEOJ >> 8) & 0xff),
		byte(pkt.SEOJ & 0xff),
		byte((pkt.DEOJ >> 16) & 0xff),
		byte((pkt.DEOJ >> 8) & 0xff),
		byte(pkt.DEOJ & 0xff),
		pkt.ESV,
		pkt.OPC,
	}

	for _, prop := range pkt.Props {
		b = append(b, prop.EPC)
		b = append(b, prop.PDC)
		b = append(b, prop.EDT...)
	}

	return b
}

func (pkt *EchonetPacket) String() string {
	s := fmt.Sprintf("EOJ:%06x=>%06x TID:%d ", pkt.SEOJ, pkt.DEOJ, pkt.TID)

	switch pkt.ESV {
	case ESV_SETI:
		s += "SetI"
	case ESV_SETC:
		s += "SetC"
	case ESV_GET:
		s += "Get"
	case ESV_INF_REQ:
		s += "INF_REQ"
	case ESV_SETGET:
		s += "SetGet"
	case ESV_SET_RES:
		s += "Set_Res" // response for SetC(0x61)
	case ESV_GET_RES:
		s += "Get_Res" // response for Get(0x62)
	case ESV_INF:
		s += "INF"
	case ESV_INFC:
		s += "INFC"
	case ESV_INFC_RES:
		s += "INFC_Res" // response for INFC(0x74)
	case ESV_SETGET_RES:
		s += "SetGet_Res" // response for SetGet(0x6e)
	case ESV_SETI_SNA:
		s += "SetI_SNA"
	case ESV_SETC_SNA:
		s += "SetC_SNA"
	case ESV_GET_SNA:
		s += "Get_SNA"
	case ESV_INF_SNA:
		s += "INF_SNA"
	case ESV_SETGET_SNA:
		s += "SetGet_SNA"
	default:
		s += fmt.Sprintf("0x%02x", pkt.ESV)
	}

	for _, prop := range pkt.Props {
		s += fmt.Sprintf(" EPC:%02x", prop.EPC)
		edt_list := prop.EDT
		if prop.EPC == 0x9d || prop.EPC == 0x9e || prop.EPC == 0x9f {
			if prop.PDC == 17 {
				edt_list = []byte{}
				for i := 0; i < 8; i++ {
					for j := 0; j < 16; j++ {
						if (prop.EDT[j+1] & (1 << i)) != 0 {
							edt_list = append(edt_list, byte(0x80+i*0x10+j))
						}
					}
				}
			} else {
				edt_list = prop.EDT[1:]
			}
		}

		for _, edt := range edt_list {
			s += fmt.Sprintf(" %02x", edt)
		}
	}

	return s
}

func (pkt *EchonetPacket) Parse(payload []byte) error {
	length := len(payload)
	if length < 12 {
		return fmt.Errorf("invalid length: %d", length)
	}

	pkt.EHD = uint16(payload[0])<<8 | uint16(payload[1])
	pkt.TID = uint16(payload[2])<<8 | uint16(payload[3])
	pkt.SEOJ = uint32(payload[4])<<16 | uint32(payload[5])<<8 | uint32(payload[6])
	pkt.DEOJ = uint32(payload[7])<<16 | uint32(payload[8])<<8 | uint32(payload[9])
	pkt.ESV = payload[10]
	pkt.OPC = payload[11]

	if pkt.EHD != 0x1081 {
		return fmt.Errorf("invalid EHD: 0x%04x", pkt.EHD)
	}

	idx := 12
	for i := 0; i < int(pkt.OPC); i++ {
		prop := EchonetProperty{
			EPC: payload[idx],
			PDC: payload[idx+1],
		}
		idx += 2
		for j := 0; j < int(prop.PDC); j++ {
			prop.EDT = append(prop.EDT, payload[idx])
			idx += 1
		}
		pkt.Props = append(pkt.Props, prop)
	}

	return nil
}

func (pkt *EchonetPacket) SetTid(tid uint16) {
	pkt.TID = tid
}

func (pkt *EchonetPacket) GetTid() uint16 {
	return pkt.TID
}

func (pkt *EchonetPacket) SetSeoj(seoj uint32) {
	pkt.SEOJ = seoj & 0xffffff
}

func (pkt *EchonetPacket) GetSeoj() uint32 {
	return pkt.SEOJ
}

func (pkt *EchonetPacket) SetDeoj(deoj uint32) {
	pkt.DEOJ = deoj & 0xffffff
}

func (pkt *EchonetPacket) GetDeoj() uint32 {
	return pkt.DEOJ
}

func (pkt *EchonetPacket) SetEsv(esv byte) {
	pkt.ESV = esv
}

func (pkt *EchonetPacket) GetEsv() byte {
	return pkt.ESV
}

func (pkt *EchonetPacket) AddProperty(epc byte, edt ...byte) {
	pkt.OPC += 1
	pkt.Props = append(pkt.Props, EchonetProperty{
		EPC: epc,
		PDC: byte(len(edt)),
		EDT: edt,
	})
}
