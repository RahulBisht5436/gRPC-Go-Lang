# `greet/` project — explanation index

Iss folder me har source file ka detailed Hinglish explanation hai. Sequence me padhne ke liye:

1. **[greet.proto.md](./greet.proto.md)** — Sab kuch yahin se start hota hai. Service aur messages ki definition.
2. **[greet.pb.go.md](./greet.pb.go.md)** — `protoc-gen-go` ne messages ke Go structs banaye.
3. **[greet_grpc.pb.go.md](./greet_grpc.pb.go.md)** — `protoc-gen-go-grpc` ne client + server interfaces banaye.
4. **[server-main.go.md](./server-main.go.md)** — Server ka bootstrap (listener, gRPC runtime, registration).
5. **[server-greet.go.md](./server-greet.go.md)** — Actual `Greet` RPC ka handler logic.
6. **[client-main.go.md](./client-main.go.md)** — Client jo server ko call karta hai.
7. **[go.mod.md](./go.mod.md)** — Module name aur dependencies.
8. **[Makefile.md](./Makefile.md)** — `make generate`, `make build` etc. ke targets.

## Mental model — ek baar dekh lo

```
                                 greet.proto
                                      |
                                      |  protoc + 2 plugins
                                      v
                  +-------------------+-------------------+
                  |                                       |
            greet.pb.go                          greet_grpc.pb.go
        (messages: structs)                (client + server interfaces)
                  |                                       |
                  +-------------------+-------------------+
                                      |
                                      |  imported as `pb`
                                      v
              +-----------------------+-----------------------+
              |                                               |
       server/main.go                                  client/main.go
       server/greet.go                                 (Greet ko call)
       (Greet ko implement)                                   |
              |                                               |
              +----------------- HTTP/2 ---------------------+
                            (network pe baatcheet)
```

Ek line summary: **proto file likho → `make generate` chalao → server me handler likho → client se call karo**.
