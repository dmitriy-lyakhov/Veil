// pcapcheck: разбор UDP/443 в pcap (формат tcpdump): отпечаток первого QUIC Initial и статистика размеров/времени.
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/gaukas/clienthellod"
)

type pkt struct {
	t      float64
	in     bool
	length int
	pay    []byte
}

func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	clientIP := os.Args[2] // 4 байта через точку не нужен: определяем по порту 443
	_ = clientIP
	le := binary.LittleEndian
	magic := le.Uint32(b)
	nano := magic == 0xa1b23c4d
	link := le.Uint32(b[20:])
	off := 24
	var pk []pkt
	var t0 float64
	for off+16 <= len(b) {
		sec, frac, caplen := le.Uint32(b[off:]), le.Uint32(b[off+4:]), int(le.Uint32(b[off+8:]))
		off += 16
		d := b[off : off+caplen]
		off += caplen
		ts := float64(sec)
		if nano {
			ts += float64(frac) / 1e9
		} else {
			ts += float64(frac) / 1e6
		}
		var ip []byte
		switch link {
		case 276: // SLL2
			ip = d[20:]
		case 113: // SLL
			ip = d[16:]
		case 1:
			ip = d[14:]
		default:
			continue
		}
		if len(ip) < 28 || ip[0]>>4 != 4 || ip[9] != 17 {
			continue
		}
		ihl := int(ip[0]&0xf) * 4
		udp := ip[ihl:]
		sp, dp := binary.BigEndian.Uint16(udp), binary.BigEndian.Uint16(udp[2:])
		pay := udp[8:]
		if dp != 443 && sp != 443 {
			continue
		}
		if t0 == 0 {
			t0 = ts
		}
		pk = append(pk, pkt{t: ts - t0, in: dp == 443, length: len(pay), pay: pay})
	}
	fmt.Printf("пакетов UDP/443: %d\n", len(pk))

	// первый Initial клиента
	for _, p := range pk {
		if p.in && p.length >= 1200 {
			cip, err := clienthellod.ParseQUICCIP(p.pay)
			if err != nil {
				fmt.Println("parse initial:", err)
				break
			}
			fmt.Printf("первый пакет клиента: %d байт, первый байт 0x%02x, версия QUIC %x, DCID %d байт\n", p.length, p.pay[0], p.pay[1:5], p.pay[5])
			if cip.QCH != nil {
				j, _ := json.MarshalIndent(cip.QCH.ClientHello, "", " ")
				fmt.Println("ClientHello (расшифрован как это сделал бы DPI):\n" + string(j))
			}
			if cip.QTP != nil {
				fmt.Printf("QUIC transport params ids: %v\n", cip.QTP.QTPIDs)
			}
			break
		}
	}

	// статистика
	for _, dir := range []bool{true, false} {
		name := map[bool]string{true: "клиент→сервер", false: "сервер→клиент"}[dir]
		cnt := map[int]int{}
		mod := map[int]int{}
		var n, big int
		var bytes int
		maxl := 0
		for _, p := range pk {
			if p.in != dir {
				continue
			}
			n++
			cnt[p.length]++
			bytes += p.length
			if p.length > maxl {
				maxl = p.length
			}
			if p.length > 200 {
				big++
				mod[p.length%16]++
			}
		}
		type kv struct{ k, v int }
		var top []kv
		for k, v := range cnt {
			top = append(top, kv{k, v})
		}
		sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
		fmt.Printf("\n[%s] пакетов %d, байт %d, макс. размер %d\n", name, n, bytes, maxl)
		fmt.Print("  самые частые размеры: ")
		for i := 0; i < 6 && i < len(top); i++ {
			fmt.Printf("%d×%d (%.0f%%)  ", top[i].k, top[i].v, 100*float64(top[i].v)/float64(n))
		}
		fmt.Println()
		fmt.Printf("  пакетов >200Б: %d; распределение длины mod 16: ", big)
		var ms []kv
		for k, v := range mod {
			ms = append(ms, kv{k, v})
		}
		sort.Slice(ms, func(i, j int) bool { return ms[i].v > ms[j].v })
		for i := 0; i < 4 && i < len(ms); i++ {
			fmt.Printf("%d:%.0f%% ", ms[i].k, 100*float64(ms[i].v)/float64(big))
		}
		fmt.Println()
	}
	// ритм: пакетов в секунду и тишина
	if len(pk) > 1 {
		dur := pk[len(pk)-1].t
		maxGap, prev := 0.0, 0.0
		for _, p := range pk {
			if p.t-prev > maxGap {
				maxGap = p.t - prev
			}
			prev = p.t
		}
		fmt.Printf("\nдлительность %.1f с, средне %.0f пакетов/с, максимальная пауза %.2f с\n", dur, float64(len(pk))/dur, maxGap)
	}
	// первые 12 пакетов хендшейка
	fmt.Println("\nпервые пакеты (t, направление, размер):")
	for i := 0; i < 14 && i < len(pk); i++ {
		d := "←"
		if pk[i].in {
			d = "→"
		}
		fmt.Printf("  %.3f %s %d\n", pk[i].t, d, pk[i].length)
	}
}
