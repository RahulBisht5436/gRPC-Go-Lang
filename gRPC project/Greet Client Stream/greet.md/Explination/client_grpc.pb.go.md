# `proto/client_grpc.pb.go` — Generated gRPC client + server interfaces (Client Streaming)

> ⚠️ Ye bhi **auto-generated** file hai. `protoc-gen-go-grpc` plugin banata hai. Manual edit mat karo.

## Iss file ka kaam

`client.pb.go` me **messages** (data) thay. Iss file me **service** (behavior) hai. Yaha 4 main cheezein generate hoti hain:

1. `UserServiceClient` — interface jo client side use karta hai.
2. `userServiceClient` (lowercase) — uss interface ka implementation.
3. `UserServiceServer` — interface jo server side implement karta hai.
4. `UnimplementedUserServiceServer` — default implementation jo har RPC ke liye "Unimplemented" return karta hai.

Plus ek **registration helper** (`RegisterUserServiceServer`) aur ek **service descriptor** (`UserService_ServiceDesc`).

> **Iss project ka mode**: Client-side streaming. `SendUser` RPC me **multiple requests** upstream jaate hain, **ek single response** downstream aata hai. Iska impact iss file pe **bahut** hai — signatures pichli (server-streaming) wali file se mirror-image dikhte hain. Aage compare bhi karenge.

---

## Dono interfaces saath dekho

```30:32:proto/client_grpc.pb.go
type UserServiceClient interface {
	SendUser(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[UsersRequest, UserResponse], error)
}
```

```58:61:proto/client_grpc.pb.go
type UserServiceServer interface {
	SendUser(grpc.ClientStreamingServer[UsersRequest, UserResponse]) error
	mustEmbedUnimplementedUserServiceServer()
}
```

Notice: dono me `SendUser` method hai **but signatures pichli files se bilkul alag pattern** ke hain — kyunki client streaming hai:

| | Client side | Server side |
|---|---|---|
| Method | `SendUser(ctx, opts...)` | `SendUser(stream)` |
| Request parameter | **NAHI HAI** (Send karte ho stream pe) | **NAHI HAI** (Recv karte ho stream pe) |
| Return | `(grpc.ClientStreamingClient[Req, Res], error)` | `error` |
| Stream object | Returned (caller `Send()`/`CloseAndRecv()` istemaal kare) | Parameter ke roop me (handler `Recv()`/`SendAndClose()` istemaal kare) |
| `ctx` | Explicit pehla parameter | `ctx` ab `stream.Context()` se milta hai |

#### Key insight: `in *UsersRequest` **kahin nahi hai** signature me

Unary me `in` parameter tha. Server streaming me bhi `in` parameter tha (single request). **Client streaming me request as parameter chala gaya** — kyunki client multiple requests bhejega, ek-ek `stream.Send(...)` karke. Yahi sabse bada conceptual shift hai is mode me.

Server side bhi same — koi `in *UsersRequest` parameter nahi. Sab kuch `stream.Recv()` se aayega ek-ek karke.

#### Generic types — `ClientStreamingClient[Req, Res]` me **dono types**

Notice generic me 2 type parameters hain:

```go
grpc.ClientStreamingClient[UsersRequest, UserResponse]
//                          ^^^^^^^^^^^^   ^^^^^^^^^^^^
//                          request type   response type
```

Pichli (server streaming) wali me sirf 1 type tha (`grpc.ServerStreamingClient[GreetResponse]`) — kyunki server-streaming me client sirf receive karta hai, request ab parameter me chala gaya tha. Yahaan dono types chahiye kyunki ye stream **dono direction me kaam karta hai** — client `Send(req)` aur baad me `CloseAndRecv() (res, error)` dono karega.

---

## Method ka full path constant

```23:25:proto/client_grpc.pb.go
const (
	UserService_SendUser_FullMethodName = "/clientStream.UserService/SendUser"
)
```

Format: `/[package].[Service]/[Method]`. Ye proto file ke `package clientStream;` aur `service UserService { rpc SendUser ... }` se banta hai. Wire pe HTTP/2 `:path` header me ye string travel karti hai.

---

## Client side — kaise kaam karta hai?

### `UserServiceClient` interface — public face

```30:32:proto/client_grpc.pb.go
type UserServiceClient interface {
	SendUser(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[UsersRequest, UserResponse], error)
}
```

Tumhare client code me `client` variable ka type yahi interface hai:

```go
client := pb.NewUserServiceClient(connc)
// client ka type: pb.UserServiceClient
```

### `userServiceClient` struct — actual implementation

```34:36:proto/client_grpc.pb.go
type userServiceClient struct {
	cc grpc.ClientConnInterface
}
```

Ek single field — `cc` jo wahi connection hai jo tumne `grpc.NewClient(addr, ...)` se banaya tha.

### `NewUserServiceClient` — constructor

```38:40:proto/client_grpc.pb.go
func NewUserServiceClient(cc grpc.ClientConnInterface) UserServiceClient {
	return &userServiceClient{cc}
}
```

Bas connection ko struct me daal ke wapas dediya. **Bahut chhota function** — yahi reason hai ki tumhe magic feel hota hai.

### `SendUser` method ki actual body — sabse interesting cheez

```42:50:proto/client_grpc.pb.go
func (c *userServiceClient) SendUser(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[UsersRequest, UserResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &UserService_ServiceDesc.Streams[0], UserService_SendUser_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[UsersRequest, UserResponse]{ClientStream: stream}
	return x, nil
}
```

Step-by-step (vs server streaming version):

1. `c.cc.NewStream(ctx, ...)` — gRPC ka streaming-specific method. Ye ek `ClientStream` object banata hai jo HTTP/2 stream represent karta hai.
2. `x := &grpc.GenericClientStream[UsersRequest, UserResponse]{ClientStream: stream}` — generic wrapper jo type-safety deta hai.
3. **Return `x` — yahi pe!** Notice — **koi `SendMsg(in)` call nahi hai** is function me. Server streaming version me `SendMsg(in)` + `CloseSend()` is constructor ke andar tha (kyunki ek hi request thi to abhi bhej do). Client streaming me requests **baad me, ek-ek karke** stream pe send hone hain — to ye function bas stream khol ke deta hai, message bhejne ka kaam caller pe chhod deta hai.

#### Server streaming vs Client streaming — constructor flow ka diff

| Step | Server streaming (`GreetManyTimes` constructor) | Client streaming (`SendUser` constructor) |
|---|---|---|
| 1 | `NewStream(...)` | `NewStream(...)` |
| 2 | `SendMsg(in)` — request abhi bhejo | (skipped — caller bhejega) |
| 3 | `CloseSend()` — bata diya done | (skipped — caller bataega) |
| 4 | Return stream | Return stream |

Yahi reason hai ki server streaming me caller ko `req` constructor me dena pada, aur client streaming me nahi.

#### Methods kaha se aate hain stream object pe?

`grpc.GenericClientStream[UsersRequest, UserResponse]` me already ye methods defined hain gRPC library me:

- `Send(*UsersRequest) error` — ek request bhejna (multiple times call kar sakte ho).
- `CloseAndRecv() (*UserResponse, error)` — "main aur kuch nahi bhejunga, ab response do" signal + response read.
- `Context() context.Context` — ctx access.

Tumhare `client/main.go` me jab `stream.Send(...)` aur `stream.CloseAndRecv()` likhte ho, vahi methods run hote hain.

### Backward-compat alias

```52:53:proto/client_grpc.pb.go
// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type UserService_SendUserClient = grpc.ClientStreamingClient[UsersRequest, UserResponse]
```

Pehle (Go generics aane se pehle) gRPC har RPC ke liye ek custom interface generate karta tha jaise `UserService_SendUserClient` with `Send()` / `CloseAndRecv()` methods. Ab generics aa gayi (`grpc.ClientStreamingClient[Req, Res]`), but old code break na kare iss liye alias rakha — dono naam same type hain.

---

## Server side — kaise kaam karta hai?

### `UserServiceServer` — interface tumhe satisfy karna hai

```58:61:proto/client_grpc.pb.go
type UserServiceServer interface {
	SendUser(grpc.ClientStreamingServer[UsersRequest, UserResponse]) error
	mustEmbedUnimplementedUserServiceServer()
}
```

Tumhare `server/main.go` me `server` struct iss interface ko satisfy karta hai:

- `SendUser` method tumne `server/main.go` me hi likha (same file me — separate file me nahi).
- `mustEmbed...` `UnimplementedUserServiceServer` ko embed karne se mila.

### `UnimplementedUserServiceServer` — default fallback

```68:74:proto/client_grpc.pb.go
type UnimplementedUserServiceServer struct{}

func (UnimplementedUserServiceServer) SendUser(grpc.ClientStreamingServer[UsersRequest, UserResponse]) error {
	return status.Error(codes.Unimplemented, "method SendUser not implemented")
}
func (UnimplementedUserServiceServer) mustEmbedUnimplementedUserServiceServer() {}
func (UnimplementedUserServiceServer) testEmbeddedByValue()                     {}
```

Yeh ek empty struct hai jo har RPC ke liye **default Unimplemented response** deta hai. Important properties:

- Empty struct (`struct{}`) — zero memory cost.
- Methods **value receiver** pe hain (`func (UnimplementedUserServiceServer)`, not `*UnimplementedUserServiceServer`). Ye intentional hai — comments me likha hai "embed by value, not pointer".

#### Magic — overriding kaise hota hai?

Jab tum `server` struct me embed karte ho aur khud `SendUser` likhte ho, Go **method resolution** rule kehta hai:

> "Direct method on `server` wins over method from embedded type."

Yaani tumhara `func (s *server) SendUser(...)` hi run hoga, embedded wala default ignore ho jaayega. Iss tarah forward compatibility milti hai — agar proto me naya RPC `GetUser` add ho jaaye:

- Tumne abhi tak nahi implement kiya → embedded wala `Unimplemented` chalega → client ko `codes.Unimplemented` milega
- Tumhare baaki RPCs as-is chalu rahenge

**Yahi pattern gRPC ki sabse smart designs me se ek hai.**

### `RegisterUserServiceServer` — wiring helper

```83:92:proto/client_grpc.pb.go
func RegisterUserServiceServer(s grpc.ServiceRegistrar, srv UserServiceServer) {
	// If the following call panics, it indicates UnimplementedUserServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&UserService_ServiceDesc, srv)
}
```

Tumne `main.go` me ye line likhi thi:

```go
pb.RegisterUserServiceServer(s, &server{})
```

Function ka kaam:

1. **Safety check**: agar `Unimplemented...` pointer se embed kiya hai (galat tarika), to panic ho jaaye startup pe — runtime crash se behtar.
2. **Registration**: `s.RegisterService(&UserService_ServiceDesc, srv)` — gRPC runtime ko `UserService_ServiceDesc` (jo neeche define hai) deta hai.

### `UserService_ServiceDesc` — service ka "metadata"

```104:116:proto/client_grpc.pb.go
var UserService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "clientStream.UserService",
	HandlerType: (*UserServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SendUser",
			Handler:       _UserService_SendUser_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "client.proto",
}
```

Ye ek **dispatch table** hai — gRPC runtime isi ko padh ke decide karta hai ki kaunsa method kis handler pe jaaye:

| Field | Matlab |
|---|---|
| `ServiceName` | "clientStream.UserService" — jab client `/clientStream.UserService/SendUser` call kare to is naam ko match karega |
| `HandlerType` | Type assertion ke liye — `(*UserServiceServer)(nil)` |
| `Methods` | Array of **unary** RPCs (yahan **khali** hai — koi unary nahi) |
| `Streams` | Array of **streaming** RPCs (yahan `SendUser` hai with `ClientStreams: true`) |
| `Metadata` | Source proto file ka naam (debugging me kaam aata hai) |

#### Server streaming vs Client streaming descriptor — clear difference

Pichla (server streaming) version me ye dikhta tha:

```go
Streams: []grpc.StreamDesc{
    {
        StreamName:    "GreetManyTimes",
        Handler:       _GreetService_GreetManyTimes_Handler,
        ServerStreams: true,    // <-- ye flag
    },
},
```

Ab (client streaming) version me:

```go
Streams: []grpc.StreamDesc{
    {
        StreamName:    "SendUser",
        Handler:       _UserService_SendUser_Handler,
        ClientStreams: true,    // <-- ye flag (different)
    },
},
```

`ClientStreams: true` flag gRPC runtime ko batata hai "ye method client-side streaming hai — handler bridge ko stream object pass karna **without** pre-decoded request". Agar bidirectional hota to dono `ClientStreams: true` aur `ServerStreams: true` hote.

### `_UserService_SendUser_Handler` — bridge function

```94:96:proto/client_grpc.pb.go
func _UserService_SendUser_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(UserServiceServer).SendUser(&grpc.GenericServerStream[UsersRequest, UserResponse]{ServerStream: stream})
}
```

Yeh **bridge** hai gRPC runtime aur tumhare handler ke beech. **Server streaming bridge se aur unary bridge se signature aur logic dono alag hain**. Notice:

- Koi `m := new(UsersRequest)` aur `stream.RecvMsg(m)` nahi hai (server streaming me tha — pehle se ek request decode ho jaata tha).
- Bas raw `grpc.ServerStream` ko typed wrapper me daal ke handler ko pakda diya.

Logic:

1. `&grpc.GenericServerStream[UsersRequest, UserResponse]{ServerStream: stream}` — raw stream ko typed wrapper me badla. Ab handler is pe `Recv()` (typed) aur `SendAndClose()` (typed) call kar sakta hai.
2. `srv.(UserServiceServer).SendUser(typedStream)` — type assertion ke baad tumhara handler invoke. **Yahi moment hai jab tumhara `SendUser` handler chalu hota hai.**
3. Tumhara handler chalega, `for { stream.Recv() }` me jitne baar messages aaye padhega, end pe `stream.SendAndClose(&pb.UserResponse{...})` se single response bhejega, fir return karega.

### Backward-compat alias (server side)

```98:99:proto/client_grpc.pb.go
// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type UserService_SendUserServer = grpc.ClientStreamingServer[UsersRequest, UserResponse]
```

Same reason as client side — purana code break na ho. Agar tum chaaho:

```go
func (s *server) SendUser(stream pb.UserService_SendUserServer) error {
    // bilkul same — sirf alias use kiya
}
```

Dono valid hain. Naye Go code me generic `grpc.ClientStreamingServer[Req, Res]` zyada idiomatic hai.

---

## Client + Server flow (full picture, client streaming version)

```
CLIENT SIDE                              |   SERVER SIDE
                                         |
client.SendUser(ctx)                     |
  |                                      |
  v                                      |
userServiceClient.SendUser               |
  - cc.NewStream(...)                    |
  - return ClientStreamingClient[Req,Res]|
  |                                      |
  v ===== HTTP/2 wire (open stream) ===> |
                                         |   gRPC server receives stream
                                         |   - parses path: /clientStream.UserService/SendUser
                                         |   - looks up UserService_ServiceDesc.Streams[0]
                                         |   - finds _UserService_SendUser_Handler
                                         |   - dispatches:
                                         |       srv.SendUser(typedStream)
                                         |   - YOUR HANDLER RUNS
                                         |       for {
for _, name := range names {             |          req, err := stream.Recv()
  stream.Send(&UsersRequest{name})       |          if err == io.EOF { break }   <-- triggered by CloseAndRecv() below
}                                        |          names = append(names, req.GetName())
  - SendMsg(req) × N --> DATA frames --> |       }
                                         |       result := "hello, " + strings.Join(names, ", ")
res, _ := stream.CloseAndRecv()          |       return stream.SendAndClose(&UserResponse{Result: result})
  - CloseSend() --> END_STREAM upstream  |                                           |
  - waits for response                   |                                           |
  <===== response DATA frame ============|<───────── DATA frame (UserResponse) ──────┘
  <===== HTTP/2 trailers ================|  // handler returned, stream closed
                                         |
res = &UserResponse{Result: "hello,..."} |
fmt.Println(res.GetResult())             |
```

**Yahan key difference vs server streaming**: client `Send` loop chalata hai (server me Recv loop), aur sirf `CloseAndRecv` se ek aakhri response leta hai. Wire pe ek HTTP/2 stream bana, N DATA frames upstream + 1 DATA frame downstream + trailers se band.

---

## Tum sirf 2 cheez likhte ho

| Side | Code |
|---|---|
| Client | `stream, _ := client.SendUser(ctx)` + `for { stream.Send(...) }` + `res, _ := stream.CloseAndRecv()` |
| Server | `func (s *server) SendUser(stream) error { for { req, err := stream.Recv(); if err == io.EOF { ... return stream.SendAndClose(...) } } }` |

Baaki **saara plumbing** is generated file ne handle kar diya — HTTP/2 stream management, framing, flow-control, encoding/decoding, trailer handling.

---

## TL;DR table

| Generated Symbol | Tum kab use karte ho |
|---|---|
| `UserServiceClient` | Client variable ka type |
| `NewUserServiceClient(conn)` | Client side me, raw conn ko stub me convert karne ke liye |
| `UserServiceServer` | Server struct ko ye satisfy karna hai (signature reference) |
| `UnimplementedUserServiceServer` | Server struct me embed karna mandatory |
| `RegisterUserServiceServer(s, &server{})` | `main.go` me wiring step |
| `grpc.ClientStreamingClient[UsersRequest, UserResponse]` | Client side return type — `Send()` aur `CloseAndRecv()` istemaal karne ke liye |
| `grpc.ClientStreamingServer[UsersRequest, UserResponse]` | Server side parameter — `Recv()` aur `SendAndClose()` istemaal karne ke liye |
| `UserService_SendUserClient` | Backward-compat alias for client stream type |
| `UserService_SendUserServer` | Backward-compat alias for server stream type |
| `UserService_ServiceDesc` | Tum directly use nahi karte — registration ke andar |
| `_UserService_SendUser_Handler` | Tum directly use nahi karte — internal bridge |
| `UserService_SendUser_FullMethodName` | Internal — full RPC path |
