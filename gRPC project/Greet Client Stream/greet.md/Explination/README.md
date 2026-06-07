# `Greet Client Stream/` project — explanation index (Client Streaming)

Iss folder me har source file ka detailed Hinglish explanation hai. Ye project **Client-Side Streaming gRPC** ka hai — client multiple requests stream me bhejta hai, server **ek single aggregate response** wapas deta hai. Sequence me padhne ke liye:

1. **[client.proto.md](./client.proto.md)** — Sab kuch yahin se start hota hai. `stream` keyword ab **request side** pe hai (not response side).
2. **[client.pb.go.md](./client.pb.go.md)** — `protoc-gen-go` ne `UsersRequest` aur `UserResponse` ke Go structs banaye.
3. **[client_grpc.pb.go.md](./client_grpc.pb.go.md)** — `protoc-gen-go-grpc` ne client + server **client-streaming** interfaces banaye.
4. **[server-main.go.md](./server-main.go.md)** — Server ka bootstrap + `SendUser` handler (`for { Recv() }` loop + `SendAndClose(...)`).
5. **[client-main.go.md](./client-main.go.md)** — Client jo stream open karke `Send` loop chalata hai, fir `CloseAndRecv()` se single response leta hai.
6. **[go.mod.md](./go.mod.md)** — Module name aur dependencies.

## Mental model — ek baar dekh lo

```
                                 client.proto
                              (rpc SendUser
                               (stream UsersRequest) returns (UserResponse))
                                      |
                                      |  protoc + 2 plugins
                                      v
                  +-------------------+-------------------+
                  |                                       |
            client.pb.go                         client_grpc.pb.go
        (messages: structs)         (client-streaming client + server interfaces)
                  |                                       |
                  +-------------------+-------------------+
                                      |
                                      |  imported as `pb`
                                      v
              +-----------------------+-----------------------+
              |                                               |
       server/main.go                                  client/main.go
       (Recv loop until EOF                            (Send loop
        + SendAndClose)                                 + CloseAndRecv)
              |                                               |
              +----------------- HTTP/2 ---------------------+
              (N requests upstream → 1 response downstream,
               ek hi HTTP/2 stream pe multiple DATA frames + trailers)
```

Ek line summary: **proto file me request side pe `stream` likho → server me `Recv` loop until `io.EOF` → fir `SendAndClose(&res)` → client me `Send` loop → fir `CloseAndRecv()` for the final response**.

## Unary vs Server Streaming vs Client Streaming — quick compare

| Aspect | Unary | Server Streaming | **Client Streaming (current)** |
|---|---|---|---|
| Proto RPC line | `rpc Greet (Req) returns (Res);` | `rpc GreetManyTimes (Req) returns (stream Res);` | `rpc SendUser (stream Req) returns (Res);` |
| Server handler | `(ctx, *Req) (*Res, error)` | `(*Req, ServerStreamingServer[Res]) error` | `(ClientStreamingServer[Req, Res]) error` |
| Server "respond" | `return &Res{...}, nil` | `stream.Send(&Res{...})` × N | `return stream.SendAndClose(&Res{...})` |
| Client call | `res, _ := client.Greet(...)` | `stream, _ := client.GreetManyTimes(...)` | `stream, _ := client.SendUser(ctx)` (no req!) |
| Client "transmit" | Single `*Req` in call | Single `*Req` in call | `for { stream.Send(...) }` + `CloseAndRecv()` |
| EOF appears on | n/a | Client side `Recv()` | **Server side `Recv()`** |
| Wire | 1 req + 1 resp | 1 req + N resp | **N req + 1 resp** |
