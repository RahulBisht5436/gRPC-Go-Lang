# `proto/bideirectionalStreams_grpc.pb.go` — Generated gRPC interfaces (Bidirectional Streaming)

> ⚠️ Ye bhi **auto-generated** file hai. `protoc-gen-go-grpc` plugin banata hai. Manual edit mat karo.

## Iss file ka kaam

`bideirectionalStreams.pb.go` me **messages** (data) thay. Iss file me **service** (behavior) hai. Yaha 4 main cheezein generate hoti hain:

1. `GreetServiceClient` — interface jo client side use karta hai.
2. `greetServiceClient` (lowercase) — uss interface ka implementation.
3. `GreetServiceServer` — interface jo server side implement karta hai.
4. `UnimplementedGreetServiceServer` — default implementation jo har RPC ke liye "Unimplemented" return karta hai.

Plus ek **registration helper** (`RegisterGreetServiceServer`) aur ek **service descriptor** (`GreetService_ServiceDesc`).

> **Iss project ka mode**: Bidirectional streaming. `GreetEveryone` RPC me **dono directions me multiple messages** flow karte hain. Iska impact iss file pe sabse interesting hai — signatures dono client streaming aur server streaming ka mix hain, aur descriptor me **dono flags** set hote hain.

---

## Dono interfaces saath dekho

```28:30:proto/bideirectionalStreams_grpc.pb.go
type GreetServiceClient interface {
	GreetEveryone(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[GreetRequest, GreetResponse], error)
}
```

```56:59:proto/bideirectionalStreams_grpc.pb.go
type GreetServiceServer interface {
	GreetEveryone(grpc.BidiStreamingServer[GreetRequest, GreetResponse]) error
	mustEmbedUnimplementedGreetServiceServer()
}
```

Notice: dono me `GreetEveryone` method hai **but signatures bilkul minimal hain**:

| | Client side | Server side |
|---|---|---|
| Method | `GreetEveryone(ctx, opts...)` | `GreetEveryone(stream)` |
| Request parameter | **NAHI HAI** (Send karte ho stream pe) | **NAHI HAI** (Recv karte ho stream pe) |
| Return | `(grpc.BidiStreamingClient[Req, Res], error)` | `error` |
| Stream object | Returned (caller dono `Send()` aur `Recv()` istemaal kare) | Parameter ke roop me (handler dono `Recv()` aur `Send()` istemaal kare) |
| `ctx` | Explicit pehla parameter | `ctx` ab `stream.Context()` se milta hai |

#### Key insight: BiDirectional sabse "symmetric" hai

Yaha **dono interfaces ek doosre ka mirror image** hain — neither has request as parameter. Sab kuch stream pe hota hai. Aur stream object pe dono `Send` aur `Recv` methods available hain.

#### Generic type — `BidiStreamingClient[Req, Res]`

```go
grpc.BidiStreamingClient[GreetRequest, GreetResponse]
//                       ^^^^^^^^^^^^   ^^^^^^^^^^^^^
//                       request type   response type
```

Client streaming wala (`ClientStreamingClient[Req, Res]`) aur bidirectional wala (`BidiStreamingClient[Req, Res]`) — **dono ke types signature same dikhte hain** (dono 2-type-parameter), but underlying methods alag hain:

| | `ClientStreamingClient[Req, Res]` | `BidiStreamingClient[Req, Res]` |
|---|---|---|
| `Send(*Req)` | Yes | Yes |
| `Recv() (*Res, error)` | **No** (use `CloseAndRecv`) | **Yes** (loop me call kar sakte ho) |
| `CloseSend()` | Yes (rarely used directly) | Yes (must call after Send done) |
| `CloseAndRecv() (*Res, error)` | Yes (single final response) | **No** (multiple responses aate hain) |

Yaani bidirectional me `CloseAndRecv` nahi hota (kyunki responses single nahi hain), `Recv` loop me call hota hai jaise server streaming me.

---

## Method ka full path constant

```21:23:proto/bideirectionalStreams_grpc.pb.go
const (
	GreetService_GreetEveryone_FullMethodName = "/bidirectional.GreetService/GreetEveryone"
)
```

Format: `/[package].[Service]/[Method]`. Ye proto file ke `package bidirectional;` aur `service GreetService { rpc GreetEveryone ... }` se banta hai. Wire pe HTTP/2 `:path` header me ye string travel karti hai.

---

## Client side — kaise kaam karta hai?

### `GreetServiceClient` interface — public face

```28:30:proto/bideirectionalStreams_grpc.pb.go
type GreetServiceClient interface {
	GreetEveryone(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[GreetRequest, GreetResponse], error)
}
```

Tumhare client code me `client` variable ka type yahi interface hai:

```go
client := pb.NewGreetServiceClient(conn)
// client ka type: pb.GreetServiceClient
```

### `greetServiceClient` struct — actual implementation

```32:34:proto/bideirectionalStreams_grpc.pb.go
type greetServiceClient struct {
	cc grpc.ClientConnInterface
}
```

Ek single field — `cc` jo wahi connection hai jo tumne `grpc.NewClient(addr, ...)` se banaya tha.

### `NewGreetServiceClient` — constructor

```36:38:proto/bideirectionalStreams_grpc.pb.go
func NewGreetServiceClient(cc grpc.ClientConnInterface) GreetServiceClient {
	return &greetServiceClient{cc}
}
```

Bas connection ko struct me daal ke wapas dediya.

### `GreetEveryone` method ki actual body — sabse minimal of all 4 modes!

```40:48:proto/bideirectionalStreams_grpc.pb.go
func (c *greetServiceClient) GreetEveryone(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[GreetRequest, GreetResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &GreetService_ServiceDesc.Streams[0], GreetService_GreetEveryone_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[GreetRequest, GreetResponse]{ClientStream: stream}
	return x, nil
}
```

Step-by-step:

1. `c.cc.NewStream(ctx, ...)` — gRPC ka streaming-specific method.
2. `x := &grpc.GenericClientStream[GreetRequest, GreetResponse]{ClientStream: stream}` — generic wrapper jo type-safety deta hai.
3. **Return `x`**. Bas! Koi `SendMsg`, koi `CloseSend`, kuch nahi.

#### Compare to all 4 modes me constructor body

| Mode | Constructor body |
|---|---|
| Unary (`Invoke`) | `c.cc.Invoke(ctx, path, in, out)` + return error |
| Server streaming | `NewStream(...)` + `SendMsg(in)` + `CloseSend()` + return stream |
| Client streaming | `NewStream(...)` + return stream (caller Send + CloseAndRecv) |
| **Bidirectional** | **`NewStream(...)` + return stream (caller Send loop + Recv loop, both manual)** |

Bidirectional sabse **chhota** constructor hai — kuch hi nahi karta, sirf stream khol ke deta. Sab kuch caller pe chhod deta — kyunki caller decide karega kab Send karna, kab Recv karna, kab CloseSend karna.

#### Stream object pe dono methods available

`grpc.GenericClientStream[GreetRequest, GreetResponse]` me ye sab methods defined hain gRPC library me:

- `Send(*GreetRequest) error` — request bhejna (multiple times).
- `Recv() (*GreetResponse, error)` — response padhna (multiple times, ya EOF tak).
- `CloseSend() error` — "ab aur kuch nahi bhejunga" signal.
- `Context() context.Context` — ctx access.

Tumhare `client/main.go` me dono `stream.Send(...)` aur `stream.Recv()` istemaal hote hain — yahi.

### Backward-compat alias

```50:51:proto/bideirectionalStreams_grpc.pb.go
// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GreetService_GreetEveryoneClient = grpc.BidiStreamingClient[GreetRequest, GreetResponse]
```

Pehle (Go generics aane se pehle) gRPC har RPC ke liye ek custom interface generate karta tha. Ab generics aa gayi, but old code break na kare iss liye alias rakha.

---

## Server side — kaise kaam karta hai?

### `GreetServiceServer` — interface tumhe satisfy karna hai

```56:59:proto/bideirectionalStreams_grpc.pb.go
type GreetServiceServer interface {
	GreetEveryone(grpc.BidiStreamingServer[GreetRequest, GreetResponse]) error
	mustEmbedUnimplementedGreetServiceServer()
}
```

Tumhare `server/main.go` me `server` struct iss interface ko satisfy karta hai:

- `GreetEveryone` method tumne `server/main.go` me hi likha.
- `mustEmbed...` `UnimplementedGreetServiceServer` ko embed karne se mila.

### `UnimplementedGreetServiceServer` — default fallback

```66:72:proto/bideirectionalStreams_grpc.pb.go
type UnimplementedGreetServiceServer struct{}

func (UnimplementedGreetServiceServer) GreetEveryone(grpc.BidiStreamingServer[GreetRequest, GreetResponse]) error {
	return status.Error(codes.Unimplemented, "method GreetEveryone not implemented")
}
func (UnimplementedGreetServiceServer) mustEmbedUnimplementedGreetServiceServer() {}
func (UnimplementedGreetServiceServer) testEmbeddedByValue()                      {}
```

Yeh ek empty struct hai jo har RPC ke liye **default Unimplemented response** deta hai. Forward compatibility ke liye essential.

#### Magic — overriding kaise hota hai?

Jab tum `server` struct me embed karte ho aur khud `GreetEveryone` likhte ho, Go **method resolution** rule kehta hai: "Direct method on `server` wins over method from embedded type." Yaani tumhara `func (s *server) GreetEveryone(...)` hi run hoga.

### `RegisterGreetServiceServer` — wiring helper

```81:90:proto/bideirectionalStreams_grpc.pb.go
func RegisterGreetServiceServer(s grpc.ServiceRegistrar, srv GreetServiceServer) {
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&GreetService_ServiceDesc, srv)
}
```

Tumne `main.go` me ye line likhi thi:

```go
pb.RegisterGreetServiceServer(s, &server{})
```

Function ka kaam: (1) safety check — `Unimplemented...` pointer se embed kiya to panic. (2) Registration via `s.RegisterService(...)`.

### `GreetService_ServiceDesc` — service ka "metadata"

```102:115:proto/bideirectionalStreams_grpc.pb.go
var GreetService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "bidirectional.GreetService",
	HandlerType: (*GreetServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GreetEveryone",
			Handler:       _GreetService_GreetEveryone_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "bideirectionalStreams.proto",
}
```

#### **Yahaan dono flags set hain — `ServerStreams: true` aur `ClientStreams: true`**

Yahi bidirectional ka encoding hai service descriptor me. Compare karo:

| Mode | `Methods` me | `Streams` me | Flags |
|---|---|---|---|
| Unary | Method entry | empty | (neither) |
| Server streaming | empty | Stream entry | `ServerStreams: true` |
| Client streaming | empty | Stream entry | `ClientStreams: true` |
| **Bidirectional** | empty | Stream entry | **both: `ServerStreams: true`, `ClientStreams: true`** |

Yahi se gRPC runtime ko pata chalta hai dispatch kaise karna hai — bidirectional ke liye **na kuch pre-decode hota** request side, **na auto-close** hota response side. Pura control handler ke paas.

### `_GreetService_GreetEveryone_Handler` — bridge function (sabse choti!)

```92:94:proto/bideirectionalStreams_grpc.pb.go
func _GreetService_GreetEveryone_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GreetServiceServer).GreetEveryone(&grpc.GenericServerStream[GreetRequest, GreetResponse]{ServerStream: stream})
}
```

**Sirf 1 line.** Compare to other modes:

| Mode | Bridge complexity |
|---|---|
| Unary | `dec` callback + interceptor + UnaryServerInfo (~10 lines) |
| Server streaming | `RecvMsg(m)` (pre-decode 1 req) + dispatch (~5 lines) |
| Client streaming | Just wrap and dispatch (~2 lines) |
| **Bidirectional** | **Just wrap and dispatch (~1 line)** |

Reason: bidirectional me kuch pre-process karne ko hai hi nahi. Na request decode karna (kyunki multiple aayenge, handler khud Recv kar lega), na response auto-send karna (kyunki multiple bhejni hain, handler khud Send karega). Bas raw stream typed wrapper me daal ke handler ko dediya.

### Backward-compat alias (server side)

```96:97:proto/bideirectionalStreams_grpc.pb.go
type GreetService_GreetEveryoneServer = grpc.BidiStreamingServer[GreetRequest, GreetResponse]
```

Same reason as client side — purana code break na ho.

---

## Client + Server flow (bidirectional)

```
CLIENT SIDE                              |   SERVER SIDE
                                         |
client.GreetEveryone(ctx)                |
  |                                      |
  v                                      |
greetServiceClient.GreetEveryone         |
  - cc.NewStream(...)                    |
  - return BidiStreamingClient[Req,Res]  |
  |                                      |
  v ===== HTTP/2 wire (open stream) ===> |
                                         |   gRPC server receives stream
                                         |   - parses path: /bidirectional.GreetService/GreetEveryone
                                         |   - looks up GreetService_ServiceDesc.Streams[0]
                                         |   - finds _GreetService_GreetEveryone_Handler
                                         |   - dispatches:
                                         |       srv.GreetEveryone(typedStream)
                                         |   - YOUR HANDLER RUNS
                                         |       for {
                                         |          req, err := stream.Recv()
                                         |          if err == io.EOF { return nil }
                                         |          stream.Send(&Res{...})
                                         |       }
                                         |
go func() {                              |
   for {                                 |
     res, err := stream.Recv()           |
     ...                                 |
   }                                     |
}()                                      |
                                         |
for _, name := range names {             |
   stream.Send(&Req{...}) ---DATA--->    |  req <-- stream.Recv() (in loop)
                                         |  stream.Send(&Res{...}) ---DATA--->
                                                                                <-- received in goroutine
}                                        |
stream.CloseSend() ---END_STREAM--->     |  stream.Recv() returns io.EOF
                                         |  handler exits loop, returns nil
                                         |  ---trailers--->
                                                                                <-- io.EOF received, close(waitc)
<-waitc  (blocks until goroutine exit)   |
log.Println("Done")                      |
```

**Yahaan key insight**: client side me **2 goroutines** kaam kar rahi hain parallel. Send loop main goroutine me, Recv loop background goroutine me. Reason: agar dono ek hi goroutine me hote, to `Send` block hota Recv ka wait karte, ya vice versa — **deadlock**! Detail [conversation/goroutines-and-waitc-explained.md](../conversation/goroutines-and-waitc-explained.md) me.

Server side me usually **single goroutine** kaafi hota hai jab pattern echo-style ho (har Recv ke baad Send). Lekin agar dono asynchronous ho (e.g., chat broadcast jahaan har user ke messages saare users ko bhejne hain), to server me bhi 2 goroutines chahiye hote hain.

---

## Tum sirf 2 cheez likhte ho

| Side | Code |
|---|---|
| Client | `stream, _ := client.GreetEveryone(ctx)`; background goroutine: `for { Recv() }`; main: `for { Send(...) }` + `CloseSend()` + `<-waitc` |
| Server | `func (s *server) GreetEveryone(stream) error { for { Recv(); Send(); } }` |

Baaki **saara plumbing** is generated file ne handle kar diya.

---

## Char modes ka bridge function compare (final picture)

| Mode | Bridge function (essence) |
|---|---|
| Unary | Decode request → call handler → encode response |
| Server streaming | Decode request (1) → call handler with typed stream |
| Client streaming | Just wrap stream → call handler (no pre-decode) |
| **Bidirectional** | **Just wrap stream → call handler (no pre-decode)** |

Last 2 modes ka bridge function identical hai — sirf type alias different. Difference handler ke andar hota hai (kya methods stream pe call kar sakte ho).

---

## TL;DR table

| Generated Symbol | Tum kab use karte ho |
|---|---|
| `GreetServiceClient` | Client variable ka type |
| `NewGreetServiceClient(conn)` | Client side me, raw conn ko stub me convert karne ke liye |
| `GreetServiceServer` | Server struct ko ye satisfy karna hai (signature reference) |
| `UnimplementedGreetServiceServer` | Server struct me embed karna mandatory |
| `RegisterGreetServiceServer(s, &server{})` | `main.go` me wiring step |
| `grpc.BidiStreamingClient[GreetRequest, GreetResponse]` | Client side return type — dono `Send` aur `Recv` istemaal karne ke liye |
| `grpc.BidiStreamingServer[GreetRequest, GreetResponse]` | Server side parameter — dono `Recv` aur `Send` istemaal karne ke liye |
| `GreetService_GreetEveryoneClient` / `Server` | Backward-compat aliases |
| `GreetService_ServiceDesc` | Tum directly use nahi karte — registration ke andar |
| `_GreetService_GreetEveryone_Handler` | Tum directly use nahi karte — internal bridge |
| Descriptor flags: `ServerStreams: true` **AND** `ClientStreams: true` | Bidirectional mode marker |
