package wgkey

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

const Size = 32

func Generate() (priv, pub string, err error) {
	var k [Size]byte
	if _, err := rand.Read(k[:]); err != nil {
		return "", "", err
	}
	k[0] &= 248
	k[31] = (k[31] & 127) | 64
	p, err := curve25519.X25519(k[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(k[:]), base64.StdEncoding.EncodeToString(p), nil
}

func Public(priv string) (string, error) {
	k, err := decode(priv)
	if err != nil {
		return "", err
	}
	p, err := curve25519.X25519(k, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

func Hex(key string) (string, error) {
	k, err := decode(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(k), nil
}

func decode(key string) ([]byte, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(k) != Size {
		return nil, fmt.Errorf("некорректный ключ WireGuard")
	}
	return k, nil
}
