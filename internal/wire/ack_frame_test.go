package wire

import (
	"bytes"
	"io"
	"math"
	"time"

	"github.com/fkwhite/quic-go/internal/protocol"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ACK Frame (for IETF QUIC)", func() {
	Context("parsing", func() {
		It("parses an ACK frame without any ranges", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(100)...) // largest acked
			data = append(data, encodeVarInt(0)...)   // delay
			data = append(data, encodeVarInt(0)...)   // num blocks
			data = append(data, encodeVarInt(10)...)  // first ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(100)))
			Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(90)))
			Expect(frame.HasMissingRanges()).To(BeFalse())
			Expect(b.Len()).To(BeZero())
		})

		It("parses an ACK frame that only acks a single packet", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(55)...) // largest acked
			data = append(data, encodeVarInt(0)...)  // delay
			data = append(data, encodeVarInt(0)...)  // num blocks
			data = append(data, encodeVarInt(0)...)  // first ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(55)))
			Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(55)))
			Expect(frame.HasMissingRanges()).To(BeFalse())
			Expect(b.Len()).To(BeZero())
		})

		It("accepts an ACK frame that acks all packets from 0 to largest", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(20)...) // largest acked
			data = append(data, encodeVarInt(0)...)  // delay
			data = append(data, encodeVarInt(0)...)  // num blocks
			data = append(data, encodeVarInt(20)...) // first ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(20)))
			Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(0)))
			Expect(frame.HasMissingRanges()).To(BeFalse())
			Expect(b.Len()).To(BeZero())
		})

		It("rejects an ACK frame that has a first ACK block which is larger than LargestAcked", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(20)...) // largest acked
			data = append(data, encodeVarInt(0)...)  // delay
			data = append(data, encodeVarInt(0)...)  // num blocks
			data = append(data, encodeVarInt(21)...) // first ack block
			b := bytes.NewReader(data)
			_, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).To(MatchError("invalid first ACK range"))
		})

		It("parses an ACK frame that has a single block", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(1000)...) // largest acked
			data = append(data, encodeVarInt(0)...)    // delay
			data = append(data, encodeVarInt(1)...)    // num blocks
			data = append(data, encodeVarInt(100)...)  // first ack block
			data = append(data, encodeVarInt(98)...)   // gap
			data = append(data, encodeVarInt(50)...)   // ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(1000)))
			Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(750)))
			Expect(frame.HasMissingRanges()).To(BeTrue())
			Expect(frame.AckRanges).To(Equal([]AckRange{
				{Largest: 1000, Smallest: 900},
				{Largest: 800, Smallest: 750},
			}))
			Expect(b.Len()).To(BeZero())
		})

		It("parses an ACK frame that has a multiple blocks", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(100)...) // largest acked
			data = append(data, encodeVarInt(0)...)   // delay
			data = append(data, encodeVarInt(2)...)   // num blocks
			data = append(data, encodeVarInt(0)...)   // first ack block
			data = append(data, encodeVarInt(0)...)   // gap
			data = append(data, encodeVarInt(0)...)   // ack block
			data = append(data, encodeVarInt(1)...)   // gap
			data = append(data, encodeVarInt(1)...)   // ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(100)))
			Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(94)))
			Expect(frame.HasMissingRanges()).To(BeTrue())
			Expect(frame.AckRanges).To(Equal([]AckRange{
				{Largest: 100, Smallest: 100},
				{Largest: 98, Smallest: 98},
				{Largest: 95, Smallest: 94},
			}))
			Expect(b.Len()).To(BeZero())
		})

		It("uses the ack delay exponent", func() {
			const delayTime = 1 << 10 * time.Millisecond
			f := &AckFrame{
				AckRanges: []AckRange{{Smallest: 1, Largest: 1}},
				DelayTime: delayTime,
			}
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			for i := uint8(0); i < 8; i++ {
				r := bytes.NewReader(b)
				frame, err := parseAckFrame(r, protocol.AckDelayExponent+i, protocol.Version1)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.DelayTime).To(Equal(delayTime * (1 << i)))
			}
		})

		It("gracefully handles overflows of the delay time", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(100)...)              // largest acked
			data = append(data, encodeVarInt(math.MaxUint64/5)...) // delay
			data = append(data, encodeVarInt(0)...)                // num blocks
			data = append(data, encodeVarInt(0)...)                // first ack block
			b := bytes.NewReader(data)
			frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.DelayTime).To(BeNumerically(">", 0))
			// The maximum encodable duration is ~292 years.
			Expect(frame.DelayTime.Hours()).To(BeNumerically("~", 292*365*24, 365*24))
		})

		It("errors on EOF", func() {
			data := []byte{0x2}
			data = append(data, encodeVarInt(1000)...) // largest acked
			data = append(data, encodeVarInt(0)...)    // delay
			data = append(data, encodeVarInt(1)...)    // num blocks
			data = append(data, encodeVarInt(100)...)  // first ack block
			data = append(data, encodeVarInt(98)...)   // gap
			data = append(data, encodeVarInt(50)...)   // ack block
			_, err := parseAckFrame(bytes.NewReader(data), protocol.AckDelayExponent, protocol.Version1)
			Expect(err).NotTo(HaveOccurred())
			for i := range data {
				_, err := parseAckFrame(bytes.NewReader(data[0:i]), protocol.AckDelayExponent, protocol.Version1)
				Expect(err).To(MatchError(io.EOF))
			}
		})

		Context("ACK_ECN", func() {
			It("parses", func() {
				data := []byte{0x3}
				data = append(data, encodeVarInt(100)...)        // largest acked
				data = append(data, encodeVarInt(0)...)          // delay
				data = append(data, encodeVarInt(0)...)          // num blocks
				data = append(data, encodeVarInt(10)...)         // first ack block
				data = append(data, encodeVarInt(0x42)...)       // ECT(0)
				data = append(data, encodeVarInt(0x12345)...)    // ECT(1)
				data = append(data, encodeVarInt(0x12345678)...) // ECN-CE
				b := bytes.NewReader(data)
				frame, err := parseAckFrame(b, protocol.AckDelayExponent, protocol.Version1)
				Expect(err).ToNot(HaveOccurred())
				Expect(frame.LargestAcked()).To(Equal(protocol.PacketNumber(100)))
				Expect(frame.LowestAcked()).To(Equal(protocol.PacketNumber(90)))
				Expect(frame.HasMissingRanges()).To(BeFalse())
				Expect(b.Len()).To(BeZero())
			})

			It("errors on EOF", func() {
				data := []byte{0x3}
				data = append(data, encodeVarInt(1000)...)       // largest acked
				data = append(data, encodeVarInt(0)...)          // delay
				data = append(data, encodeVarInt(1)...)          // num blocks
				data = append(data, encodeVarInt(100)...)        // first ack block
				data = append(data, encodeVarInt(98)...)         // gap
				data = append(data, encodeVarInt(50)...)         // ack block
				data = append(data, encodeVarInt(0x42)...)       // ECT(0)
				data = append(data, encodeVarInt(0x12345)...)    // ECT(1)
				data = append(data, encodeVarInt(0x12345678)...) // ECN-CE
				_, err := parseAckFrame(bytes.NewReader(data), protocol.AckDelayExponent, protocol.Version1)
				Expect(err).NotTo(HaveOccurred())
				for i := range data {
					_, err := parseAckFrame(bytes.NewReader(data[0:i]), protocol.AckDelayExponent, protocol.Version1)
					Expect(err).To(MatchError(io.EOF))
				}
			})
		})
	})

	Context("when writing", func() {
		It("writes a simple frame", func() {
			f := &AckFrame{
				AckRanges: []AckRange{{Smallest: 100, Largest: 1337}},
			}
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			expected := []byte{0x2}
			expected = append(expected, encodeVarInt(1337)...) // largest acked
			expected = append(expected, 0)                     // delay
			expected = append(expected, encodeVarInt(0)...)    // num ranges
			expected = append(expected, encodeVarInt(1337-100)...)
			Expect(b).To(Equal(expected))
		})

		It("writes an ACK-ECN frame", func() {
			f := &AckFrame{
				AckRanges: []AckRange{{Smallest: 10, Largest: 2000}},
				ECT0:      13,
				ECT1:      37,
				ECNCE:     12345,
			}
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			expected := []byte{0x3}
			expected = append(expected, encodeVarInt(2000)...) // largest acked
			expected = append(expected, 0)                     // delay
			expected = append(expected, encodeVarInt(0)...)    // num ranges
			expected = append(expected, encodeVarInt(2000-10)...)
			expected = append(expected, encodeVarInt(13)...)
			expected = append(expected, encodeVarInt(37)...)
			expected = append(expected, encodeVarInt(12345)...)
			Expect(b).To(Equal(expected))
		})

		It("writes a frame that acks a single packet", func() {
			f := &AckFrame{
				AckRanges: []AckRange{{Smallest: 0x2eadbeef, Largest: 0x2eadbeef}},
				DelayTime: 18 * time.Millisecond,
			}
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			r := bytes.NewReader(b)
			frame, err := parseAckFrame(r, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame).To(Equal(f))
			Expect(frame.HasMissingRanges()).To(BeFalse())
			Expect(frame.DelayTime).To(Equal(f.DelayTime))
			Expect(r.Len()).To(BeZero())
		})

		It("writes a frame that acks many packets", func() {
			f := &AckFrame{
				AckRanges: []AckRange{{Smallest: 0x1337, Largest: 0x2eadbeef}},
			}
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			r := bytes.NewReader(b)
			frame, err := parseAckFrame(r, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame).To(Equal(f))
			Expect(frame.HasMissingRanges()).To(BeFalse())
			Expect(r.Len()).To(BeZero())
		})

		It("writes a frame with a a single gap", func() {
			f := &AckFrame{
				AckRanges: []AckRange{
					{Smallest: 400, Largest: 1000},
					{Smallest: 100, Largest: 200},
				},
			}
			Expect(f.validateAckRanges()).To(BeTrue())
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			r := bytes.NewReader(b)
			frame, err := parseAckFrame(r, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame).To(Equal(f))
			Expect(frame.HasMissingRanges()).To(BeTrue())
			Expect(r.Len()).To(BeZero())
		})

		It("writes a frame with multiple ranges", func() {
			f := &AckFrame{
				AckRanges: []AckRange{
					{Smallest: 10, Largest: 10},
					{Smallest: 8, Largest: 8},
					{Smallest: 5, Largest: 6},
					{Smallest: 1, Largest: 3},
				},
			}
			Expect(f.validateAckRanges()).To(BeTrue())
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			r := bytes.NewReader(b)
			frame, err := parseAckFrame(r, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame).To(Equal(f))
			Expect(frame.HasMissingRanges()).To(BeTrue())
			Expect(r.Len()).To(BeZero())
		})

		It("limits the maximum size of the ACK frame", func() {
			const numRanges = 1000
			ackRanges := make([]AckRange, numRanges)
			for i := protocol.PacketNumber(1); i <= numRanges; i++ {
				ackRanges[numRanges-i] = AckRange{Smallest: 2 * i, Largest: 2 * i}
			}
			f := &AckFrame{AckRanges: ackRanges}
			Expect(f.validateAckRanges()).To(BeTrue())
			b, err := f.Append(nil, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(HaveLen(int(f.Length(protocol.Version1))))
			// make sure the ACK frame is *a little bit* smaller than the MaxAckFrameSize
			Expect(len(b)).To(BeNumerically(">", protocol.MaxAckFrameSize-5))
			Expect(len(b)).To(BeNumerically("<=", protocol.MaxAckFrameSize))
			r := bytes.NewReader(b)
			frame, err := parseAckFrame(r, protocol.AckDelayExponent, protocol.Version1)
			Expect(err).ToNot(HaveOccurred())
			Expect(frame.HasMissingRanges()).To(BeTrue())
			Expect(r.Len()).To(BeZero())
			Expect(len(frame.AckRanges)).To(BeNumerically("<", numRanges)) // make sure we dropped some ranges
		})
	})

	Context("ACK range validator", func() {
		It("rejects ACKs without ranges", func() {
			Expect((&AckFrame{}).validateAckRanges()).To(BeFalse())
		})

		It("accepts an ACK without NACK Ranges", func() {
			ack := AckFrame{
				AckRanges: []AckRange{{Smallest: 1, Largest: 7}},
			}
			Expect(ack.validateAckRanges()).To(BeTrue())
		})

		It("rejects ACK ranges with Smallest greater than Largest", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 8, Largest: 10},
					{Smallest: 4, Largest: 3},
				},
			}
			Expect(ack.validateAckRanges()).To(BeFalse())
		})

		It("rejects ACK ranges in the wrong order", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 2, Largest: 2},
					{Smallest: 6, Largest: 7},
				},
			}
			Expect(ack.validateAckRanges()).To(BeFalse())
		})

		It("rejects with overlapping ACK ranges", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 5, Largest: 7},
					{Smallest: 2, Largest: 5},
				},
			}
			Expect(ack.validateAckRanges()).To(BeFalse())
		})

		It("rejects ACK ranges that are part of a larger ACK range", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 4, Largest: 7},
					{Smallest: 5, Largest: 6},
				},
			}
			Expect(ack.validateAckRanges()).To(BeFalse())
		})

		It("rejects with directly adjacent ACK ranges", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 5, Largest: 7},
					{Smallest: 2, Largest: 4},
				},
			}
			Expect(ack.validateAckRanges()).To(BeFalse())
		})

		It("accepts an ACK with one lost packet", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 5, Largest: 10},
					{Smallest: 1, Largest: 3},
				},
			}
			Expect(ack.validateAckRanges()).To(BeTrue())
		})

		It("accepts an ACK with multiple lost packets", func() {
			ack := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 15, Largest: 20},
					{Smallest: 10, Largest: 12},
					{Smallest: 1, Largest: 3},
				},
			}
			Expect(ack.validateAckRanges()).To(BeTrue())
		})
	})

	Context("check if ACK frame acks a certain packet", func() {
		It("works with an ACK without any ranges", func() {
			f := AckFrame{
				AckRanges: []AckRange{{Smallest: 5, Largest: 10}},
			}
			Expect(f.AcksPacket(1)).To(BeFalse())
			Expect(f.AcksPacket(4)).To(BeFalse())
			Expect(f.AcksPacket(5)).To(BeTrue())
			Expect(f.AcksPacket(8)).To(BeTrue())
			Expect(f.AcksPacket(10)).To(BeTrue())
			Expect(f.AcksPacket(11)).To(BeFalse())
			Expect(f.AcksPacket(20)).To(BeFalse())
		})

		It("works with an ACK with multiple ACK ranges", func() {
			f := AckFrame{
				AckRanges: []AckRange{
					{Smallest: 15, Largest: 20},
					{Smallest: 5, Largest: 8},
				},
			}
			Expect(f.AcksPacket(4)).To(BeFalse())
			Expect(f.AcksPacket(5)).To(BeTrue())
			Expect(f.AcksPacket(6)).To(BeTrue())
			Expect(f.AcksPacket(7)).To(BeTrue())
			Expect(f.AcksPacket(8)).To(BeTrue())
			Expect(f.AcksPacket(9)).To(BeFalse())
			Expect(f.AcksPacket(14)).To(BeFalse())
			Expect(f.AcksPacket(15)).To(BeTrue())
			Expect(f.AcksPacket(18)).To(BeTrue())
			Expect(f.AcksPacket(19)).To(BeTrue())
			Expect(f.AcksPacket(20)).To(BeTrue())
			Expect(f.AcksPacket(21)).To(BeFalse())
		})
	})
})
