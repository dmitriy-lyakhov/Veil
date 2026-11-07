package proto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

const (
	tokenTSLen    = 8
	tokenNonceLen = 16
	tokenMACLen   = 16
	tokenHeadLen  = tokenTSLen + tokenNonceLen
	tokenLen      = tokenHeadLen + tokenMACLen
	TokenWindow   = 90 * time.Second
	gcInterval    = 10 * time.Second
)

var (
	ErrBadToken = errors.New("invalid token")
	ErrReplay   = errors.New("replayed token")
	authLabel   = []byte("veil/auth/v1")
)

func NewToken(key []byte, now time.Time) string {
	buf := make([]byte, tokenHeadLen, tokenLen)
	binary.BigEndian.PutUint64(buf, uint64(now.Unix()))
	_, _ = rand.Read(buf[tokenTSLen:])
	buf = append(buf, tokenMAC(key, buf)...)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func tokenMAC(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(authLabel)
	m.Write(msg)
	return m.Sum(nil)[:tokenMACLen]
}

type Verifier struct {
	keysMu sync.RWMutex
	keys   map[string][]byte
	now    func() time.Time
	mu     sync.Mutex
	seen   map[[tokenNonceLen]byte]time.Time
	lastGC time.Time
}

func NewVerifier(keys map[string][]byte) *Verifier {
	if keys == nil {
		keys = map[string][]byte{}
	}
	return &Verifier{keys: keys, now: time.Now, seen: map[[tokenNonceLen]byte]time.Time{}}
}

func (v *Verifier) Set(peer string, key []byte) {
	v.keysMu.Lock()
	v.keys[peer] = key
	v.keysMu.Unlock()
}

func (v *Verifier) Remove(peer string) {
	v.keysMu.Lock()
	delete(v.keys, peer)
	v.keysMu.Unlock()
}

func (v *Verifier) Verify(token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != tokenLen {
		return "", ErrBadToken
	}
	now := v.now()
	ts := time.Unix(int64(binary.BigEndian.Uint64(raw)), 0)
	if d := now.Sub(ts); d > TokenWindow || d < -TokenWindow {
		return "", ErrBadToken
	}
	head, mac := raw[:tokenHeadLen], raw[tokenHeadLen:]
	peer := ""
	v.keysMu.RLock()
	for name, key := range v.keys {
		if hmac.Equal(tokenMAC(key, head), mac) {
			peer = name
			break
		}
	}
	v.keysMu.RUnlock()
	if peer == "" {
		return "", ErrBadToken
	}
	var nonce [tokenNonceLen]byte
	copy(nonce[:], raw[tokenTSLen:tokenHeadLen])
	v.mu.Lock()
	defer v.mu.Unlock()
	if now.Sub(v.lastGC) > gcInterval {
		for n, exp := range v.seen {
			if now.After(exp) {
				delete(v.seen, n)
			}
		}
		v.lastGC = now
	}
	if _, dup := v.seen[nonce]; dup {
		return "", ErrReplay
	}
	v.seen[nonce] = now.Add(2 * TokenWindow)
	return peer, nil
}
