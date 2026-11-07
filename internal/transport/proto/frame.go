package proto

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"sync"
)

const (
	frameData      byte = 0
	framePad       byte = 1
	frameHeaderLen      = 3
	MaxPayload          = 1<<16 - 1

	warmupFrames   = 24
	warmupPadMax   = 512
	steadyPadOdds  = 16
	steadyPadMax   = 256
	readBufferSize = 64 << 10
)

var ErrFrameType = errors.New("unknown frame type")

type FrameWriter struct {
	mu     sync.Mutex
	w      io.Writer
	flush  func()
	buf    []byte
	frames int
	closed bool
}

func NewFrameWriter(w io.Writer, flush func()) *FrameWriter {
	return &FrameWriter{w: w, flush: flush}
}

func (fw *FrameWriter) WritePacket(p []byte) error {
	if len(p) > MaxPayload {
		return errors.New("packet too large")
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if fw.closed {
		return net.ErrClosed
	}
	fw.buf = appendFrame(fw.buf[:0], frameData, len(p))
	fw.buf = append(fw.buf, p...)
	if n := fw.paddingLen(); n > 0 {
		fw.buf = appendFrame(fw.buf, framePad, n)
		fw.buf = append(fw.buf, make([]byte, n)...)
	}
	fw.frames++
	if _, err := fw.w.Write(fw.buf); err != nil {
		fw.closed = true
		return err
	}
	if fw.flush != nil {
		fw.flush()
	}
	return nil
}

func (fw *FrameWriter) paddingLen() int {
	if fw.frames < warmupFrames {
		return rand.IntN(warmupPadMax)
	}
	if rand.IntN(steadyPadOdds) == 0 {
		return rand.IntN(steadyPadMax)
	}
	return 0
}

func (fw *FrameWriter) Close() {
	fw.mu.Lock()
	fw.closed = true
	fw.mu.Unlock()
}

func appendFrame(b []byte, typ byte, n int) []byte {
	b = append(b, typ, 0, 0)
	binary.BigEndian.PutUint16(b[len(b)-2:], uint16(n))
	return b
}

type FrameReader struct {
	r   *bufio.Reader
	hdr [frameHeaderLen]byte
	buf []byte
}

func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: bufio.NewReaderSize(r, readBufferSize), buf: make([]byte, MaxPayload)}
}

func (fr *FrameReader) ReadPacket() ([]byte, error) {
	for {
		if _, err := io.ReadFull(fr.r, fr.hdr[:]); err != nil {
			return nil, err
		}
		n := int(binary.BigEndian.Uint16(fr.hdr[1:]))
		switch fr.hdr[0] {
		case frameData:
			if _, err := io.ReadFull(fr.r, fr.buf[:n]); err != nil {
				return nil, err
			}
			return fr.buf[:n], nil
		case framePad:
			if _, err := fr.r.Discard(n); err != nil {
				return nil, err
			}
		default:
			return nil, ErrFrameType
		}
	}
}
