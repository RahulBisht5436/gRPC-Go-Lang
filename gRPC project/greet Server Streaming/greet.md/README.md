# `greet/` project — explanation index (Server Streaming version)

Iss folder me har source file ka detailed Hinglish explanation hai. Ye project **Server-Side Streaming gRPC** dikhata hai — ek request bhejo, server multiple responses (stream me) wapas bheje. Sequence me padhne ke liye:

1. **[greet.proto.md](./Explination/greet.proto.md)** — Sab kuch yahin se start hota hai. Service aur messages ki definition. Yahan `stream` keyword hai jo unary se streaming me badal deta hai.
2. **[greet.pb.go.md](./Explination/greet.pb.go.md)** — `protoc-gen-go` ne messages ke Go structs banaye (streaming me ye file na ke barabar badalti hai).
3. **[greet_grpc.pb.go.md](./Explination/greet_grpc.pb.go.md)** — `protoc-gen-go-grpc` ne client + server streaming interfaces banaye (`grpc.ServerStreamingClient` aur `grpc.ServerStreamingServer`).
4. **[server-main.go.md](./Explination/server-main.go.md)** — Server ka bootstrap (listener, gRPC runtime, registration). Streaming me bhi ye code identical hota hai.
5. **[server-greet.go.md](./Explination/server-greet.go.md)** — `GreetManyTimes` streaming RPC ka handler logic — `stream.Send()` loop.
6. **[client-main.go.md](./Explination/client-main.go.md)** — Client jo `GreetManyTimes` call karta hai aur `stream.Recv()` loop chala ke saari responses padhta hai.
7. **[go.mod.md](./Explination/go.mod.md)** — Module name aur dependencies.
8. **[Makefile.md](./Explination/Makefile.md)** — `make generate`, `make build` etc. ke targets.

## Deep-dive notes (`conversation/` folder)

Conversation me jo extra topics discuss hue, unka detailed study material:

- **[conversation/context-explained.md](./conversation/context-explained.md)** — `context.Context` ka pura A-to-Z: kyu zaruri, `Background()` vs `TODO()`, 4 problems jo solve karta, saare constructors, gRPC me wire pe travel kaise, common patterns aur galtiyaan. Streaming me `stream.Context()` se ctx milta hai — yahi knowledge wahan bhi apply hoti hai.

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
               wire pe ek hi stream pe multiple DATA frames)
```

Ek line summary: **proto file me `stream` keyword likho → `make generate` chalao → server me `Send()` loop wala handler likho → client se `Recv()` loop me responses padho**.

## Unary vs Server Streaming — quick comparison

| Aspect | Unary (purana) | Server Streaming (yeh project) |
|---|---|---|
| Proto | `rpc Greet (Req) returns (Res);` | `rpc GreetManyTimes (Req) returns (stream Res);` |
| Server handler signature | `(ctx, *Req) (*Res, error)` | `(*Req, ServerStreamingServer[Res]) error` |
| Server "response" mechanism | `return &Res{...}, nil` | `stream.Send(&Res{...})` (multiple times) |
| Client call | `res, err := client.Greet(ctx, req)` | `stream, err := client.GreetManyTimes(ctx, req)` |
| Client "response" mechanism | Ek `*Res` direct mil gaya | `for { stream.Recv() }` jab tak `io.EOF` |
| Wire | 1 request frame, 1 response frame | 1 request frame, **N response frames + trailers** |
| Use cases | CRUD, RPC, simple queries | Live feeds, paginated results, log tailing, notifications |
