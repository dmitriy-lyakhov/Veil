package proto

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/quic-go/quic-go"
)

func pathWithMTU(mtu int, sink *[][]byte) func([]byte) error {
	return pathWithPrefix(mtu, 0, sink)
}

func pathWithPrefix(mtu, prefix int, sink *[][]byte) func([]byte) error {
	return func(b []byte) error {
		if len(b)+prefix > mtu {
			return &quic.DatagramTooLargeError{MaxDatagramPayloadSize: int64(mtu)}
		}
		*sink = append(*sink, append([]byte(nil), b...))
		return nil
	}
}

func TestDatagramRoundTrip(t *testing.T) {
	for _, mtu := range []int{1200, 1350, 1452} {
		for _, size := range []int{32, 148, 1100, 1300, 1412} {
			t.Run(fmt.Sprintf("mtu%d/pkt%d", mtu, size), func(t *testing.T) {
				var wire [][]byte
				s := NewDatagramSender(pathWithMTU(mtu, &wire))
				r := NewDatagramReceiver()
				for i := 0; i < 40; i++ {
					p := make([]byte, size)
					for j := range p {
						p[j] = byte(rand.Uint32())
					}
					wire = wire[:0]
					if err := s.WritePacket(p); err != nil {
						t.Fatal(err)
					}
					var got []byte
					for _, d := range wire {
						if len(d) > mtu {
							t.Fatalf("datagram %d exceeds path mtu %d", len(d), mtu)
						}
						out, err := r.Decode(d)
						if err != nil {
							t.Fatal(err)
						}
						if out != nil {
							got = append([]byte(nil), out...)
						}
					}
					if !bytes.Equal(got, p) {
						t.Fatalf("packet %d: mismatch (got %d bytes, want %d)", i, len(got), len(p))
					}
				}
			})
		}
	}
}

func TestDatagramFragmentsOutOfOrderAndLoss(t *testing.T) {
	var wire [][]byte
	s := NewDatagramSender(pathWithMTU(700, &wire))
	r := NewDatagramReceiver()
	p := bytes.Repeat([]byte{7}, 1400)
	if err := s.WritePacket(p); err != nil {
		t.Fatal(err)
	}
	if len(wire) < 3 {
		t.Fatalf("expected fragmentation, got %d datagrams", len(wire))
	}
	for i := len(wire) - 1; i >= 0; i-- {
		out, err := r.Decode(wire[i])
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && !bytes.Equal(out, p) {
			t.Fatal("reordered reassembly failed")
		}
	}
	wire = wire[:0]
	if err := s.WritePacket(p); err != nil {
		t.Fatal(err)
	}
	for _, d := range wire[1:] {
		if out, _ := r.Decode(d); out != nil {
			t.Fatal("incomplete packet delivered")
		}
	}
	if len(r.pending) != 1 {
		t.Fatalf("pending = %d", len(r.pending))
	}
}

func TestDatagramRejectsGarbage(t *testing.T) {
	r := NewDatagramReceiver()
	for _, d := range [][]byte{{}, {9}, {0, 0xff, 0xff, 1}, {1, 0, 0, 5, 3, 1}, {1, 0, 0, 0, 1, 1}} {
		if _, err := r.Decode(d); err == nil {
			t.Fatalf("accepted %v", d)
		}
	}
}

func TestDatagramAccountsForHTTP3Prefix(t *testing.T) {
	for _, prefix := range []int{1, 2, 4, 8} {
		var wire [][]byte
		s := NewDatagramSender(pathWithPrefix(1200, prefix, &wire))
		r := NewDatagramReceiver()
		for i := 0; i < 20; i++ {
			p := bytes.Repeat([]byte{byte(i)}, 1412)
			wire = wire[:0]
			if err := s.WritePacket(p); err != nil {
				t.Fatalf("prefix %d packet %d: %v", prefix, i, err)
			}
			var got []byte
			for _, d := range wire {
				if out, _ := r.Decode(d); out != nil {
					got = append([]byte(nil), out...)
				}
			}
			if !bytes.Equal(got, p) {
				t.Fatalf("prefix %d packet %d lost", prefix, i)
			}
		}
	}
}

func TestFixedSenderNeverExceedsLimit(t *testing.T) {
	var wire [][]byte
	silentDrop := func(b []byte) error {
		wire = append(wire, append([]byte(nil), b...))
		return nil
	}
	s := NewFixedDatagramSender(silentDrop, 1150)
	r := NewDatagramReceiver()
	for _, size := range []int{60, 1147, 1148, 1412, 3000} {
		wire = wire[:0]
		p := bytes.Repeat([]byte{byte(size)}, size)
		if err := s.WritePacket(p); err != nil {
			t.Fatal(err)
		}
		var got []byte
		for _, d := range wire {
			if len(d) > 1150 {
				t.Fatalf("size %d: datagram of %d bytes exceeds fixed limit", size, len(d))
			}
			if out, _ := r.Decode(d); out != nil {
				got = append([]byte(nil), out...)
			}
		}
		if !bytes.Equal(got, p) {
			t.Fatalf("size %d lost", size)
		}
	}
}
