//go:build linux

package lagran

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coreos/go-iptables/iptables"
	"github.com/florianl/go-nfqueue"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/panjf2000/ants/v2"

	"move86go/core"
	"move86go/core/logx"
)

var HttpPort = "80,443,8888,9999"

const (
	EnableSynAck       = true
	EnableAck          = true
	EnablePshAck       = true
	EnableFinAck       = true
	QueueBalanceSynAck = "1000:4000"
	QueueBalanceAck    = "10000:13000"
	QueueBalancePshAck = "45000:46000"
	QueueBalanceFinAck = "50000:51000"

	MaxQueueLen  = 10000
	WriteTimeout = 15 * time.Millisecond
)

var WindowSizeOfSynAck = 5
var WindowSizeOfAck = 5
var WindowSizeOfPshAck = 5
var WindowSizeOfFinAck = 5

var EnableRandomWindow = true

// var WindowJitter = 0
var WindowJitter = 2
var httpPortSet map[uint16]struct{}

var M86Debug = true

func SetDebug(d bool) {
	M86Debug = d
}

func Run() {
	rand.Seed(time.Now().UnixNano())

	setIptable(HttpPort)
	httpPortSet = buildPortSet(HttpPort)
	var wg sync.WaitGroup
	if EnableSynAck {
		startQueues(QueueBalanceSynAck, &wg)
	}
	if EnableAck {
		startQueues(QueueBalanceAck, &wg)
	}
	if EnablePshAck {
		startQueues(QueueBalancePshAck, &wg)
	}
	if EnableFinAck {
		startQueues(QueueBalanceFinAck, &wg)
	}
}

func setIptable(sport string) {
	ipt, err := iptables.New()
	if err != nil {
		logx.Error("[lagran service] Iptabels new error:%v\n", err)
		return
	}
	if EnableSynAck {
		addNFQueueRule(ipt, sport, "SYN,ACK", QueueBalanceSynAck)
	}
	if EnableAck {
		addNFQueueRule(ipt, sport, "ACK", QueueBalanceAck)
	}
	if EnablePshAck {
		addNFQueueRule(ipt, sport, "PSH,ACK", QueueBalancePshAck)
	}
	if EnableFinAck {
		addNFQueueRule(ipt, sport, "FIN,ACK", QueueBalanceFinAck)
	}
}

func UnsetIptable(sport string) {
	ipt, err := iptables.New()
	if err != nil {
		logx.Error("[lagran service] Iptabels new error:%v", err)
		return
	}
	if EnableSynAck {
		rmNFQueueRule(ipt, sport, "SYN,ACK", QueueBalanceSynAck)
	}
	if EnableAck {
		rmNFQueueRule(ipt, sport, "ACK", QueueBalanceAck)
	}
	if EnablePshAck {
		rmNFQueueRule(ipt, sport, "PSH,ACK", QueueBalancePshAck)
	}
	if EnableFinAck {
		rmNFQueueRule(ipt, sport, "FIN,ACK", QueueBalanceFinAck)
	}
}

func packetHandle(queueNum int) {
	nfqconfig := nfqueue.Config{
		NfQueue:      uint16(queueNum),
		MaxPacketLen: 0xFFFF,
		MaxQueueLen:  MaxQueueLen,
		Copymode:     nfqueue.NfQnlCopyPacket,
		WriteTimeout: WriteTimeout,
	}

	nf, err := nfqueue.Open(&nfqconfig)
	if err != nil {
		logx.Error("[lagran] could not open nfqueue socket:", err)
		return
	}

	defer nf.Close()

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	fn := func(a nfqueue.Attribute) int {
		id := *a.PacketID
		packet := gopacket.NewPacket(*a.Payload, layers.LayerTypeIPv4, gopacket.Default)
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
				tcp, _ := tcpLayer.(*layers.TCP)
				sport := uint16(tcp.SrcPort)
				if _, ok := httpPortSet[sport]; ok {
					var ok1 = EnableSynAck && tcp.SYN && tcp.ACK
					var ok2 = EnableAck && tcp.ACK && !tcp.PSH && !tcp.FIN && !tcp.SYN && !tcp.RST
					var ok3 = EnablePshAck && tcp.PSH && tcp.ACK
					var ok4 = EnableFinAck && tcp.FIN && tcp.ACK
					size, modify := computeWindowSize(tcp, ok1, ok2, ok3, ok4)
					if !modify {
						err := nf.SetVerdict(id, nfqueue.NfAccept)
						if err != nil {
							logx.Error("[lagran] SetVerdict error: %v\n", err)
						}
						return 0
					}

					windowSize := size
					if EnableRandomWindow {
						windowSize = adjustWindowSize(windowSize, WindowJitter)
					}
					tcp.Window = windowSize

					var err error
					if nl, ok := ipLayer.(gopacket.NetworkLayer); ok {
						err = tcp.SetNetworkLayerForChecksum(nl)
					} else {
						err = tcp.SetNetworkLayerForChecksum(packet.NetworkLayer())
					}
					if err != nil {
						logx.Error("[lagran] SetNetworkLayerForChecksum error: %v\n", err)
					}
					buffer := gopacket.NewSerializeBuffer()
					options := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
					if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
						logx.Error("[lagran] SerializePacket error: %v\n", err)
					}
					packetBytes := buffer.Bytes()
					if M86Debug {

						if ip4, ok := ipLayer.(*layers.IPv4); ok {
							logx.Debug("[lagran] srcIP:", ip4.SrcIP.String())
						}

						logx.Debug("[lagran] windowSize:", windowSize)

						dump := hexDump(packetBytes, 128)
						logx.Debug("[lagran] packetBytes len:", len(packetBytes))
						logx.Debug("[lagran] packetBytes dump:\n" + dump)
					}

					err = nf.SetVerdictModPacket(id, nfqueue.NfAccept, packetBytes)
					if err != nil {
						logx.Error("[lagran] SetVerdictModPacket error: %v\n", err)
					}
					return 0
				}
				err := nf.SetVerdict(id, nfqueue.NfAccept)
				if err != nil {
					logx.Error("[lagran] SetVerdict error: %v\n", err)
				}
				return 0
			}
		}

		err := nf.SetVerdict(id, nfqueue.NfAccept)
		if err != nil {
			logx.Error("[lagran] SetVerdict error: %v\n", err)
		}
		return 0
	}
	err = nf.RegisterWithErrorFunc(ctx, fn, func(e error) int {
		if e != nil {
			logx.Error("[lagran] RegisterWithErrorFunc Error:%v\n", e)
		}
		return 0
	})
	if err != nil {
		logx.Error("[lagran] error: %v\n", err)
	}
	<-ctx.Done()
}

func startQueues(queueRange string, wg *sync.WaitGroup) {
	queueStart, queueEnd, poolNum := getQueueBalance(queueRange)
	p, _ := ants.NewPoolWithFunc(poolNum, func(i interface{}) {
		packetHandle(i.(int))
		wg.Done()
	})
	defer p.Release()
	for i := queueStart; i < queueEnd; i++ {
		wg.Add(1)
		_ = p.Invoke(int(i))
	}
}

func addNFQueueRule(ipt *iptables.IPTables, sport, tcpFlags, queueRange string) {
	_ = ipt.AppendUnique("filter", "OUTPUT", "-p", "tcp", "-m", "multiport", "--sport", sport, "--tcp-flags", "SYN,RST,ACK,FIN,PSH", tcpFlags, "-j", "NFQUEUE", "--queue-balance", queueRange)
}

func rmNFQueueRule(ipt *iptables.IPTables, sport, target, queueBalance string) {
	_ = ipt.Delete("filter", "OUTPUT", "-p", "tcp", "-m", "multiport", "--sport", sport, "--tcp-flags", "SYN,RST,ACK,FIN,PSH", target, "-j", "NFQUEUE", "--queue-balance", queueBalance)
}

func getQueueBalance(val string) (int, int, int) {
	vals := strings.Split(val, ":")
	val1 := core.Str2int(vals[0])
	val2 := core.Str2int(vals[1])
	val3 := val2 - val1
	return val1, val2, val3
}

func buildPortSet(s string) map[uint16]struct{} {
	m := make(map[uint16]struct{})
	parts := strings.Split(s, ",")
	for _, p := range parts {
		v := uint16(core.Str2int(p))
		m[v] = struct{}{}
	}
	return m
}

func computeWindowSize(tcp *layers.TCP, ok1, ok2, ok3, ok4 bool) (uint16, bool) {
	if ok1 {
		return uint16(WindowSizeOfSynAck), true
	}
	if ok2 {
		return uint16(WindowSizeOfAck), true
	}
	if ok3 {
		return uint16(WindowSizeOfPshAck), true
	}
	if ok4 {
		return uint16(WindowSizeOfFinAck), true
	}
	return 0, false
}

func adjustWindowSize(base uint16, jitter int) uint16 {
	if jitter <= 0 {
		return base
	}
	b := int(base)
	min := b - jitter
	if min < 0 {
		min = 0
	}
	max := b + jitter
	if max > 65535 {
		max = 65535
	}
	return uint16(min + rand.Intn(max-min+1))
}

func asciiPreview(b []byte, sample int) string {
	n := sample
	if len(b) < n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		c := b[i]
		if c >= 32 && c <= 126 {
			out[i] = c
		} else {
			out[i] = '.'
		}
	}
	if n < len(b) {
		return string(out) + "..."
	}
	return string(out)
}

func utf8Preview(b []byte, sample int) string {
	if sample <= 0 {
		return ""
	}
	n := sample
	if len(b) < n {
		n = len(b)
	}
	var sb strings.Builder
	i := 0
	for i < n {
		r, size := utf8.DecodeRune(b[i:n])
		if r == utf8.RuneError && size == 1 {
			sb.WriteByte('.')
			i++
			continue
		}
		if unicode.IsPrint(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('.')
		}
		i += size
	}
	if n < len(b) {
		sb.WriteString("...")
	}
	return sb.String()
}

func hexDump(b []byte, sample int) string {
	if sample <= 0 {
		return ""
	}
	n := sample
	if len(b) < n {
		n = len(b)
	}
	var sb strings.Builder
	width := 16
	for i := 0; i < n; i += width {
		end := i + width
		if end > n {
			end = n
		}
		sb.WriteString(fmt.Sprintf("%04x: ", i))
		for j := i; j < i+width; j++ {
			if j < end {
				sb.WriteString(fmt.Sprintf("%02x ", b[j]))
			} else {
				sb.WriteString("   ")
			}
		}
		sb.WriteString(" ")
		for j := i; j < end; j++ {
			c := b[j]
			if c >= 32 && c <= 126 {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		if end < n {
			sb.WriteByte('\n')
		}
	}
	if n < len(b) {
		sb.WriteString("\n...")
	}
	return sb.String()
}
