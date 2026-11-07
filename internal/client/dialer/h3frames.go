package dialer

import (
	quic "github.com/refraction-networking/uquic"
)

// safeFrames оборачивает построитель кадров Initial-пакета из пресета uquic. QUICRandomFrames.Build паникует
// («crypto/rand: argument to Int is <= 0»), когда повторный (probe) Initial-пакет собирается без данных CRYPTO или с очень короткими:
// ошибка в самой библиотеке, а паника происходит в её горутине, и обычным recover снаружи её не поймать — падало всё приложение.
// При сбое строится обычный детерминированный набор кадров: один CRYPTO на все данные (при пустых данных — PING).
type safeFrames struct{ quic.QUICFrameBuilder }

func (s safeFrames) Build(cryptoData []byte) (out []byte, err error) {
	if len(cryptoData) >= 2 {
		defer func() {
			if recover() != nil {
				out, err = plainFrames(cryptoData)
			}
		}()
		return s.QUICFrameBuilder.Build(cryptoData)
	}
	return plainFrames(cryptoData)
}

func plainFrames(cryptoData []byte) ([]byte, error) {
	if len(cryptoData) == 0 {
		return quic.QUICFrames{quic.QUICFramePing{}}.Build(nil)
	}
	return quic.QUICFrames{quic.QUICFrameCrypto{Offset: 0, Length: len(cryptoData)}}.Build(cryptoData)
}

// guardFrameBuilder ставит безопасную обёртку на построитель кадров Initial-пакета спецификации.
func guardFrameBuilder(spec *quic.QUICSpec) {
	if b := spec.InitialPacketSpec.FrameBuilder; b != nil {
		spec.InitialPacketSpec.FrameBuilder = safeFrames{b}
	}
}
