package front

import (
	"encoding/base64"
	"errors"
	"time"

	utls "github.com/refraction-networking/utls"
)

// DecodeKey разбирает ключ из профиля (base64).
func DecodeKey(s string) ([]byte, error) {
	k, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(k) != KeyLen {
		return nil, errors.New("front key must be 32 bytes")
	}
	return k, nil
}

// Apply кладёт метку в session_id ClientHello клиента uTLS. Вызывается до Handshake.
func Apply(uc *utls.UConn, key []byte) error {
	if err := uc.BuildHandshakeState(); err != nil {
		return err
	}
	h := uc.HandshakeState.Hello
	if len(h.SessionId) != SessionIDLen {
		return errors.New("ClientHello has no 32-byte session_id")
	}
	copy(h.SessionId, Tag(key, h.Random, time.Now()))
	return uc.MarshalClientHello()
}
