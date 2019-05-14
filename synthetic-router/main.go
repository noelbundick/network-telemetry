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
	memory "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_nto_misc_oper/memory_summary/nodes/node/summary"
	cpu "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/cisco_ios_xr_wdsysmon_fd_oper/system_monitoring/cpu_utilization"
	dialout "github.com/cisco/bigmuddy-network-telemetry-proto/proto_go/mdt_grpc_dialout"
)

var (
	serverAddr = flag.String("serverAddr", "127.0.0.1:10000", "The server host and port")
	insecure   = flag.Bool("insecure", false, "If set, will use an insecure connection")
	deviceName = flag.String("deviceName", "device1", "The device name to send")
	baseCPU    = flag.Int("baseCPU", 10, "The base CPU percentage to simulate")
	dataType   = flag.String("type", "cpu", "The data type to send. Can be one of [cpu, memory, env, routing]")
	seconds    = flag.Int("seconds", 60, "Number of seconds to send data")
	itemCount  = int32(0)
	t1         time.Time
	t2         time.Time
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

func getEnvData() (string, []byte, []byte) {
	log.Fatal("Not implemented")
	return "", nil, nil
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

func sendMessages(conn *grpc.ClientConn) {
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

	var events []*dialout.MdtDialoutArgs
	itemsPerBatch := 50
	for i := 0; i < itemsPerBatch; i++ {

		var path string
		var detailBytes []byte
		var keyBytes []byte

		switch *dataType {
		case "cpu":
			path, detailBytes, _ = getCPUData()
		case "env":
			path, detailBytes, _ = getEnvData()
		case "memory":
			path, detailBytes, _ = getMemoryData()
		case "routing":
			path, detailBytes, _ = getRoutingData()
		}

		now := time.Now()
		t := &telem.Telemetry{
			NodeId:              &telem.Telemetry_NodeIdStr{NodeIdStr: *deviceName},
			Subscription:        &telem.Telemetry_SubscriptionIdStr{SubscriptionIdStr: "test"},
			EncodingPath:        path,
			CollectionId:        uint64(i),
			CollectionStartTime: uint64(now.UnixNano() / 1000000),
			MsgTimestamp:        uint64(now.UnixNano() / 1000000),
			CollectionEndTime:   uint64(now.UnixNano() / 1000000),
			DataGpb: &telem.TelemetryGPBTable{Row: []*telem.TelemetryRowGPB{
				&telem.TelemetryRowGPB{
					Timestamp: uint64(time.Now().UnixNano() / 1000000),
					Content:   detailBytes,
					Keys:      keyBytes,
				},
			}},
		}
		b, _ := proto.Marshal(t)
		events = append(events, &dialout.MdtDialoutArgs{
			ReqId: int64(i),
			Data:  b,
		})
	}
	for {
		if err != nil {
			log.Fatalf("could not open stream: %v", err)
		}

		for _, ev := range events {
			if err = stream.Send(ev); err != nil {
				log.Fatalf("%v.Send(%v) = %v", stream, ev, err)
			}
			atomic.AddInt32(&itemCount, int32(1))
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
	sendMessages(conn)
}
