# `server/greet.go` — Greet RPC ka actual handler

`server/main.go` server ko **start** karta hai. Lekin actual logic — yaani jab koi `Greet` call kare to kya karna hai — vo is file me hai. Ye **separation of concerns** ka idiom hai: bootstrap aur handlers ko alag rakho.

## Pura file

```go
package main

import (
    "context"
    "log"

    pb "example.com/greet/proto"
)

func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
    log.Printf("Greet invoked with: %s", in.GetFirstName())
    return &pb.GreetResponse{
        Result: "Hello " + in.GetFirstName(),
    }, nil
}
```

> `package main` likha hai isliye yaha — kyunki ye file `main.go` ke saath same folder me hai, dono ek hi binary banate hain. Ek folder = ek package, Go ka rule.

---

## Sabse pehla sawaal — ye **method** kaha se decide hua?

`protoc-gen-go-grpc` ne tumhare proto ki `service GreetService` ko padh ke ye Go interface generate kiya:

```53:56:gRPC project/greet/proto/greet_grpc.pb.go
type GreetServiceServer interface {
	Greet(context.Context, *GreetRequest) (*GreetResponse, error)
	mustEmbedUnimplementedGreetServiceServer()
}
```

Tumhara `func (s *Server) Greet(...)` exactly is interface ke `Greet` method ki shape match karta hai. Yeh shape match hi vo cheez hai jo tumhare struct ko `GreetServiceServer` "ban" deti hai.

Yaani: **tum khud ne ye signature nahi socha — vo proto se aaya hai.**

---

## Line-by-line breakdown

### `func (s *Server) Greet(...)` — receiver

```go
func (s *Server) Greet(...)
//   ^^^^^^^^^^
//   "method receiver" — ye Greet `*Server` type pe attach hai
```

Iska matlab:

- Ye **method** hai, normal function nahi.
- Sirf `*Server` ke instance pe call ho sakta hai.
- Yaad karo `main.go` me ye line:

  ```go
  pb.RegisterGreetServiceServer(s, &Server{})
  ```

  Yahan `&Server{}` banaya — ab is instance pe `Greet` method available hai.

`s` parameter ka istemaal hum is function me **nahi** kar rahe. Future me agar tumhe DB connection inject karna ho:

```go
type Server struct {
    pb.UnimplementedGreetServiceServer
    db *sql.DB   // nayi field
}

func (s *Server) Greet(...) (*pb.GreetResponse, error) {
    user, _ := s.db.Query(...)   // ab `s.db` use karoge
    ...
}
```

To `s` ka kaam **state inject karna** hota hai handlers me.

### `ctx context.Context`

`context` Go ka standard package hai. gRPC ke har handler me **pehla parameter** `context.Context` hota hai. Ye 3 cheezein carry karta hai:

#### 1. Cancellation / Deadlines

Client ne agar 5-second timeout set kiya tha:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
res, err := client.Greet(ctx, ...)
```

Vo timeout `ctx` ke saath server tak pohochta hai. Tumhare handler me check kar sakte ho:

```go
select {
case <-ctx.Done():
    return nil, ctx.Err()  // client ne cancel kar diya
default:
    // continue
}
```

Iss simple example me hum check nahi kar rahe (kyunki kaam itna fast hai), but bade handlers me ye zaruri hota hai.

#### 2. Metadata

Client headers (jaise auth tokens) `ctx` ke through bhejte hain. Server me:

```go
md, _ := metadata.FromIncomingContext(ctx)
token := md.Get("authorization")
```

#### 3. Tracing / Request ID

Distributed tracing tools (jaeger, opentelemetry) `ctx` me trace IDs propagate karte hain.

> **TL;DR**: `ctx` ko **dabba** samjho jo client se server tak deadline + metadata leke aata hai. Iss simple handler me use nahi ho raha, bas signature ka part hai.

### `in *pb.GreetRequest`

Ye actual request message hai jo client ne bheji.

- `*pb.GreetRequest` — pointer to `GreetRequest` struct.
- `GreetRequest` struct kaha define hua? `greet.pb.go` me:

```24:29:gRPC project/greet/proto/greet.pb.go
type GreetRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FirstName     string                 `protobuf:"bytes,1,opt,name=firstName,proto3" json:"firstName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}
```

Yahan se tumne ye field generate hote dekhi: `FirstName string`. Ye **actually wahi hai** jo proto me likha tha:

```7:9:gRPC project/greet/proto/greet.proto
message GreetRequest{
    string firstName =1;
}
```

`firstName` (camelCase) → `FirstName` (PascalCase) ban gaya. Pointer kyu? Kyunki proto messages bade ho sakte hain — pointer pass karna sasta hai memory aur copy ke hisab se.

#### `in.GetFirstName()` vs `in.FirstName`

Tumne dono variations dekhi hongi. Difference:

```go
in.FirstName        // direct access — agar `in` nil hua to crash
in.GetFirstName()   // safe getter — `in` nil hua to "" return karega
```

`greet.pb.go` me getter dekho:

```61:66:gRPC project/greet/proto/greet.pb.go
func (x *GreetRequest) GetFirstName() string {
	if x != nil {
		return x.FirstName
	}
	return ""
}
```

**Production code me hamesha `GetXxx()` use karo** — nil-safe hai.

### `(*pb.GreetResponse, error)` — return type

Do values return karte ho:

1. `*pb.GreetResponse` — actual response. `nil` bhi return kar sakte ho agar error hai.
2. `error` — Go ka standard error mechanism. gRPC isse automatically **status code** me convert karta hai.

#### Error return karne ka idiomatic tarika

Plain `errors.New(...)` mat use karo gRPC me. Specific status code dena chahiye:

```go
import "google.golang.org/grpc/status"
import "google.golang.org/grpc/codes"

return nil, status.Error(codes.InvalidArgument, "first name cannot be empty")
```

Common codes:
- `codes.OK` — success (default jab `nil` return kar do)
- `codes.InvalidArgument` — galat input
- `codes.NotFound` — resource nahi mila
- `codes.Internal` — server bug
- `codes.DeadlineExceeded` — timeout

### `log.Printf("Greet invoked with: %s", in.GetFirstName())`

Simple debug logging. Server ke terminal me dikhega:

```
Greet invoked with: Rahul
```

Production me structured logging (jaise `zap`, `zerolog`) use hoti hai, but seekhne ke liye `log` perfect hai.

### Return statement

```go
return &pb.GreetResponse{
    Result: "Hello " + in.GetFirstName(),
}, nil
```

- `&pb.GreetResponse{...}` — naya response struct, **pointer** liya (kyunki return type pointer hai).
- `Result` field me string assemble karke daal di. Agar `in.GetFirstName() == "Rahul"` to `Result == "Hello Rahul"`.
- `, nil` — error nahi, sab theek hai.

---

## Ek baar pura request/response cycle dekho

```
1. Client:
     client.Greet(ctx, &GreetRequest{FirstName: "Rahul"})
                                         |
                                         |  protobuf bytes me serialize
                                         v
2. Network (HTTP/2):
     POST /greet.GreetService/Greet
     [protobuf-encoded GreetRequest body]
                                         |
                                         v
3. Server gRPC runtime:
     - bytes ko decode kar ke *GreetRequest banaya
     - method dispatch kiya: "Greet ke handler ko call karo"
                                         |
                                         v
4. Tumhara handler chala:
     func (s *Server) Greet(ctx, in) (*GreetResponse, error)
        in.FirstName == "Rahul"
        return &GreetResponse{Result: "Hello Rahul"}, nil
                                         |
                                         v
5. Server gRPC runtime:
     - response struct ko bytes me serialize kiya
     - HTTP/2 response me bheja
                                         |
                                         v
6. Client:
     res.GetResult() == "Hello Rahul"
```

Tumne sirf **step 4** likha. Baaki sab automatic.

---

## Real handlers me kya badlta hai?

Production handler me typically aur cheezein hoti hain:

```go
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
    if in.GetFirstName() == "" {
        return nil, status.Error(codes.InvalidArgument, "first name required")
    }

    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Imagine some DB call
    // user, err := s.db.GetUser(ctx, in.GetFirstName())
    // if err != nil {
    //     return nil, status.Errorf(codes.Internal, "db error: %v", err)
    // }

    return &pb.GreetResponse{
        Result: "Hello " + in.GetFirstName(),
    }, nil
}
```

Pattern same hai — bas validation + ctx-check + actual work + proper errors.

---

## TL;DR

| Cheez | Kya hai |
|---|---|
| `func (s *Server) Greet` | Method on `*Server` — tum likhte ho |
| Method signature | Proto se generate hua interface se 1-to-1 |
| `ctx context.Context` | Cancellation/deadline/metadata carrier |
| `in *pb.GreetRequest` | Decoded request from client |
| `in.GetFirstName()` | nil-safe getter |
| `(*pb.GreetResponse, error)` | Response + status |
| `&pb.GreetResponse{...}` | Naya response banaya, pointer return |
| `nil` (error) | Sab theek, success |
