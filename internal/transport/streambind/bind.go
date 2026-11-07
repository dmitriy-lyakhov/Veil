package streambind

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"

	"golang.zx2c4.com/wireguard/conn"
)

const (
	queueLen  = 1024
	batchSize = 16
)

var ErrNoTransport = errors.New("no transport connected")

type Sink interface {
	WritePacket([]byte) error
}

type Endpoint struct {
	name string
	sink Sink
}

func NewEndpoint(name string, sink Sink) *Endpoint {
	return &Endpoint{name: name, sink: sink}
}

func (e *Endpoint) ClearSrc()           {}
func (e *Endpoint) SrcToString() string { return "" }
func (e *Endpoint) DstToString() string { return e.name }
func (e *Endpoint) DstToBytes() []byte  { return []byte(e.name) }
func (e *Endpoint) DstIP() netip.Addr   { return netip.Addr{} }
func (e *Endpoint) SrcIP() netip.Addr   { return netip.Addr{} }

type packet struct {
	data []byte
	ep   *Endpoint
}

type sinkRef struct{ Sink }

type Bind struct {
	in      chan packet
	virtual *Endpoint
	def     atomic.Pointer[sinkRef]
	mu      sync.Mutex
	done    chan struct{}
}

func New() *Bind {
	return &Bind{in: make(chan packet, queueLen), virtual: NewEndpoint("veil", nil)}
}

func (b *Bind) Virtual() *Endpoint { return b.virtual }

func (b *Bind) SetDefault(s Sink) {
	if s == nil {
		b.def.Store(nil)
		return
	}
	b.def.Store(&sinkRef{s})
}

func (b *Bind) Deliver(ep *Endpoint, p []byte) bool {
	pkt := packet{data: append([]byte(nil), p...), ep: ep}
	select {
	case b.in <- pkt:
		return true
	default:
		return false
	}
}

func (b *Bind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}
	done := make(chan struct{})
	b.done = done
	return []conn.ReceiveFunc{b.receiver(done)}, port, nil
}

func (b *Bind) receiver(done chan struct{}) conn.ReceiveFunc {
	return func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		var pkt packet
		select {
		case <-done:
			return 0, net.ErrClosed
		case pkt = <-b.in:
		}
		n := 0
		for {
			if len(pkt.data) <= len(packets[n]) {
				sizes[n] = copy(packets[n], pkt.data)
				eps[n] = pkt.ep
				n++
			}
			if n == len(packets) {
				return n, nil
			}
			select {
			case pkt = <-b.in:
			default:
				return n, nil
			}
		}
	}
}

func (b *Bind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done != nil {
		close(b.done)
		b.done = nil
	}
	return nil
}

func (b *Bind) SetMark(uint32) error { return nil }

func (b *Bind) BatchSize() int { return batchSize }

func (b *Bind) ParseEndpoint(string) (conn.Endpoint, error) { return b.virtual, nil }

func (b *Bind) Send(bufs [][]byte, ep conn.Endpoint) error {
	var s Sink
	if e, ok := ep.(*Endpoint); ok && e.sink != nil {
		s = e.sink
	} else if r := b.def.Load(); r != nil {
		s = r.Sink
	}
	if s == nil {
		return ErrNoTransport
	}
	for _, buf := range bufs {
		if err := s.WritePacket(buf); err != nil {
			return err
		}
	}
	return nil
}
