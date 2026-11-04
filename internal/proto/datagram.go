package proto

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	dgWhole byte = 0
	dgFrag  byte = 1

	dgWholeHeader = 3
	dgFragHeader  = 5

	DefaultDatagramLimit = 1100
	minDatagramLimit     = 256
	maxFragments         = 8
	reassemblyTimeout    = 2 * time.Second
	maxPending           = 128
	maxLimitRetries      = 3

	http3DatagramOverhead = 8
)

var errDatagramMalformed = errors.New("malformed datagram")

type DatagramSender struct {
	mu     sync.Mutex
	send   func([]byte) error
	limit  int
	fixed  bool
	nextID uint16
	sent   int
	buf    []byte
}

func NewDatagramSender(send func([]byte) error) *DatagramSender {
	return &DatagramSender{send: send, limit: DefaultDatagramLimit, nextID: uint16(rand.Uint32())}
}

func NewFixedDatagramSender(send func([]byte) error, limit int) *DatagramSender {
	s := NewDatagramSender(send)
	s.limit, s.fixed = limit, true
	return s
}

func (s *DatagramSender) Limit() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.limit
}

func (s *DatagramSender) WritePacket(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent++
	whole := dgWholeHeader + len(p)
	if whole <= s.limit {
		pad := 0
		if s.sent <= warmupFrames {
			pad = min(rand.IntN(warmupPadMax), s.limit-whole)
		}
		err := s.sendWhole(p, pad)
		if !s.learn(err) {
			return err
		}
	} else if !s.fixed {
		err := s.sendWhole(p, 0)
		if err == nil {
			s.limit = whole
			return nil
		}
		if !s.learn(err) {
			return err
		}
	}
	for range maxLimitRetries {
		err := s.sendFragments(p)
		if !s.learn(err) {
			return err
		}
	}
	return errors.New("datagram path limit keeps shrinking")
}

func (s *DatagramSender) learn(err error) bool {
	var tooLarge *quic.DatagramTooLargeError
	if err == nil || !errors.As(err, &tooLarge) {
		return false
	}
	next := int(tooLarge.MaxDatagramPayloadSize) - http3DatagramOverhead
	if next >= s.limit {
		next = s.limit - http3DatagramOverhead
	}
	s.limit = max(minDatagramLimit, next)
	return true
}

func (s *DatagramSender) sendWhole(p []byte, pad int) error {
	s.buf = append(s.buf[:0], dgWhole, 0, 0)
	binary.BigEndian.PutUint16(s.buf[1:], uint16(len(p)))
	s.buf = append(s.buf, p...)
	s.buf = append(s.buf, make([]byte, pad)...)
	return s.send(s.buf)
}

func (s *DatagramSender) sendFragments(p []byte) error {
	chunk := s.limit - dgFragHeader
	count := (len(p) + chunk - 1) / chunk
	if count > maxFragments {
		return errors.New("packet too large for datagram path")
	}
	id := s.nextID
	s.nextID++
	for i := range count {
		part := p[i*chunk : min(len(p), (i+1)*chunk)]
		s.buf = append(s.buf[:0], dgFrag, 0, 0, byte(i), byte(count))
		binary.BigEndian.PutUint16(s.buf[1:], id)
		s.buf = append(s.buf, part...)
		if err := s.send(s.buf); err != nil {
			return err
		}
	}
	return nil
}

type pending struct {
	parts    [maxFragments][]byte
	got      int
	count    int
	deadline time.Time
}

type DatagramReceiver struct {
	pending map[uint16]*pending
	now     func() time.Time
	out     []byte
}

func NewDatagramReceiver() *DatagramReceiver {
	return &DatagramReceiver{pending: map[uint16]*pending{}, now: time.Now}
}

func (r *DatagramReceiver) Decode(d []byte) ([]byte, error) {
	if len(d) == 0 {
		return nil, errDatagramMalformed
	}
	switch d[0] {
	case dgWhole:
		if len(d) < dgWholeHeader {
			return nil, errDatagramMalformed
		}
		n := int(binary.BigEndian.Uint16(d[1:]))
		if dgWholeHeader+n > len(d) {
			return nil, errDatagramMalformed
		}
		return d[dgWholeHeader : dgWholeHeader+n], nil
	case dgFrag:
		return r.fragment(d)
	}
	return nil, errDatagramMalformed
}

func (r *DatagramReceiver) fragment(d []byte) ([]byte, error) {
	if len(d) <= dgFragHeader {
		return nil, errDatagramMalformed
	}
	id := binary.BigEndian.Uint16(d[1:])
	idx, count := int(d[3]), int(d[4])
	if count < 2 || count > maxFragments || idx >= count {
		return nil, errDatagramMalformed
	}
	now := r.now()
	pe := r.pending[id]
	if pe == nil || pe.count != count || now.After(pe.deadline) {
		if len(r.pending) >= maxPending {
			r.expire(now)
		}
		pe = &pending{count: count, deadline: now.Add(reassemblyTimeout)}
		r.pending[id] = pe
	}
	if pe.parts[idx] != nil {
		return nil, nil
	}
	pe.parts[idx] = append([]byte(nil), d[dgFragHeader:]...)
	pe.got++
	if pe.got < pe.count {
		return nil, nil
	}
	delete(r.pending, id)
	r.out = r.out[:0]
	for i := range pe.count {
		r.out = append(r.out, pe.parts[i]...)
	}
	return r.out, nil
}

func (r *DatagramReceiver) expire(now time.Time) {
	var oldest uint16
	var oldestAt time.Time
	for id, pe := range r.pending {
		if now.After(pe.deadline) {
			delete(r.pending, id)
			continue
		}
		if oldestAt.IsZero() || pe.deadline.Before(oldestAt) {
			oldest, oldestAt = id, pe.deadline
		}
	}
	if len(r.pending) >= maxPending {
		delete(r.pending, oldest)
	}
}
