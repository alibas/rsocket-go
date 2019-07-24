# rsocket-go
![logo](./logo.jpg)

[![Travis (.org)](https://img.shields.io/travis/rsocket/rsocket-go.svg)]((https://img.shields.io/travis/rsocket/rsocket-go.svg))
[![Slack](https://img.shields.io/badge/slack-rsocket--go-blue.svg)](https://rsocket.slack.com/messages/C9VGZ5MV3)
[![GoDoc](https://godoc.org/github.com/rsocket/rsocket-go?status.svg)](https://godoc.org/github.com/rsocket/rsocket-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/rsocket/rsocket-go)](https://goreportcard.com/report/github.com/rsocket/rsocket-go)
[![License](https://img.shields.io/github/license/rsocket/rsocket-go.svg)](https://github.com/rsocket/rsocket-go/blob/master/LICENSE)
[![GitHub Release](https://img.shields.io/github/release-pre/rsocket/rsocket-go.svg)](https://github.com/rsocket/rsocket-go/releases)

rsocket-go is an implementation of the [RSocket](http://rsocket.io/) protocol in Go. It is still under development, APIs are unstable and maybe change at any time until release of v1.0.0. **Please do not use it in a production environment**.

## Features
 - Design For Golang.
 - Thin [reactive-streams](http://www.reactive-streams.org/) implementation.
 - Simulate Java SDK API.

## Getting started

> Start an echo server
```go
package main

import (
	"context"

	"github.com/rsocket/rsocket-go"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx/mono"
)

func main() {
	err := rsocket.Receive().
		Resume().
		Fragment(1024).
		Acceptor(func(setup payload.SetupPayload, sendingSocket rsocket.CloseableRSocket) rsocket.RSocket {
			// bind responder
			return rsocket.NewAbstractSocket(
				rsocket.RequestResponse(func(msg payload.Payload) mono.Mono {
					return mono.Just(msg)
				}),
			)
		}).
		Transport("tcp://127.0.0.1:7878").
		Serve(context.Background())
	panic(err)
}
```

> Connect to echo server

```go
package main

import (
	"context"
	"log"

	"github.com/rsocket/rsocket-go"
	"github.com/rsocket/rsocket-go/payload"
)

func main() {
	// Connect to server
	cli, err := rsocket.Connect().
		Resume().
		Fragment(1024).
		SetupPayload(payload.NewString("Hello", "World")).
		Transport("tcp://127.0.0.1:7878").
		Start(context.Background())
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	// Send request
	result, err := cli.RequestResponse(payload.NewString("你好", "世界")).Block(context.Background())
	if err != nil {
		panic(err)
	}
	log.Println("response:", result)
}
```

> NOTICE: more server examples are [Here](cmd/echo/echo.go)

## Advanced

### Load Balance

Basic load balance feature, see [here](./balancer).

### Reactor API

`Mono` and `Flux` are two parts of Reactor API. They are based on my another project [reactor-go](https://github.com/jjeffcaii/reactor-go/).

#### Mono

`Mono` completes successfully by emitting an element, or with an error.
Here is a tiny example:

```go
package main

import (
	"context"
	"fmt"

	"github.com/jjeffcaii/reactor-go/scheduler"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/rsocket/rsocket-go/rx/mono"
)

func main() {
	// Create a Mono using Just.
	m := mono.Just(payload.NewString("Hello World!", "text/plain"))

	// More create
	//m := mono.Create(func(i context.Context, sink mono.Sink) {
	//	sink.Success(payload.NewString("Hello World!", "text/plain"))
	//})

	done := make(chan struct{})

	m.
		DoFinally(func(s rx.SignalType) {
			close(done)
		}).
		DoOnSuccess(func(input payload.Payload) {
			// Handle and consume payload.
			// Do something here...
			fmt.Println("bingo:", input)
		}).
		SubscribeOn(scheduler.Elastic()).
		Subscribe(context.Background())

	<-done
}
```

### Flux

`Flux` emits 0 to N elements, and then completes (successfully or with an error).
Here is tiny example:

```go
package main

import (
	"context"
	"fmt"

	flxx "github.com/jjeffcaii/reactor-go/flux"
	"github.com/rsocket/rsocket-go/extension"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx/flux"
)

func main() {
	// Create a Flux and produce 10 elements.
	f := flux.Create(func(ctx context.Context, sink flux.Sink) {
		for i := 0; i < 10; i++ {
			sink.Next(payload.NewString(fmt.Sprintf("Hello@%d", i), extension.TextPlain.String()))
		}
		sink.Complete()
	})

	// Or use Just.
	//f := flux.Just(
	//	payload.NewString("foo", extension.TextPlain.String()),
	//	payload.NewString("bar", extension.TextPlain.String()),
	//	payload.NewString("qux", extension.TextPlain.String()),
	//)

	f.
		DoOnNext(func(elem payload.Payload) {
			// Handle and consume elements
			// Do something here...
			fmt.Println("bingo:", elem)
		}).
		Subscribe(context.Background())

	// Or you can use Raw reactor-go API. :-D
	f2 := flux.Raw(flxx.Range(0, 10).Map(func(i interface{}) interface{} {
		return payload.NewString(fmt.Sprintf("Hello@%d", i.(int)), extension.TextPlain.String())
	}))
	f2.
		DoOnNext(func(input payload.Payload) {
			fmt.Println("bingo:", input)
		}).
		BlockLast(context.Background())
}
```

#### Backpressure & RequestN

`Flux` support **backpressure**.

You can call func `Request` in `Subscription` or use `LimitRate` before subscribe.

```go
package main

import (
	"context"
	"fmt"

	"github.com/rsocket/rsocket-go/extension"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/rsocket/rsocket-go/rx/flux"
)

func main() {
	// Here is an example which consume Payload one by one.
	f := flux.Create(func(ctx context.Context, s flux.Sink) {
		for i := 0; i < 5; i++ {
			s.Next(payload.NewString(fmt.Sprintf("Hello@%d", i), extension.TextPlain.String()))
		}
		s.Complete()
	})

	var su rx.Subscription
	f.
		DoOnRequest(func(n int) {
			fmt.Printf("requesting next %d element......\n", n)
		}).
		Subscribe(
			context.Background(),
			rx.OnSubscribe(func(s rx.Subscription) {
				// Init Request 1 element.
				su = s
				su.Request(1)
			}),
			rx.OnNext(func(elem payload.Payload) {
				// Consume element, do something...
				fmt.Println("bingo:", elem)
				// Request for next one manually.
				su.Request(1)
			}),
		)
}

```

#### Dependencies
 - [reactor-go](https://github.com/jjeffcaii/reactor-go)
 - [bytebufferpool](https://github.com/valyala/bytebufferpool)
 - [testify](https://github.com/stretchr/testify)
 - [websocket](https://github.com/gorilla/websocket)

### TODO

#### Transport
 - [x] TCP
 - [x] Websocket

#### Duplex Socket
 - [x] MetadataPush
 - [x] RequestFNF
 - [x] RequestResponse
 - [x] RequestStream
 - [x] RequestChannel

##### Others
 - [x] Resume
 - [x] Keepalive
 - [x] Fragmentation
 - [x] Thin Reactor
 - [x] Cancel
 - [x] Error
 - [x] Flow Control: RequestN
 - [ ] Flow Control: Lease
 - [x] Load Balance
