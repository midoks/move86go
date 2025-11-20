//go:build linux

package lagran

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/florianl/go-nfqueue"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/panjf2000/ants/v2"

	"move86go/core"
	"move86go/core/logx"
)

var HttpPort = "80,443,8888"

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

var EnableRandomWindow = false
var WindowJitter = 0

func Run() {
	rand.Seed(time.Now().UnixNano())

	setIptable(HttpPort)
	var wg sync.WaitGroup
	if EnableSynAck {
		queueStart, queueEnd, poolNum := getQueueBalance(QueueBalanceSynAck)
		p1, _ := ants.NewPoolWithFunc(poolNum, func(i interface{}) {
			packetHandle(i.(int))
			wg.Done()
		})
		defer p1.Release()
		for i := queueStart; i < queueEnd; i++ {
			wg.Add(1)
			_ = p1.Invoke(int(i))
		}
	}
	if EnableAck {
		queueStart, queueEnd, poolNum := getQueueBalance(QueueBalanceAck)
		p2, _ := ants.NewPoolWithFunc(poolNum, func(i interface{}) {
			packetHandle(i.(int))
			wg.Done()
		})
		defer p2.Release()
		for i := queueStart; i < queueEnd; i++ {
			wg.Add(1)
			_ = p2.Invoke(int(i))
		}
	}
	if EnablePshAck {
		queueStart, queueEnd, poolNum := getQueueBalance(QueueBalancePshAck)
		p3, _ := ants.NewPoolWithFunc(poolNum, func(i interface{}) {
			packetHandle(i.(int))
			wg.Done()
		})
		defer p3.Release()
		for i := queueStart; i < queueEnd; i++ {
			wg.Add(1)
			_ = p3.Invoke(int(i))
		}
	}
	if EnableFinAck {
		queueStart, queueEnd, poolNum := getQueueBalance(QueueBalanceFinAck)
		p4, _ := ants.NewPoolWithFunc(poolNum, func(i interface{}) {
			packetHandle(i.(int))
			wg.Done()
		})
		defer p4.Release()
		for i := queueStart; i < queueEnd; i++ {
			wg.Add(1)
			_ = p4.Invoke(int(i))
		}
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
				ports := strings.Split(HttpPort, ",")
				sport := strings.TrimPrefix(tcp.SrcPort.String(), "Port(")
				sport = strings.TrimSuffix(sport, ")")
				if contains(ports, sport) {
					var ok1 = EnableSynAck && tcp.SYN && tcp.ACK
					var ok2 = EnableAck && tcp.ACK && !tcp.PSH && !tcp.FIN && !tcp.SYN && !tcp.RST
					var ok3 = EnablePshAck && tcp.PSH && tcp.ACK
					var ok4 = EnableFinAck && tcp.FIN && tcp.ACK
					var windowSize uint16
					if ok1 || ok2 || ok3 || ok4 {
						if ok1 {
							windowSize = uint16(WindowSizeOfSynAck)
						}
						if ok2 {
							windowSize = uint16(WindowSizeOfAck)
						}
						if ok3 {
							windowSize = uint16(WindowSizeOfPshAck)
						}
						if ok4 {
							windowSize = uint16(WindowSizeOfFinAck)
						}
						if EnableRandomWindow {
							base := int(windowSize)
							jitter := WindowJitter
							if jitter > 0 {
								min := base - jitter
								if min < 0 {
									min = 0
								}
								max := base + jitter
								windowSize = uint16(min + rand.Intn(max-min+1))
							}
						}
						tcp.Window = windowSize
						err := tcp.SetNetworkLayerForChecksum(packet.NetworkLayer())
						if err != nil {
							logx.Error("[lagran] SetNetworkLayerForChecksum error: %v\n", err)
						}
						buffer := gopacket.NewSerializeBuffer()
						options := gopacket.SerializeOptions{
							ComputeChecksums: true,
							FixLengths:       true,
						}
						if err := gopacket.SerializePacket(buffer, options, packet); err != nil {
							logx.Error("[lagran] SerializePacket error: %v\n", err)
						}
						packetBytes := buffer.Bytes()
						err = nf.SetVerdictModPacket(id, nfqueue.NfAccept, packetBytes)
						if err != nil {
							logx.Error("[lagran] SetVerdictModified error: %v\n", err)
						}
						return 0
					}
					err := nf.SetVerdict(id, nfqueue.NfAccept)
					if err != nil {
						logx.Error("[lagran] SetVerdictModified error: %v\n", err)
					}
					return 0
				}
				err := nf.SetVerdict(id, nfqueue.NfAccept)
				if err != nil {
					logx.Error("[lagran] SetVerdictModified error: %v\n", err)
				}
				return 0
			}
		}

		err := nf.SetVerdict(id, nfqueue.NfAccept)
		if err != nil {
			logx.Error("[lagran] SetVerdictModified error: %v\n", err)
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

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
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
