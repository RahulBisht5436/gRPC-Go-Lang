# `Greet Request and Response Streams/` project — explanation index (Bidirectional Streaming)

Iss folder me har source file ka detailed Hinglish explanation hai. Ye project **Bidirectional Streaming gRPC** ka hai — client aur server dono independently messages stream karte hain. Sequence me padhne ke liye:

1. **[bideirectionalStreams.proto.md](./bideirectionalStreams.proto.md)** — Sab kuch yahin se start hota hai. `stream` keyword **dono sides** pe hai.
2. **[bideirectionalStreams.pb.go.md](./bideirectionalStreams.pb.go.md)** — `protoc-gen-go` ne `GreetRequest` aur `GreetResponse` ke Go structs banaye.
3. **[bideirectionalStreams_grpc.pb.go.md](./bideirectionalStreams_grpc.pb.go.md)** — `protoc-gen-go-grpc` ne bidirectional interfaces banaye.
4. **[server-main.go.md](./server-main.go.md)** — Server bootstrap + `GreetEveryone` handler (`Recv → Send` echo loop).
5. **[client-main.go.md](./client-main.go.md)** — Client jo 2 goroutines me Send + Recv parallel chalata hai, `waitc` channel se sync.
6. **[go.mod.md](./go.mod.md)** — Module name aur dependencies.

## Mental model — ek baar dekh lo

```
                                 bideirectionalStreams.proto
                              (rpc GreetEveryone
                               (stream Req) returns (stream Res))
                                      |
                                      |  protoc + 2 plugins
                                      v
                  +-------------------+-------------------+
                  |                                       |
        bideirectionalStreams.pb.go        bideirectionalStreams_grpc.pb.go
        (messages: structs)         (BidiStreamingClient + BidiStreamingServer)
                  |                                       |
                  +-------------------+-------------------+
                                      |
                                      |  imported as `pb`
                                      v
              +-----------------------+-----------------------+
              |                                               |
       server/main.go                                  client/main.go
       (single goroutine:                              (TWO goroutines:
         for { Recv()                                    main:       Send + CloseSend
               Send() })                                 goroutine:  Recv loop)
              |                                               |
              +----------------- HTTP/2 ---------------------+
              (Both streams flowing independently,
               ek hi HTTP/2 stream pe DATA frames dono directions me)
```

Ek line summary: **proto me dono sides pe `stream` likho → server me `Recv → Send` echo → client me 2 goroutines (Send + Recv) parallel, sync via `waitc` channel**.

## Char modes ki final comparison

| Aspect | Unary | Server Streaming | Client Streaming | **Bidirectional (current)** |
|---|---|---|---|---|
| Proto | `(Req) returns (Res)` | `(Req) returns (stream Res)` | `(stream Req) returns (Res)` | `(stream Req) returns (stream Res)` |
| Server handler | `(ctx, *Req) (*Res, err)` | `(*Req, SSS[Res]) err` | `(CSS[Req,Res]) err` | `(BSS[Req,Res]) err` |
| Server "respond" | `return &Res, nil` | `Send` × N + `return nil` | `return SendAndClose(&Res)` | `Send` × N + `return nil` |
| Client call | `client.X(ctx, req)` | `client.X(ctx, req)` | `client.X(ctx)` | `client.X(ctx)` |
| Client "transmit" | request param | request param | `Send` × N + `CloseAndRecv` | `Send` × N + `CloseSend` |
| Client "receive" | returned `*Res` | `Recv` loop | `CloseAndRecv` | `Recv` loop |
| Client goroutines | 1 | 1 | 1 | **2 (Send + Recv parallel)** |
| EOF lands on | n/a | client | server | **both** |
| Wire | 1 req + 1 res | 1 req + N res | N req + 1 res | N req + N res |

> **Stream keyword ka rule**: jis side `stream` likha hai, vahi side **multiple Send** karta hai. Bidirectional me dono jagah `stream` hai → dono sides Send karte hain.
