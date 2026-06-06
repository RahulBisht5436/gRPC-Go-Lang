# `proto/greet_grpc.pb.go` — Generated gRPC client + server interfaces (Server Streaming)

> ⚠️ Ye bhi **auto-generated** file hai. `protoc-gen-go-grpc` plugin banata hai. Manual edit mat karo.

## Iss file ka kaam

`greet.pb.go` me **messages** (data) thay. Iss file me **service** (behavior) hai. Yaha 4 main cheezein generate hoti hain:

1. `GreetServiceClient` — interface jo client side use karta hai.
2. `greetServiceClient` (lowercase) — uss interface ka implementation.
3. `GreetServiceServer` — interface jo server side implement karta hai.
4. `UnimplementedGreetServiceServer` — default implementation jo har RPC ke liye "Unimplemented" return karta hai.

Plus ek **registration helper** (`RegisterGreetServiceServer`) aur ek **service descriptor** (`GreetService_ServiceDesc`).

> **Iss project ka mode**: Server-side streaming. `GreetManyTimes` RPC me ek request jaata hai, multiple responses wapas aate hain. Iska impact iss file pe **bahut** hai — signatures, dispatch table, handler bridge — sab unary se alag dikhte hain. Aage compare bhi karenge.

---

## Dono interfaces saath dekho

```28:30:proto/greet_grpc.pb.go
type GreetServiceClient interface {
	GreetManyTimes(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GreetResponse], error)
}
```

```62:65:proto/greet_grpc.pb.go
type GreetServiceServer interface {
	GreetManyTimes(*GreetRequest, grpc.ServerStreamingServer[GreetResponse]) error
	mustEmbedUnimplementedGreetServiceServer()
}
```

Notice: dono me `GreetManyTimes` method hai **but signatures bilkul alag pattern** ke hain — kyunki streaming hai:

| | Client side | Server side |
|---|---|---|
| Method | `GreetManyTimes(ctx, in, opts...)` | `GreetManyTimes(in, stream)` |
| Return | `(grpc.ServerStreamingClient[GreetResponse], error)` | `error` |
| Stream object | Returned (caller `Recv()` se messages padhta) | Parameter ke roop me (handler `Send()` karta) |
| `ctx` | Explicit pehla parameter | `ctx` ab `stream.Context()` se milta hai (parameter me nahi) |

#### Key insight: `*GreetResponse` kahin nahi hai signature me

Unary me return `(*GreetResponse, error)` hota tha. Yahan dono jagah **`*GreetResponse` direct return value nahi hai**. Iski jagah ek **stream object** hai — `grpc.ServerStreamingClient[GreetResponse]` (client) ya `grpc.ServerStreamingServer[GreetResponse]` (server). Ye objects:

- `Send(*GreetResponse)` (server side)
- `Recv() (*GreetResponse, error)` (client side)

methods provide karte hain. **Yahi server streaming ka core abstraction hai.**

#### Differences kyu?

- **`opts ...grpc.CallOption`** (client side) — variadic options jaise per-call timeout, retries, custom headers. Streaming me ye specially useful hote hain (e.g., `grpc.MaxCallRecvMsgSize` for large streams).
- **`mustEmbed...`** (server side) — tumne pehle bhi padha, **forced embedding** mechanism. Unexported method hai isliye sirf `UnimplementedGreetServiceServer` ko embed karke hi mil sakta hai.

---

## Method ka full path constant

```21:23:proto/greet_grpc.pb.go
const (
	GreetService_GreetManyTimes_FullMethodName = "/greet.GreetService/GreetManyTimes"
)
```

Format: `/[package].[Service]/[Method]`. Ye proto file ke `package greet;` aur `service GreetService { rpc GreetManyTimes ... }` se banta hai. Wire pe HTTP/2 `:path` header me ye string travel karti hai.

---

## Client side — kaise kaam karta hai?

### `GreetServiceClient` interface — public face

```28:30:proto/greet_grpc.pb.go
type GreetServiceClient interface {
	GreetManyTimes(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GreetResponse], error)
}
```

Tumhare client code me `client` variable ka type yahi interface hai:

```go
client := pb.NewGreetServiceClient(conn)
// client ka type: pb.GreetServiceClient
```

### `greetServiceClient` struct — actual implementation

```32:34:proto/greet_grpc.pb.go
type greetServiceClient struct {
	cc grpc.ClientConnInterface
}
```

Ek single field — `cc` jo wahi connection hai jo tumne `grpc.NewClient(addr, ...)` se banaya tha.

### `NewGreetServiceClient` — constructor

```36:38:proto/greet_grpc.pb.go
func NewGreetServiceClient(cc grpc.ClientConnInterface) GreetServiceClient {
	return &greetServiceClient{cc}
}
```

Bas connection ko struct me daal ke wapas dediya. **Bahut chhota function** — yahi reason hai ki tumhe magic feel hota hai.

### `GreetManyTimes` method ki actual body

```40:54:proto/greet_grpc.pb.go
func (c *greetServiceClient) GreetManyTimes(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[GreetResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &GreetService_ServiceDesc.Streams[0], GreetService_GreetManyTimes_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GreetRequest, GreetResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
```

Step-by-step (vs unary version, jo simple `Invoke()` call thi):

1. `c.cc.NewStream(ctx, &GreetService_ServiceDesc.Streams[0], path, cOpts...)` — gRPC ka **streaming-specific** method (unary me `c.cc.Invoke(...)` use hota tha). Ye ek `ClientStream` object banata hai jo HTTP/2 stream represent karta hai.
2. `x := &grpc.GenericClientStream[GreetRequest, GreetResponse]{ClientStream: stream}` — generic wrapper jo type-safety deta hai (input `GreetRequest`, output `GreetResponse`).
3. `x.ClientStream.SendMsg(in)` — server streaming me **sirf ek hi message bhejna hota hai** (request). Vo abhi hi bhej diya.
4. `x.ClientStream.CloseSend()` — bata diya server ko "main aur kuch nahi bhejunga". Ye **important** hai — server tab tak wait kar sakta hai jab tak client `CloseSend()` na call kare.
5. Return `x` — ye `grpc.ServerStreamingClient[GreetResponse]` interface ko satisfy karta hai. Caller (tumhara `client/main.go`) is pe `Recv()` loop chala sakta hai.

#### `Recv()` aata kahan se hai?

`grpc.GenericClientStream[GreetRequest, GreetResponse]` me already `Recv()` method defined hai gRPC library me — jo internally `RecvMsg(out)` call karta hai aur typed `*GreetResponse` deta hai. Tumhare client me jab `stream.Recv()` likhte ho, vahi method run hota hai.

### Backward-compat alias

```56:57:proto/greet_grpc.pb.go
// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GreetService_GreetManyTimesClient = grpc.ServerStreamingClient[GreetResponse]
```

Pehle (Go generics aane se pehle) gRPC har RPC ke liye ek custom interface generate karta tha jaise `GreetService_GreetManyTimesClient` with `Recv()` method. Ab generics aa gayi (`grpc.ServerStreamingClient[T]`), but old code ko break na kare iss liye alias rakha — dono naam same type hain.

---

## Server side — kaise kaam karta hai?

### `GreetServiceServer` — interface tumhe satisfy karna hai

```62:65:proto/greet_grpc.pb.go
type GreetServiceServer interface {
	GreetManyTimes(*GreetRequest, grpc.ServerStreamingServer[GreetResponse]) error
	mustEmbedUnimplementedGreetServiceServer()
}
```

Tumhare `server/main.go` me `Server` struct iss interface ko satisfy karta hai:

- `GreetManyTimes` method tumne `server/greet.go` me likha (with the loop calling `stream.Send(...)` 10 times).
- `mustEmbed...` `UnimplementedGreetServiceServer` ko embed karne se mila.

### `UnimplementedGreetServiceServer` — default fallback

```72:78:proto/greet_grpc.pb.go
type UnimplementedGreetServiceServer struct{}

func (UnimplementedGreetServiceServer) GreetManyTimes(*GreetRequest, grpc.ServerStreamingServer[GreetResponse]) error {
	return status.Error(codes.Unimplemented, "method GreetManyTimes not implemented")
}
func (UnimplementedGreetServiceServer) mustEmbedUnimplementedGreetServiceServer() {}
func (UnimplementedGreetServiceServer) testEmbeddedByValue()                      {}
```

Yeh ek empty struct hai jo har RPC ke liye **default Unimplemented response** deta hai. Important properties:

- Empty struct (`struct{}`) — zero memory cost.
- Methods **value receiver** pe hain (`func (UnimplementedGreetServiceServer)`, not `*UnimplementedGreetServiceServer`). Ye intentional hai — comments me likha hai "embed by value, not pointer".

#### Magic — overriding kaise hota hai?

Jab tum `Server` struct me embed karte ho aur khud `GreetManyTimes` likhte ho, Go **method resolution** rule kehta hai:

> "Direct method on `Server` wins over method from embedded type."

Yaani tumhara `func (s *Server) GreetManyTimes(...)` hi run hoga, embedded wala default ignore ho jaayega. Iss tarah forward compatibility milti hai — agar proto me naya RPC `GreetEveryone` add ho jaaye:

- Tumne abhi tak nahi implement kiya → embedded wala `Unimplemented` chalega → client ko `codes.Unimplemented` milega
- Tumhare baaki RPCs as-is chalu rahenge

**Yahi pattern gRPC ki sabse smart designs me se ek hai.**

### `RegisterGreetServiceServer` — wiring helper

```87:96:proto/greet_grpc.pb.go
func RegisterGreetServiceServer(s grpc.ServiceRegistrar, srv GreetServiceServer) {
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&GreetService_ServiceDesc, srv)
}
```

Tumne `main.go` me ye line likhi thi:

```go
pb.RegisterGreetServiceServer(s, &Server{})
```

Function ka kaam:

1. **Safety check**: agar `Unimplemented...` pointer se embed kiya hai (galat tarika), to panic ho jaaye startup pe — runtime crash se behtar.
2. **Registration**: `s.RegisterService(&GreetService_ServiceDesc, srv)` — gRPC runtime ko `GreetService_ServiceDesc` (jo neeche define hai) deta hai.

### `GreetService_ServiceDesc` — service ka "metadata"

```112:124:proto/greet_grpc.pb.go
var GreetService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "greet.GreetService",
	HandlerType: (*GreetServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GreetManyTimes",
			Handler:       _GreetService_GreetManyTimes_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "greet.proto",
}
```

Ye ek **dispatch table** hai — gRPC runtime isi ko padh ke decide karta hai ki kaunsa method kis handler pe jaaye:

| Field | Matlab |
|---|---|
| `ServiceName` | "greet.GreetService" — jab client `/greet.GreetService/GreetManyTimes` call kare to is naam ko match karega |
| `HandlerType` | Type assertion ke liye — `(*GreetServiceServer)(nil)` |
| `Methods` | Array of **unary** RPCs (yahan **khali** hai, kyunki koi unary nahi) |
| `Streams` | Array of **streaming** RPCs (yahan `GreetManyTimes` hai) |
| `Metadata` | Source proto file ka naam (debugging me kaam aata hai) |

#### Unary vs streaming descriptor — clear difference

Pichle (unary) version me ye dikhta tha:

```go
Methods: []grpc.MethodDesc{
    { MethodName: "Greet", Handler: _GreetService_Greet_Handler },
},
Streams: []grpc.StreamDesc{},  // empty
```

Ab (streaming) version me **ulta**:

```go
Methods: []grpc.MethodDesc{},   // empty
Streams: []grpc.StreamDesc{
    {
        StreamName:    "GreetManyTimes",
        Handler:       _GreetService_GreetManyTimes_Handler,
        ServerStreams: true,    // <-- naye flag
    },
},
```

`ServerStreams: true` flag gRPC runtime ko batata hai "ye method server-side streaming hai — handler bridge ko stream object pass karna, normal `Invoke` flow nahi". Agar bidirectional hota to `ClientStreams: true` bhi hota.

### `_GreetService_GreetManyTimes_Handler` — bridge function

```98:104:proto/greet_grpc.pb.go
func _GreetService_GreetManyTimes_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GreetRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(GreetServiceServer).GreetManyTimes(m, &grpc.GenericServerStream[GreetRequest, GreetResponse]{ServerStream: stream})
}
```

Yeh **bridge** hai gRPC runtime aur tumhare handler ke beech. **Unary version se signature aur logic dono alag hain** — yahan koi `dec`, `interceptor`, `UnaryServerInfo` nahi (vo cheezein streaming bridge me kaam nahi karti same way me). Logic:

1. `m := new(GreetRequest)` — empty request struct.
2. `stream.RecvMsg(m)` — wire se ek (aur sirf ek) request message decode kiya. Server streaming me **client se sirf ek message expected hai**.
3. `&grpc.GenericServerStream[GreetRequest, GreetResponse]{ServerStream: stream}` — raw `grpc.ServerStream` ko typed wrapper me badla. Iska `Send(*GreetResponse)` method hai — yahi tumhare handler ke `stream.Send(...)` me kaam aata hai.
4. `srv.(GreetServiceServer).GreetManyTimes(m, typedStream)` — type assertion ke baad tumhara handler invoke. **Yahi 4. step is the moment your handler runs.**
5. Tumhara handler chalega, jitne baar `stream.Send(...)` karega utne `*GreetResponse` wire pe jaayenge. Handler return karega `nil` (ya error) — phir gRPC runtime stream ko gracefully close kar dega (`HTTP/2 trailers` send karke).

> **Interceptor flow**: Streaming RPCs me middleware bhi hote hain (`grpc.StreamServerInterceptor`), but vo registration time pe `s.RegisterService(...)` ke andar wrap hote hain — is bridge function me directly nahi dikhte. Ye unary se thoda alag flow hai.

### Backward-compat alias (server side)

```106:107:proto/greet_grpc.pb.go
// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GreetService_GreetManyTimesServer = grpc.ServerStreamingServer[GreetResponse]
```

Same reason as client side — purana code break na ho. Agar tum chaaho to apne handler signature me ye alias use kar sakte ho:

```go
func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream pb.GreetService_GreetManyTimesServer) error {
    // same as: stream grpc.ServerStreamingServer[pb.GreetResponse]
}
```

Dono valid hain.

---

## Client + Server flow (full picture, streaming version)

```
CLIENT SIDE                              |   SERVER SIDE
                                         |
client.GreetManyTimes(ctx, &Req{...})    |
  |                                      |
  v                                      |
greetServiceClient.GreetManyTimes        |
  - cc.NewStream(...)                    |
  - SendMsg(req)                         |
  - CloseSend()                          |
  - return ServerStreamingClient[Res]    |
  |                                      |
  v ===== HTTP/2 wire (open stream) ===> |
                                         |   gRPC server receives stream
                                         |   - parses path: /greet.GreetService/GreetManyTimes
                                         |   - looks up GreetService_ServiceDesc.Streams[0]
                                         |   - finds _GreetService_GreetManyTimes_Handler
                                         |   - dispatches:
                                         |       m = decoded GreetRequest (1 msg)
                                         |       srv.GreetManyTimes(m, typedStream)
                                         |   - YOUR HANDLER RUNS
                                         |       loop:
                                         |         stream.Send(&Res{...})  ──┐
for {                                    |       return nil                  │
   res, err := stream.Recv()             |                                   │
   if err == io.EOF { break }            |                                   │
   ...                                   |                                   │
} <===== HTTP/2 DATA frames (many) ======|<──────────────────────────────────┘
  <===== HTTP/2 trailers (END_STREAM) ===|  // handler returned, stream closed
                                         |
stream.Recv() returns io.EOF             |
client loop exits                        |
```

**Yahan key difference vs unary**: ek `Invoke()` call → ek response, ki jagah ek `NewStream()` call → multiple `Recv()` ki loop. Wire pe ek HTTP/2 stream bana, multiple DATA frames chale, fir trailers se band hua.

---

## Tum sirf 2 cheez likhte ho

| Side | Code |
|---|---|
| Client | `stream, _ := client.GreetManyTimes(ctx, req)` + `for { res, err := stream.Recv(); ... }` |
| Server | `func (s *Server) GreetManyTimes(in, stream) error { for { stream.Send(...) } }` |

Baaki **saara plumbing** is generated file ne handle kar diya — HTTP/2 stream management, framing, flow-control, encoding/decoding, trailer handling.

---

## TL;DR table

| Generated Symbol | Tum kab use karte ho |
|---|---|
| `GreetServiceClient` | Client variable ka type |
| `NewGreetServiceClient(conn)` | Client side me, raw conn ko stub me convert karne ke liye |
| `GreetServiceServer` | Server struct ko ye satisfy karna hai (signature reference) |
| `UnimplementedGreetServiceServer` | Server struct me embed karna mandatory |
| `RegisterGreetServiceServer(s, &Server{})` | `main.go` me wiring step |
| `grpc.ServerStreamingClient[GreetResponse]` | Client side return type — `Recv()` istemaal karne ke liye |
| `grpc.ServerStreamingServer[GreetResponse]` | Server side parameter — `Send(...)` istemaal karne ke liye |
| `GreetService_GreetManyTimesClient` | Backward-compat alias for client stream type |
| `GreetService_GreetManyTimesServer` | Backward-compat alias for server stream type |
| `GreetService_ServiceDesc` | Tum directly use nahi karte — registration ke andar |
| `_GreetService_GreetManyTimes_Handler` | Tum directly use nahi karte — internal bridge |
| `GreetService_GreetManyTimes_FullMethodName` | Internal — full RPC path |
