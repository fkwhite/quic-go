package ackhandler

import (
	"sync"
	"time"

	"github.com/fkwhite/quic-go/internal/protocol"
)

// A Packet is a packet
type Packet struct {
	PacketNumber    protocol.PacketNumber
	Frames          []Frame
	LargestAcked    protocol.PacketNumber // InvalidPacketNumber if the packet doesn't contain an ACK
	Length          protocol.ByteCount
	EncryptionLevel protocol.EncryptionLevel
	SendTime        time.Time

	IsPathMTUProbePacket bool // We don't report the loss of Path MTU probe packets to the congestion controller.

	includedInBytesInFlight bool
	declaredLost            bool
	skippedPacket           bool
}

func (p *Packet) outstanding() bool {
	return !p.declaredLost && !p.skippedPacket && !p.IsPathMTUProbePacket
}

var packetPool = sync.Pool{New: func() any { return &Packet{} }}

func GetPacket() *Packet {
	p := packetPool.Get().(*Packet)
	p.PacketNumber = 0
	p.Frames = nil
	p.LargestAcked = 0
	p.Length = 0
	p.EncryptionLevel = protocol.EncryptionLevel(0)
	p.SendTime = time.Time{}
	p.IsPathMTUProbePacket = false
	p.includedInBytesInFlight = false
	p.declaredLost = false
	p.skippedPacket = false
	return p
}

// We currently only return Packets back into the pool when they're acknowledged (not when they're lost).
// This simplifies the code, and gives the vast majority of the performance benefit we can gain from using the pool.
func putPacket(p *Packet) { packetPool.Put(p) }
