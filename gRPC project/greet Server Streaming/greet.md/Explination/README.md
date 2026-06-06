# `greet/` project — explanation index (Server Streaming)

Iss folder me har source file ka detailed Hinglish explanation hai. Ye project **Server-Side Streaming gRPC** ka hai — ek request bhejo, server stream me multiple responses bheje. Sequence me padhne ke liye:

1. **[greet.proto.md](./greet.proto.md)** — Sab kuch yahin se start hota hai. Service aur messages ki definition. Yahan `stream` keyword hi RPC mode set karta hai.
2. **[greet.pb.go.md](./greet.pb.go.md)** — `protoc-gen-go` ne messages ke Go structs banaye (streaming me ye file na ke barabar badalti hai).
3. **[greet_grpc.pb.go.md](./greet_grpc.pb.go.md)** — `protoc-gen-go-grpc` ne client + server streaming interfaces banaye.
4. **[server-main.go.md](./server-main.go.md)** — Server ka bootstrap (listener, gRPC runtime, registration). Streaming aur unary me bootstrap identical hota hai.
5. **[server-greet.go.md](./server-greet.go.md)** — `GreetManyTimes` streaming RPC ka handler — `stream.Send()` loop.
6. **[client-main.go.md](./client-main.go.md)** — Client jo `GreetManyTimes` call karta hai aur `stream.Recv()` loop chalata hai.
7. **[go.mod.md](./go.mod.md)** — Module name aur dependencies.
8. **[Makefile.md](./Makefile.md)** — `make generate`, `make build` etc. ke targets.

## Mental model — ek baar dekh lo

```
                                 greet.proto
                              (rpc GreetManyTimes
                               returns (stream GreetResponse))
                                      |
                                      |  protoc + 2 plugins
                                      v
                  +-------------------+-------------------+
                  |                                       |
            greet.pb.go                          greet_grpc.pb.go
        (messages: structs)         (streaming client + server interfaces)
                  |                                       |
                  +-------------------+-------------------+
                                      |
                                      |  imported as `pb`
                                      v
              +-----------------------+-----------------------+
              |                                               |
       server/main.go                                  client/main.go
       server/greet.go                                 (GreetManyTimes call
       (stream.Send loop)                               + stream.Recv loop)
              |                                               |
              +----------------- HTTP/2 ---------------------+
              (1 request from client → many responses from server,
               ek hi HTTP/2 stream pe multiple DATA frames + trailers)
```

Ek line summary: **proto file me `stream` keyword likho → `make generate` chalao → server me `Send()` loop wala handler likho → client se `Recv()` loop me responses padho**.

## Unary vs Server Streaming — quick comparison

| Aspect | Unary (purana) | Server Streaming (yeh project) |
|---|---|---|
| Proto RPC line | `rpc Greet (Req) returns (Res);` | `rpc GreetManyTimes (Req) returns (stream Res);` |
| Server handler | `(ctx, *Req) (*Res, error)` | `(*Req, ServerStreamingServer[Res]) error` |
| Server "respond" | `return &Res{...}, nil` | `stream.Send(&Res{...})` × N times |
| Client call | `res, err := client.Greet(...)` | `stream, err := client.GreetManyTimes(...)` |
| Client "receive" | Direct `*Res` | `for { stream.Recv() }` until `io.EOF` |
| Wire | 1 req + 1 resp | 1 req + N resp + trailers |
