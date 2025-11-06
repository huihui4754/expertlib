package types

type MessageHeader struct {
	Magic      uint32
	Version    uint16
	Type       uint16
	BodyLength uint32
	Reserved   uint32
}
