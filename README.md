# network telemetry tools

## synthetic-router

This app pretends to be a Cisco IOS-XR device for the purposes of testing with Cisco's [bigmuddy-network-telemetry-pipeline](https://github.com/cisco/bigmuddy-network-telemetry-pipeline) project

Downloading & running the code:

```shell
go get -u github.com/noelbundick/network-telemetry
cd $GOPATH/src/github.com/noelbundick/network-telemetry
go run ./synthetic-router/main.go <options>
```

Options:

```
-baseCPU int
    The base CPU percentage to simulate (default 10)
-deviceName string
    The device name to send (default "device1")
-insecure
    If set, will use an insecure connection
-seconds int
    Number of seconds to send data (default 60)
-serverAddr string
    The server host and port (default "127.0.0.1:10000")
-type string
    The data type to send. Can be one of [cpu, memory, env, routing] (default "cpu")
```