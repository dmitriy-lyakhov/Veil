package provision

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"net/netip"
	"time"

	"golang.org/x/crypto/bcrypt"

	"veil/internal/config"
	"veil/internal/site"
	"veil/internal/vpnconf/wgkey"
)

const (
	tunnelNet4 = "10.66.0.1/24"
	authKeyLen = 32
)

var DefaultDNS = []string{"1.1.1.1", "2606:4700:4700::1111"}

func NewServer(siteLang string) (*config.Server, error) {
	priv, _, err := wgkey.Generate()
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM, err := selfSignedCert()
	if err != nil {
		return nil, err
	}
	ula, err := randomULA()
	if err != nil {
		return nil, err
	}
	cover := site.Generate(siteLang)
	return &config.Server{
		Listen:     fmt.Sprintf(":%d", config.DefaultPort),
		Path:       "/api/v2/" + randomHex(8),
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		PrivateKey: priv,
		Address4:   tunnelNet4,
		Address6:   ula,
		MTU:        config.DefaultMTU,
		Interface:  config.DefaultInterface,
		Site:       &cover,
	}, nil
}

func AddRemotePeer(s *config.Server, name, publicKey, relayOf string) (config.Peer, error) {
	a4, a6, err := nextAddresses(s)
	if err != nil {
		return config.Peer{}, err
	}
	peer := config.Peer{
		Name:      name,
		PublicKey: publicKey,
		Address4:  netip.PrefixFrom(a4, 32).String(),
		Address6:  netip.PrefixFrom(a6, 128).String(),
		Created:   time.Now().UTC().Truncate(time.Second),
		RelayOf:   relayOf,
	}
	s.Peers = append(s.Peers, peer)
	return peer, nil
}

func NewAuthKey() string {
	auth := make([]byte, authKeyLen)
	_, _ = rand.Read(auth)
	return base64.StdEncoding.EncodeToString(auth)
}

func AddPeer(s *config.Server, name string) (*config.Profile, error) {
	a4, a6, err := nextAddresses(s)
	if err != nil {
		return nil, err
	}
	priv, pub, err := wgkey.Generate()
	if err != nil {
		return nil, err
	}
	auth := make([]byte, authKeyLen)
	if _, err := rand.Read(auth); err != nil {
		return nil, err
	}
	authB64 := base64.StdEncoding.EncodeToString(auth)
	s.Peers = append(s.Peers, config.Peer{
		Name:      name,
		PublicKey: pub,
		AuthKey:   authB64,
		Address4:  netip.PrefixFrom(a4, 32).String(),
		Address6:  netip.PrefixFrom(a6, 128).String(),
	})
	serverPub, err := wgkey.Public(s.PrivateKey)
	if err != nil {
		return nil, err
	}
	pin, err := CertPin(s.CertPEM)
	if err != nil {
		return nil, err
	}
	return &config.Profile{
		Name:            name,
		Port:            config.DefaultPort,
		Path:            s.Path,
		CertPin:         pin,
		FrontSNI:        frontSNI(s),
		FrontKey:        frontKey(s),
		ServerPublicKey: serverPub,
		PrivateKey:      priv,
		AuthKey:         authB64,
		Address4:        netip.PrefixFrom(a4, 32).String(),
		Address6:        netip.PrefixFrom(a6, 128).String(),
		DNS:             append([]string(nil), DefaultDNS...),
		MTU:             s.MTU,
	}, nil
}

func nextAddresses(s *config.Server) (netip.Addr, netip.Addr, error) {
	p4, err := netip.ParsePrefix(s.Address4)
	if err != nil {
		return netip.Addr{}, netip.Addr{}, err
	}
	p6, err := netip.ParsePrefix(s.Address6)
	if err != nil {
		return netip.Addr{}, netip.Addr{}, err
	}
	used := map[netip.Addr]bool{p4.Addr(): true}
	for _, pe := range s.Peers {
		if len(pe.Route) > 0 {
			continue
		}
		if pp, err := netip.ParsePrefix(pe.Address4); err == nil {
			used[pp.Addr()] = true
		}
	}
	a4 := p4.Masked().Addr().Next()
	steps := 0
	for ; p4.Contains(a4) && used[a4]; a4 = a4.Next() {
		steps++
	}
	broadcast := !p4.Contains(a4.Next())
	if !p4.Contains(a4) || broadcast {
		return netip.Addr{}, netip.Addr{}, errors.New("в туннельной подсети закончились адреса")
	}
	a6 := p6.Masked().Addr()
	for i := 0; i <= steps; i++ {
		a6 = a6.Next()
	}
	return a4, a6, nil
}

const (
	adminPasswordLen = 16
	adminAlphabet    = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

func HashAdminPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

func NewAdminPassword() string {
	b := make([]byte, adminPasswordLen)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = adminAlphabet[int(b[i])%len(adminAlphabet)]
	}
	return string(b)
}

func AdminURL(s *config.Server) string {
	if p, err := netip.ParsePrefix(s.Address4); err == nil {
		return "https://" + p.Addr().String() + "/admin"
	}
	return ""
}

func CertPin(certPEM string) (string, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return "", errors.New("некорректный PEM сертификата")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	return SPKIPin(cert), nil
}

func SPKIPin(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func selfSignedCert() (string, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return "", "", err
	}
	// дата выпуска в прошлом (20-120 дней), срок около года: десятилетний сертификат
	// с CN=localhost и датой "сегодня" - типичная примета самодельного сервера
	now := time.Now().Add(-time.Duration(20*24+serial.Int64()%(100*24)) * time.Hour)
	notAfter := now.AddDate(0, 0, 300+int(serial.Int64()%120))
	host := randomHostname()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM), nil
}

func randomULA() (string, error) {
	var b [16]byte
	b[0] = 0xfd
	if _, err := rand.Read(b[1:6]); err != nil {
		return "", err
	}
	b[15] = 1
	return netip.PrefixFrom(netip.AddrFrom16(b), 64).String(), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// randomHostname — правдоподобное имя вида «abcdefg.com» для самоподписанного сертификата.
// Подлинность проверяется по закреплённому ключу, а не по имени.
func randomHostname() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	n := 6 + mrand.IntN(5)
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[mrand.IntN(len(letters))]
	}
	tlds := []string{"com", "net", "org", "io"}
	return string(b) + "." + tlds[mrand.IntN(len(tlds))]
}

func frontSNI(s *config.Server) string {
	if s.Front != nil {
		return s.Front.SNI
	}
	return ""
}

func frontKey(s *config.Server) string {
	if s.Front != nil {
		return s.Front.Key
	}
	return ""
}
