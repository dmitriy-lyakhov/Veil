// Package ipsecconf — данные клиента IPsec/IKEv2 (strongSwan): адрес сервера, сертификат центра
// сертификации и учётные данные (логин и пароль для EAP-MSCHAPv2 либо сертификат клиента).
package ipsecconf

import (
	"errors"
	"fmt"
	"strings"
)

const (
	AuthEAP  = "eap"  // EAP-MSCHAPv2: логин и пароль
	AuthCert = "cert" // сертификат клиента (PKCS#12)
)

type Config struct {
	Server   string `json:"server"` // адрес сервера (IP или имя) — он же в SAN серверного сертификата
	CAPEM    string `json:"ca_pem"`
	Auth     string `json:"auth"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// сертификат клиента: PKCS#12 в base64 и пароль к нему (для AuthCert)
	P12     string   `json:"p12,omitempty"`
	P12Pass string   `json:"p12_pass,omitempty"`
	DNS     []string `json:"dns,omitempty"`
	// TrustCA — пользователь разрешил добавить сертификат центра в доверенные корневые Windows
	TrustCA bool `json:"trust_ca,omitempty"`
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server) == "" {
		return errors.New("не указан адрес сервера")
	}
	if !strings.Contains(c.CAPEM, "BEGIN CERTIFICATE") {
		return errors.New("нет сертификата центра сертификации")
	}
	switch c.Auth {
	case AuthEAP, "":
		if c.Username == "" || c.Password == "" {
			return errors.New("не указаны логин и пароль")
		}
	case AuthCert:
		if c.P12 == "" {
			return errors.New("нет сертификата клиента")
		}
	default:
		return fmt.Errorf("неизвестный способ входа %q", c.Auth)
	}
	return nil
}

// Instructions — текст для выдачи клиенту: данные и порядок настройки на Windows, Android и iOS.
func (c *Config) Instructions() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Сервер: %s\n", c.Server)
	if c.Auth == AuthCert {
		b.WriteString("Вход: по сертификату клиента (файл .p12)\n")
		fmt.Fprintf(&b, "Пароль к файлу .p12: %s\n", c.P12Pass)
	} else {
		fmt.Fprintf(&b, "Логин: %s\nПароль: %s\n", c.Username, c.Password)
	}
	b.WriteString("Тип VPN: IKEv2 (IPsec). Сертификат центра сертификации — файл veil-ca.crt.\n\n")
	b.WriteString("Windows. 1) Установите veil-ca.crt: двойной щелчок → «Установить сертификат» → «Локальный компьютер» → хранилище «Доверенные корневые центры сертификации». ")
	b.WriteString("2) Параметры → Сеть → VPN → «Добавить VPN»: поставщик «Windows (встроенные)», тип VPN «IKEv2», ")
	if c.Auth == AuthCert {
		b.WriteString("тип входа «Сертификат» (предварительно импортируйте .p12 в «Локальный компьютер → Личные»).\n\n")
	} else {
		b.WriteString("тип входа «Имя пользователя и пароль».\n\n")
	}
	b.WriteString("Android. Установите приложение strongSwan из магазина приложений, добавьте профиль IKEv2 ")
	if c.Auth == AuthCert {
		b.WriteString("«Сертификат клиента», импортируйте .p12; ")
	} else {
		b.WriteString("«EAP (имя пользователя/пароль)»; ")
	}
	b.WriteString("сертификат центра выберите файлом veil-ca.crt (отключите «Выбирать автоматически»).\n\n")
	b.WriteString("iOS. Откройте veil-ca.crt и установите профиль (Настройки → Основные → VPN и управление устройством; затем Основные → Об этом устройстве → Доверие сертификатам — включите). ")
	b.WriteString("Настройки → VPN → «Добавить конфигурацию»: тип IKEv2, «Удалённый ID» и «Сервер» — ")
	b.WriteString(c.Server)
	if c.Auth == AuthCert {
		b.WriteString(", аутентификация «Сертификат».\n")
	} else {
		b.WriteString(", аутентификация «Имя пользователя», логин и пароль выше.\n")
	}
	return b.String()
}
