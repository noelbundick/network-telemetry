package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	telem "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go"
	generic_counters "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_infra_statsd_oper/infra_statistics/interfaces/interface/generic_counters"
	memory "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_nto_misc_oper/memory_summary/nodes/node/summary"
	interface_summary "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_pfi_im_cmd_oper/interfaces/interface_summary"
	cpu "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_wdsysmon_fd_oper/system_monitoring/cpu_utilization"
	dialout "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/mdt_grpc_dialout"
)

var (
	serverAddr     = flag.String("serverAddr", "127.0.0.1:10000", "The server host and port")
	insecure       = flag.Bool("insecure", false, "If set, will use an insecure connection")
	deviceName     = flag.String("deviceName", "device1", "The device name to send")
	baseCPU        = flag.Int("baseCPU", 20, "The base CPU percentage to simulate")
	dataType       = flag.String("type", "cpu", "The data type to send. Can be one of [cpu, memory, interface, routing, counters]")
	seconds        = flag.Int("seconds", 60, "Number of seconds to send data")
	messagesToSend = flag.Int("messagesToSend", -1, "Total number of messages to send")
	itemCount      = int32(0)
	t1             time.Time
	t2             time.Time
)

func getCPUData() (string, []byte, []byte) {
	path := "Cisco-IOS-XR-wdsysmon-fd-oper:system-monitoring/cpu-utilization"

	details := &cpu.NodeCpuUtil{
		TotalCpuOneMinute:     uint32(rand.Intn(20) + *baseCPU),
		TotalCpuFiveMinute:    uint32(rand.Intn(10) + *baseCPU),
		TotalCpuFifteenMinute: uint32(rand.Intn(5) + *baseCPU),
	}
	detailBytes, _ := proto.Marshal(details)

	keys := &cpu.NodeCpuUtil_KEYS{
		NodeName: "0/0/CPU0",
	}
	keyBytes, _ := proto.Marshal(keys)

	return path, detailBytes, keyBytes
}

func getGenericCountersData() (string, []byte, []byte) {
	path := "Cisco-IOS-XR-infra-statsd-oper:infra-statistics/interfaces/interface/latest/generic-counters"

	details := &generic_counters.IfstatsbagGeneric{
		Applique:                       0,
		AvailabilityFlag:               0,
		BroadcastPacketsReceived:       0,
		BroadcastPacketsSent:           0,
		BytesReceived:                  0,
		BytesSent:                      0,
		CarrierTransitions:             0,
		CrcErrors:                      0,
		FramingErrorsReceived:          0,
		GiantPacketsReceived:           0,
		InputAborts:                    0,
		InputDrops:                     0,
		InputErrors:                    0,
		InputIgnoredPackets:            0,
		InputOverruns:                  0,
		InputQueueDrops:                0,
		LastDataTime:                   1557771568,
		LastDiscontinuityTime:          1555302325,
		MulticastPacketsReceived:       0,
		MulticastPacketsSent:           0,
		OutputBufferFailures:           0,
		OutputBuffersSwappedOut:        0,
		OutputDrops:                    0,
		OutputErrors:                   0,
		OutputQueueDrops:               0,
		OutputUnderruns:                0,
		PacketsReceived:                0,
		PacketsSent:                    0,
		ParityPacketsReceived:          0,
		Resets:                         0,
		RuntPacketsReceived:            0,
		SecondsSinceLastClearCounters:  0,
		SecondsSincePacketReceived:     4294967295,
		SecondsSincePacketSent:         4294967295,
		ThrottledPacketsReceived:       0,
		UnknownProtocolPacketsReceived: 0,
	}
	detailBytes, _ := proto.Marshal(details)

	keys := &generic_counters.IfstatsbagGeneric_KEYS{
		InterfaceName: "Null0",
	}
	keyBytes, _ := proto.Marshal(keys)

	return path, detailBytes, keyBytes
}

func getInterfaceData() (string, []byte, []byte) {
	path := "Cisco-IOS-XR-pfi-im-cmd-oper:interfaces/interface-summary"

	details := &interface_summary.ImIfSummaryInfo{
		InterfaceCounts: &interface_summary.ImIfGroupCountsSt{
			AdminDownInterfaceCount: 0,
			DownInterfaceCount:      20,
			InterfaceCount:          20,
			UpInterfaceCount:        0,
		},
		InterfaceTypeList: []*interface_summary.ImIfTypeSummarySt{
			&interface_summary.ImIfTypeSummarySt{
				InterfaceCounts:          nil,
				InterfaceTypeDescription: "GigabitEthernet",
				InterfaceTypeName:        "IFT_GETHERNET",
			},
		},
	}
	detailBytes, _ := proto.Marshal(details)

	keys := &interface_summary.ImIfSummaryInfo_KEYS{}
	keyBytes, _ := proto.Marshal(keys)

	return path, detailBytes, keyBytes
}

func getMemoryData() (string, []byte, []byte) {
	path := "Cisco-IOS-XR-nto-misc-oper:memory-summary/nodes/node/summary"

	details := &memory.NodeMemInfo{
		BootRamSize:           0,
		FlashSystem:           0,
		FreeApplicationMemory: uint64(rand.Intn(1000000000) + 4000000000),
		FreePhysicalMemory:    4902529024,
		ImageMemory:           4194304,
		IoMemory:              0,
		PageSize:              4096,
		RamMemory:             11758403584,
		ReservedMemory:        0,
		SystemRamMemory:       11758403584,
	}
	detailBytes, _ := proto.Marshal(details)

	keys := &memory.NodeMemInfo_KEYS{
		NodeName: "0/RSP0/CPU0",
	}
	keyBytes, _ := proto.Marshal(keys)

	return path, detailBytes, keyBytes
}

func getRoutingData() (string, []byte, []byte) {
	log.Fatal("Not implemented")
	return "", nil, nil
}

func sendMessages(conn *grpc.ClientConn, c chan os.Signal) {
	client := dialout.NewGRPCMdtDialoutClient(conn)
	stream, err := client.MdtDialout(context.Background())

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive: %v", err)
			}
			log.Printf("Got message: %v", in)
		}
	}()

	i := 0
	for {
		var path string
		var detailBytes []byte
		var keyBytes []byte

		switch *dataType {
		case "cpu":
			path, detailBytes, keyBytes = getCPUData()
		case "interface":
			path, detailBytes, keyBytes = getInterfaceData()
		case "memory":
			path, detailBytes, keyBytes = getMemoryData()
		case "routing":
			path, detailBytes, keyBytes = getRoutingData()
		case "counters":
			path, detailBytes, keyBytes = getGenericCountersData()
		}

		now := time.Now()
		ts := uint64(now.UnixNano() / 1000000)
		t := &telem.Telemetry{
			NodeId:              &telem.Telemetry_NodeIdStr{NodeIdStr: *deviceName},
			Subscription:        &telem.Telemetry_SubscriptionIdStr{SubscriptionIdStr: "test"},
			EncodingPath:        path,
			CollectionId:        uint64(i),
			CollectionStartTime: ts,
			MsgTimestamp:        ts,
			CollectionEndTime:   ts,
			DataGpb: &telem.TelemetryGPBTable{Row: []*telem.TelemetryRowGPB{
				&telem.TelemetryRowGPB{
					Timestamp: ts,
					Content:   detailBytes,
					Keys:      keyBytes,
				},
			}},
		}
		b, _ := proto.Marshal(t)

		ev := &dialout.MdtDialoutArgs{
			ReqId: int64(i),
			Data:  b,
		}

		if err = stream.Send(ev); err != nil {
			log.Fatalf("%v.Send(%v) = %v", stream, ev, err)
		}

		atomic.AddInt32(&itemCount, int32(1))
		i++
		time.Sleep(1 * time.Millisecond)

		if *messagesToSend > 0 && i > *messagesToSend {
			fmt.Println("message limit reached")
			c <- os.Interrupt
		}
	}
}

func main() {
	flag.Parse()

	var opts []grpc.DialOption
	if *insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	}

	fmt.Printf("Connecting to: %v\n", *serverAddr)
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("could not connect: %v", serverAddr)
	}
	defer conn.Close()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			t2 = time.Now()

			diff := t2.Sub(t1)
			// sig is a ^C, handle it
			fmt.Printf("events: %v over %v\n", itemCount, diff)
			rps := float64(itemCount) / diff.Seconds()
			fmt.Printf("RPS: %v\n", rps)
			os.Exit(0)
		}
	}()

	timer := time.NewTimer(time.Duration(*seconds) * time.Second)
	go func() {
		<-timer.C
		c <- os.Interrupt
	}()

	t1 = time.Now()
	sendMessages(conn, c)
}
