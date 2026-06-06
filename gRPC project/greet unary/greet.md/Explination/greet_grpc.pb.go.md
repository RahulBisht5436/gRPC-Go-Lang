# `proto/greet_grpc.pb.go` — Generated gRPC client + server interfaces

> ⚠️ Ye bhi **auto-generated** file hai. `protoc-gen-go-grpc` plugin banata hai. Manual edit mat karo.

## Iss file ka kaam

`greet.pb.go` me **messages** (data) thay. Iss file me **service** (behavior) hai. Yaha 4 main cheezein generate hoti hain:

1. `GreetServiceClient` — interface jo client side use karta hai.
2. `greetServiceClient` (lowercase) — uss interface ka implementation.
3. `GreetServiceServer` — interface jo server side implement karta hai.
4. `UnimplementedGreetServiceServer` — default implementation jo har RPC ke liye "Unimplemented" return karta hai.

Plus ek **registration helper** (`RegisterGreetServiceServer`) aur ek **service descriptor** (`GreetService_ServiceDesc`).

---

## Dono interfaces saath dekho

```28:30:gRPC project/greet/proto/greet_grpc.pb.go
type GreetServiceClient interface {
	Greet(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (*GreetResponse, error)
}
```

```53:56:gRPC project/greet/proto/greet_grpc.pb.go
type GreetServiceServer interface {
	Greet(context.Context, *GreetRequest) (*GreetResponse, error)
	mustEmbedUnimplementedGreetServiceServer()
}
```

Notice: dono interfaces me `Greet` method hai but **signatures thode different**:

| | Client | Server |
|---|---|---|
| Method | `Greet(ctx, in, opts...)` | `Greet(ctx, in)` |
| Extra param | `opts ...grpc.CallOption` | — |
| Extra method | — | `mustEmbedUnimplementedGreetServiceServer()` |

#### Differences kyu?

- **`opts ...grpc.CallOption`** (client side) — variadic options jaise per-call timeout, retries, custom headers. 99% time tum koi nahi pass karte.
- **`mustEmbed...`** (server side) — tumne pehle bhi padha, **forced embedding** mechanism. Unexported method hai isliye sirf `UnimplementedGreetServiceServer` ko embed karke hi mil sakta hai.

---

## Client side — kaise kaam karta hai?

### `GreetServiceClient` interface — public face

```28:30:gRPC project/greet/proto/greet_grpc.pb.go
type GreetServiceClient interface {
	Greet(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (*GreetResponse, error)
}
```

Tumhare client code me `client` variable ka type yahi interface hai:

```go
client := pb.NewGreetServiceClient(conn)
// client ka type: pb.GreetServiceClient
```

### `greetServiceClient` struct — actual implementation

```32:34:gRPC project/greet/proto/greet_grpc.pb.go
type greetServiceClient struct {
	cc grpc.ClientConnInterface
}
```

Ek single field — `cc` jo wahi connection hai jo tumne `grpc.NewClient(addr, ...)` se banaya tha.

### `NewGreetServiceClient` — constructor

```36:38:gRPC project/greet/proto/greet_grpc.pb.go
func NewGreetServiceClient(cc grpc.ClientConnInterface) GreetServiceClient {
	return &greetServiceClient{cc}
}
```

Bas connection ko struct me daal ke wapas dediya. **Bahut chhota function** — yahi reason hai ki tumhe magic feel hota hai.

### `Greet` method ki actual body

```40:48:gRPC project/greet/proto/greet_grpc.pb.go
func (c *greetServiceClient) Greet(ctx context.Context, in *GreetRequest, opts ...grpc.CallOption) (*GreetResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GreetResponse)
	err := c.cc.Invoke(ctx, GreetService_Greet_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
```

Step-by-step:

1. `out := new(GreetResponse)` — empty response struct banaya jisme server ka jawab fill hoga.
2. `c.cc.Invoke(ctx, "/greet.GreetService/Greet", in, out, ...)` — gRPC ka generic invoke method. Ye:
   - `in` ko serialize karta hai
   - HTTP/2 request bhejta hai given path pe
   - response wait karta hai
   - bytes ko `out` me deserialize kar deta hai
3. Error ho to return, warna `out` return.

`GreetService_Greet_FullMethodName` constant yahan defined hai:

```22:22:gRPC project/greet/proto/greet_grpc.pb.go
	GreetService_Greet_FullMethodName = "/greet.GreetService/Greet"
```

Format: `/[package].[Service]/[Method]`. Ye proto file ke `package greet;` aur `service GreetService { rpc Greet ... }` se banta hai.

---

## Server side — kaise kaam karta hai?

### `GreetServiceServer` — interface tumhe satisfy karna hai

```53:56:gRPC project/greet/proto/greet_grpc.pb.go
type GreetServiceServer interface {
	Greet(context.Context, *GreetRequest) (*GreetResponse, error)
	mustEmbedUnimplementedGreetServiceServer()
}
```

Tumhare `server/main.go` me `Server` struct iss interface ko satisfy karta hai:

- `Greet` method tumne `server/greet.go` me likha.
- `mustEmbed...` `UnimplementedGreetServiceServer` ko embed karne se mila.

### `UnimplementedGreetServiceServer` — default fallback

```63:69:gRPC project/greet/proto/greet_grpc.pb.go
type UnimplementedGreetServiceServer struct{}

func (UnimplementedGreetServiceServer) Greet(context.Context, *GreetRequest) (*GreetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Greet not implemented")
}
func (UnimplementedGreetServiceServer) mustEmbedUnimplementedGreetServiceServer() {}
func (UnimplementedGreetServiceServer) testEmbeddedByValue()                      {}
```

Yeh ek empty struct hai jo har RPC ke liye **default Unimplemented response** deta hai. Important properties:

- Empty struct (`struct{}`) — zero memory cost.
- Methods **value receiver** pe hain (`func (UnimplementedGreetServiceServer)`, not `*UnimplementedGreetServiceServer`). Ye intentional hai — comments me likha hai "embed by value, not pointer".

#### Magic — overriding kaise hota hai?

Jab tum `Server` struct me embed karte ho aur khud `Greet` likhte ho, Go **method resolution** rule kehta hai:

> "Direct method on `Server` wins over method from embedded type."

Yaani tumhara `func (s *Server) Greet(...)` hi run hoga, embedded wala default ignore ho jaayega. Iss tarah forward compatibility milti hai — agar proto me naya RPC `GreetManyTimes` add ho jaaye:

- Tumne abhi tak nahi implement kiya → embedded wala `Unimplemented` chalega → client ko `codes.Unimplemented` milega
- Tumhare baaki RPCs as-is chalu rahenge

**Yahi pattern gRPC ki sabse smart designs me se ek hai.**

### `RegisterGreetServiceServer` — wiring helper

```78:87:gRPC project/greet/proto/greet_grpc.pb.go
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

```110:121:gRPC project/greet/proto/greet_grpc.pb.go
var GreetService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "greet.GreetService",
	HandlerType: (*GreetServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Greet",
			Handler:    _GreetService_Greet_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "greet.proto",
}
```

Ye ek **dispatch table** hai — gRPC runtime isi ko padh ke decide karta hai ki kaunsa method kis handler pe jaaye:

| Field | Matlab |
|---|---|
| `ServiceName` | "greet.GreetService" — jab client `/greet.GreetService/Greet` call kare to is naam ko match karega |
| `HandlerType` | Type assertion ke liye — `(*GreetServiceServer)(nil)` |
| `Methods` | Array of unary RPCs (Greet yaha hai) |
| `Streams` | Array of streaming RPCs (abhi khali — kyunki sirf unary hai) |
| `Metadata` | Source proto file ka naam (debugging me kaam aata hai) |

### `_GreetService_Greet_Handler` — bridge function

```89:105:gRPC project/greet/proto/greet_grpc.pb.go
func _GreetService_Greet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GreetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GreetServiceServer).Greet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: GreetService_Greet_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GreetServiceServer).Greet(ctx, req.(*GreetRequest))
	}
	return interceptor(ctx, in, info, handler)
}
```

Yeh **bridge** hai gRPC runtime aur tumhare handler ke beech. Logic:

1. Empty `GreetRequest` banaya.
2. `dec(in)` — incoming bytes ko `in` me decode kiya.
3. Agar interceptor (middleware) nahi hai, sidha `srv.Greet(ctx, in)` call kar do (yahi tumhara handler hai!).
4. Agar interceptor hai (logging, auth, etc.), pehle usse pass karo, vo finally `srv.Greet(...)` call karega.

`srv.(GreetServiceServer).Greet(ctx, in)` — type assertion ke baad tumhara `Greet` method invoke hota hai. **Yahi 4. step is the moment your handler runs.**

---

## Client + Server flow (full picture)

```
CLIENT SIDE                             |   SERVER SIDE
                                        |
client.Greet(ctx, &GreetRequest{...})   |
  |                                     |
  v                                     |
greetServiceClient.Greet                |
  - in -> bytes (serialize)             |
  - cc.Invoke("/greet.GreetService/     |
              Greet", in, out)          |
  |                                     |
  v ===== HTTP/2 wire ===========>      |
                                        |   gRPC server receives bytes
                                        |   - parses path: /greet.GreetService/Greet
                                        |   - looks up GreetService_ServiceDesc
                                        |   - finds _GreetService_Greet_Handler
                                        |   - dispatches:
                                        |       in = decoded GreetRequest
                                        |       srv.(GreetServiceServer).Greet(ctx, in)
                                        |   - your handler runs → returns response
                                        |   - response -> bytes (serialize)
  <===== HTTP/2 wire ===========        |
  - response bytes -> *GreetResponse    |
  - return (out, nil)                   |
res.GetResult()                         |
```

Ye file middle ka pura "infrastructure" provide karti hai. Tumhe sirf 2 cheez likhni padti hain:
- Client side: `client.Greet(ctx, req)` (1 line)
- Server side: `func (s *Server) Greet(...)` (handler body)

---

## TL;DR table

| Generated Symbol | Tum kab use karte ho |
|---|---|
| `GreetServiceClient` | Client variable ka type |
| `NewGreetServiceClient(conn)` | Client side me, raw conn ko stub me convert karne ke liye |
| `GreetServiceServer` | Server struct ko ye satisfy karna hai (signature reference) |
| `UnimplementedGreetServiceServer` | Server struct me embed karna mandatory |
| `RegisterGreetServiceServer(s, &Server{})` | `main.go` me wiring step |
| `GreetService_ServiceDesc` | Tum directly use nahi karte — registration ke andar |
| `_GreetService_Greet_Handler` | Tum directly use nahi karte — internal bridge |
| `GreetService_Greet_FullMethodName` | Internal — full RPC path |
