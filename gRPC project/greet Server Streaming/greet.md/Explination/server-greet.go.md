# `server/greet.go` — GreetManyTimes RPC ka actual handler (Server Streaming)

`server/main.go` server ko **start** karta hai. Lekin actual logic — yaani jab koi `GreetManyTimes` call kare to kya karna hai — vo is file me hai. Ye **separation of concerns** ka idiom hai: bootstrap aur handlers ko alag rakho.

## Pura file

```go
package main

import (
    "context"
    "fmt"
    "log"

    pb "example.com/greet/proto"
    "google.golang.org/grpc"
)

// Greet handles the unary GreetService.Greet RPC.
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
    log.Printf("Greet invoked with: %s", in.GetFirstName())
    return &pb.GreetResponse{
        Result: "Hello " + in.GetFirstName(),
    }, nil
}

func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error {
    fmt.Println("Stream Function initiated")

    for i := 0; i < 10; i++ {
        res := fmt.Sprintf("Changes for the User %s and time %d", in.FirstName, i)
        stream.Send(&pb.GreetResponse{
            Result: res,
        })
    }

    return nil
}
```

> `package main` likha hai isliye yaha — kyunki ye file `main.go` ke saath same folder me hai, dono ek hi binary banate hain. Ek folder = ek package, Go ka rule.

> **Ek note**: Iss file me ek purana unary `Greet(ctx, in)` method bhi padha hua hai jo abhi `GreetServiceServer` interface me hai hi nahi (kyunki proto me sirf `GreetManyTimes` declare hai). Iska matlab — ye method **dead code** hai. Kahin bhi call nahi hota; sirf compile hota hai aur binary me space leta hai. **Production cleanup**: ye method delete kar dena chahiye — proto se vo ab gone hai. Jab tak rakho, ye sirf ek "history" reference hai unary version ka. Hum is page pe primarily streaming method `GreetManyTimes` ko hi explain karenge.

---

## Sabse pehla sawaal — ye **method** kaha se decide hua?

`protoc-gen-go-grpc` ne tumhare proto ki `service GreetService` ko padh ke ye Go interface generate kiya:

```62:65:proto/greet_grpc.pb.go
type GreetServiceServer interface {
	GreetManyTimes(*GreetRequest, grpc.ServerStreamingServer[GreetResponse]) error
	mustEmbedUnimplementedGreetServiceServer()
}
```

Tumhara `func (s *Server) GreetManyTimes(...)` exactly is interface ke `GreetManyTimes` method ki shape match karta hai. Yeh shape match hi vo cheez hai jo tumhare struct ko `GreetServiceServer` "ban" deti hai.

Yaani: **tum khud ne ye signature nahi socha — vo proto me `stream` keyword se aaya hai.**

---

## Unary vs streaming — handler signatures saath dekho

| | Unary version (purana) | Server streaming (current) |
|---|---|---|
| Signature | `func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error)` | `func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error` |
| `ctx` | Pehla parameter | Parameter me **nahi** — `stream.Context()` se milta hai |
| Response | Return `(*GreetResponse, error)` | `stream.Send(...)` baar-baar; sirf `error` return |
| Multiple responses? | Nahi (1 only) | **Haan — jitne baar `Send` chalao** |
| Handler kab khatam? | `return` statement pe | `return nil` (ya error) — fir gRPC stream close kar deta hai |

**Yeh mental model shift important hai**: streaming handler "response banake return" nahi karta — vo "stream object pe likhta jaata hai" jab tak satisfied na ho. Yaad rakho — return type sirf `error` hai, koi `*GreetResponse` nahi.

---

## Line-by-line breakdown

### `func (s *Server) GreetManyTimes(...)` — receiver

```go
func (s *Server) GreetManyTimes(...)
//   ^^^^^^^^^^
//   "method receiver" — ye GreetManyTimes `*Server` type pe attach hai
```

Iska matlab:

- Ye **method** hai, normal function nahi.
- Sirf `*Server` ke instance pe call ho sakta hai.
- Yaad karo `main.go` me ye line:

  ```go
  pb.RegisterGreetServiceServer(s, &Server{})
  ```

  Yahan `&Server{}` banaya — ab is instance pe `GreetManyTimes` method available hai.

`s` parameter ka istemaal hum is function me **nahi** kar rahe. Future me agar tumhe DB connection inject karna ho:

```go
type Server struct {
    pb.UnimplementedGreetServiceServer
    db *sql.DB   // nayi field
}

func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error {
    rows, _ := s.db.QueryContext(stream.Context(), ...)
    for rows.Next() {
        ...
        stream.Send(&pb.GreetResponse{ Result: ... })
    }
    return nil
}
```

To `s` ka kaam **state inject karna** hota hai handlers me — same as unary.

### `in *pb.GreetRequest` — request message

Ye actual request message hai jo client ne bheji. **Sirf ek hi message** — server streaming me client ek hi request bhejta hai (multiple responses milte hain, multiple requests nahi).

- `*pb.GreetRequest` — pointer to `GreetRequest` struct.
- `GreetRequest` struct kaha define hua? `greet.pb.go` me:

```24:29:proto/greet.pb.go
type GreetRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FirstName     string                 `protobuf:"bytes,1,opt,name=firstName,proto3" json:"firstName,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}
```

#### `in.FirstName` vs `in.GetFirstName()` — current code me kya hai?

Tumhare current handler me ye line hai:

```go
res := fmt.Sprintf("Changes for the User %s and time %d", in.FirstName, i)
//                                                       ^^^^^^^^^^^^^
//                                                       direct access
```

**Yahan `in.GetFirstName()` use karna safer hota** — nil-safe getter:

```go
in.FirstName        // direct access — agar `in` nil hua to crash
in.GetFirstName()   // safe getter — `in` nil hua to "" return karega
```

`greet.pb.go` me getter:

```61:66:proto/greet.pb.go
func (x *GreetRequest) GetFirstName() string {
	if x != nil {
		return x.FirstName
	}
	return ""
}
```

> **Rule**: Production code me hamesha `GetXxx()` use karo. gRPC framework normally `nil` request kabhi nahi pass karega (vo wire decode pe ya error de dega ya valid struct dega), but habit zaruri hai — kal nested messages ho jaaye to direct access crash karega.

### `stream grpc.ServerStreamingServer[pb.GreetResponse]` — output channel

Ye **iss handler ka core abstraction** hai — ek **typed stream object** jo tumhe responses bhejne deta hai.

- `grpc.ServerStreamingServer[pb.GreetResponse]` — Go generics use karke gRPC ne ye interface banaya.
- Iska main method: `Send(*pb.GreetResponse) error` — ek response message bhejna.
- Aur ek hidden gem: `stream.Context() context.Context` — `ctx` (jo unary me parameter hota tha) yahaan se nikalta hai.

#### Backward-compat alias

`greet_grpc.pb.go` me ek alias hai:

```106:107:proto/greet_grpc.pb.go
type GreetService_GreetManyTimesServer = grpc.ServerStreamingServer[GreetResponse]
```

To agar tum chaaho:

```go
func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream pb.GreetService_GreetManyTimesServer) error {
    // bilkul same — sirf alias use kiya
}
```

Both signatures equivalent. Naye Go code me generic `grpc.ServerStreamingServer[T]` zyada idiomatic hai.

### `fmt.Println("Stream Function initiated")` — logging

Simple debug line. Server terminal me dikhega jab koi client `GreetManyTimes` call kare:

```
Stream Function initiated
```

Production me `log.Printf` ya structured logger (`zap`, `zerolog`) use karna behtar — `fmt.Println` me timestamp aur log level nahi hota.

### Loop body — main streaming logic

```go
for i := 0; i < 10; i++ {
    res := fmt.Sprintf("Changes for the User %s and time %d", in.FirstName, i)
    stream.Send(&pb.GreetResponse{
        Result: res,
    })
}
```

Step-by-step:

1. **Loop 10 iterations** — tumne hardcode kiya 10. Real handler me ye DB rows, file lines, paginated API results, ya kuch bhi ho sakta hai.
2. **`fmt.Sprintf(...)`** — har iteration me ek different message banata hai.
3. **`stream.Send(&pb.GreetResponse{Result: res})`** — yahi **wire pe ek HTTP/2 DATA frame trigger karta hai**. Client side me ek `Recv()` call ko ye message satisfy karega.

Yaani 10 baar `Send` = 10 alag-alag DATA frames wire pe = client side me 10 baar `Recv()` returns successfully.

#### `stream.Send` ke return value ko ignore karna — chhota bug

Notice that current code does:

```go
stream.Send(&pb.GreetResponse{ Result: res })
```

Lekin `Send` ka return type hai `error`. Agar client ne stream prematurely close kar diya (ya network drop hua), `Send` non-nil error return karega. Production handler me:

```go
if err := stream.Send(&pb.GreetResponse{ Result: res }); err != nil {
    return err  // gRPC framework client ko error propagate kar dega
}
```

Ye pattern follow karna safer hai. Bina check ke loop continue karta rahega even after client gone — wasted work.

#### `time.Sleep` (artificial delay) — common pattern

Real-world streaming demo me typically log me dekhne ke liye delay add karte hain:

```go
for i := 0; i < 10; i++ {
    res := fmt.Sprintf(...)
    stream.Send(&pb.GreetResponse{Result: res})
    time.Sleep(500 * time.Millisecond)   // har 500ms me ek message
}
```

Tumne abhi delay nahi rakha — 10 messages ek dum se chale jaate hain. Demo ke liye delay add karna better.

### `return nil` — handler exit = stream close

```go
return nil
```

Jab handler `return` karta hai (with `nil` error), gRPC framework:

1. **HTTP/2 trailers** bhejta hai (`grpc-status: 0`, `grpc-message: ""`).
2. Stream `END_STREAM` flag se close ho jaata hai.
3. Client side `stream.Recv()` next call pe `io.EOF` deta hai — yahi signal hai client ke liye "ab aur kuch nahi aayega".

Agar tumne `return error` kiya hota:

```go
return status.Error(codes.Internal, "DB error")
```

Trailers me error code aur message embed ho jaate. Client side `stream.Recv()` me wahi error milta — `io.EOF` ki jagah.

---

## Ek baar pura request/response cycle dekho

```
1. Client:
     stream, _ := client.GreetManyTimes(ctx, &GreetRequest{FirstName: "Rahul Bisht"})
                                              |
                                              |  protobuf bytes me serialize (1 request)
                                              v
2. Network (HTTP/2):
     POST /greet.GreetService/GreetManyTimes
     [protobuf-encoded GreetRequest body]
     END_STREAM (client side) -- via CloseSend()
                                              |
                                              v
3. Server gRPC runtime:
     - bytes ko decode kar ke *GreetRequest banaya
     - method dispatch kiya: streaming bridge ko call kiya
                                              |
                                              v
4. Tumhara handler chala:
     func (s *Server) GreetManyTimes(in, stream) error
        in.FirstName == "Rahul Bisht"
        loop 10x:
          stream.Send(&GreetResponse{Result: "Changes for the User ... time 0"})  -- DATA frame 1
          stream.Send(&GreetResponse{Result: "Changes for the User ... time 1"})  -- DATA frame 2
          ...
          stream.Send(&GreetResponse{Result: "Changes for the User ... time 9"})  -- DATA frame 10
        return nil   --> trailers (grpc-status: 0)
                                              |
                                              v
5. Server gRPC runtime:
     - har Send ko bytes me serialize kar ke wire pe daala
     - return nil pe trailers bheje, stream close
                                              |
                                              v
6. Client:
     for {
         res, err := stream.Recv()
         if err == io.EOF { break }   <-- 11th call pe yahi hota
         log.Printf("Response: %s", res.GetResult())
     }
```

Tumne sirf **step 4** likha. Baaki sab automatic.

---

## Real handlers me kya badlta hai? (production version)

```go
func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error {
    if in.GetFirstName() == "" {
        return status.Error(codes.InvalidArgument, "first name required")
    }

    ctx := stream.Context()

    for i := 0; i < 10; i++ {
        // Client cancellation check — important in streaming!
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        res := fmt.Sprintf("Changes for the User %s and time %d", in.GetFirstName(), i)
        if err := stream.Send(&pb.GreetResponse{Result: res}); err != nil {
            return err
        }

        time.Sleep(500 * time.Millisecond)
    }

    return nil
}
```

Differences vs current:

1. **Input validation** — empty `firstName` reject.
2. **`stream.Context()`** se ctx nikala.
3. **`ctx.Done()` check** har iteration me — agar client disconnect/timeout, immediately exit.
4. **`Send` ka error check** — pipeline broken to early return.
5. **`GetFirstName()` getter** — nil-safe.
6. **Artificial delay** — demo me real-time feel.

Pattern same hai — bas validation + ctx-check + actual work + proper errors.

---

## TL;DR

| Cheez | Kya hai |
|---|---|
| `func (s *Server) GreetManyTimes` | Streaming method on `*Server` — tum likhte ho |
| Method signature | Proto `stream` keyword se generated interface se 1-to-1 |
| `in *pb.GreetRequest` | Decoded request from client (sirf 1 message) |
| `stream grpc.ServerStreamingServer[pb.GreetResponse]` | Output channel — `Send()` se messages bhejte ho |
| `stream.Context()` | Cancellation/deadline carrier — yahi ctx hai |
| `stream.Send(&pb.GreetResponse{...})` | Ek response wire pe bhejna (multiple times call kar sakte ho) |
| `return nil` | Handler ne kaam khatam kiya — gRPC stream close karta hai |
| `return error` | Error → trailers me embed → client ko `Recv()` pe error milta hai |

> **Streaming ka one-liner**: "response banake return karna" ki jagah "stream pe likhna jab tak chahe", aur fir return karke gracefully close karna. Yahi conceptual shift hai unary se.
