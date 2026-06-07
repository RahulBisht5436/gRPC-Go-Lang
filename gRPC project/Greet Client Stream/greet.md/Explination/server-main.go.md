# `server/main.go` — Server bootstrap + `SendUser` handler

Iss project me **server ka bootstrap aur handler dono ek hi file me** hain. Pichle (server-streaming) project me `main.go` (bootstrap) aur `greet.go` (handler) alag the. Yahaan combined hai — small project ke liye theek hai, but production me separate karna behtar hota (next time refactor karte time `server/handler.go` me `SendUser` shift kar dena).

## Pura file (tumhare current code ke saath)

```go
package main

import (
	"io"
	"log"
	"net"
	"strings"

	pb "example.com/clientStream/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedUserServiceServer
}

var addr string = "localhost:8080"

func (s *server) SendUser(stream grpc.ClientStreamingServer[pb.UsersRequest, pb.UserResponse]) error {
	var names []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			result := "hello, " + strings.Join(names, ", ")
			return stream.SendAndClose(&pb.UserResponse{
				Result: result,
			})
		}
		if err != nil {
			log.Printf("recv error: %v", err)
			return err
		}

		log.Printf("Received name: %s", req.GetName())
		names = append(names, req.GetName())
	}
}

func main() {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Not able to initiate the server: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterUserServiceServer(s, &server{})

	log.Printf("Server listening on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
```

---

## Iss file me 2 cheezein hain — bootstrap + handler

### Part 1 — Bootstrap (4 mandatory steps)

1. **TCP listener kholo** — port pe baith jao.
2. **`*grpc.Server` banao** — gRPC ka brain.
3. **Service register karo** — runtime ko batao "ye RPC mere paas aaye".
4. **`Serve()` call karo** — connections accept karna shuru.

### Part 2 — `SendUser` handler (client streaming)

1. **Recv loop chalao** — `stream.Recv()` baar-baar tab tak jab tak `io.EOF` na aaye (signal: client ne `CloseAndRecv` call kar diya).
2. Har request se kuch state collect karo (`names` slice).
3. **EOF aane pe `SendAndClose(&Res{...})`** — single aggregate response + close in one call.

---

## Imports breakdown

```go
import (
    "io"        // <-- naya: io.EOF detect karne ke liye
    "log"
    "net"
    "strings"   // <-- naya: strings.Join() ke liye

    pb "example.com/clientStream/proto"
    "google.golang.org/grpc"
)
```

- `io` — **server streaming ke client side me jaisa tha**, ab **client streaming ke server side me chahiye**. Direction reverse ho gaya — kyunki ab EOF "client ne send khatam kiya" wala signal hai, jo server ko milta hai. Detail [conversation/streaming-direction-explained.md](../conversation/streaming-direction-explained.md) me.
- `strings` — final aggregate banane ke liye (`strings.Join(names, ", ")`).
- `pb` — generated proto package.
- `grpc` — gRPC runtime.

> **Note**: Tumhare imports clean hain. Initially tumne `github.com/golang/protobuf/protoc-gen-go/grpc` import bhi try kiya tha (duplicate `grpc` package), jo galat tha — vo legacy/internal helper hai. Sirf `google.golang.org/grpc` chahiye.

---

## `var addr = "localhost:8080"`

Server kaha listen karega, vo address. **Yahan ek warning hai**:

> ⚠️ **Tumhare current `client/main.go` me `addr = "localhost:3000"` hai**. Mismatch ke wajah se client `connection refused` deta hai. Match karwana zaruri hai. Detail [client-main.go.md](./client-main.go.md) me.

`localhost` ka matlab — sirf same machine se accessible. Compare karo `0.0.0.0` se (jo pichle project me use hua tha — saare network interfaces se accessible). Local dev ke liye `localhost` is fine.

---

## `type server struct { pb.UnimplementedUserServiceServer }`

Ye chhoti si line gRPC ka sabse important "trick" hai. Do reasons:

### Reason 1 — Interface satisfaction

`client_grpc.pb.go` me ek interface hai:

```58:61:proto/client_grpc.pb.go
type UserServiceServer interface {
	SendUser(grpc.ClientStreamingServer[UsersRequest, UserResponse]) error
	mustEmbedUnimplementedUserServiceServer()
}
```

Notice — `mustEmbed...` method **unexported** hai. Yaani tum apne package se isse implement hi nahi kar sakte directly. Sirf `UnimplementedUserServiceServer` ko **embed** karke ye method automatic mil jaata hai.

Yeh **forced embedding pattern** kehlata hai — gRPC team ne specifically isliye banaya taaki tum bhul se `UnimplementedUserServiceServer` ko embed kiye bina server na bana lo.

### Reason 2 — Forward compatibility

Aaj proto me sirf `SendUser` RPC hai. Kal agar proto me `GetUser` ya `DeleteUser` add ho gaya, to:

- **Bina embedding** ke: tumhara server compile fail kar dega.
- **With embedding**: `UnimplementedUserServiceServer` ke andar default methods already hain jo `codes.Unimplemented` return karte hain. Tumhara server compile chalu rahega.

> Iska summary: **embedding karna pure project me sabse zaruri 1 line hai**.

---

## `SendUser` handler — line-by-line (sabse interesting part)

### Method signature

```go
func (s *server) SendUser(stream grpc.ClientStreamingServer[pb.UsersRequest, pb.UserResponse]) error
```

Compare karo pichle versions se:

| | Unary | Server streaming | Client streaming (current) |
|---|---|---|---|
| Signature | `(ctx, *Req) (*Res, error)` | `(*Req, ServerStreamingServer[Res]) error` | **`(ClientStreamingServer[Req, Res]) error`** |
| Request parameter | Pehla parameter | Pehla parameter | **NAHI HAI** — `stream.Recv()` se aata hai |
| Response mechanism | `return &Res{...}, nil` | `stream.Send(...)` × N | `return stream.SendAndClose(&Res{...})` |
| `ctx` | Pehla parameter | Parameter me nahi — `stream.Context()` | Parameter me nahi — `stream.Context()` |

**Sabse bada conceptual shift** — request as parameter chala gaya. Sab kuch stream pe aata hai.

### `var names []string` — state accumulator

```go
var names []string
```

Client streaming me handler ka job hota hai **multiple requests ko aggregate karna**. Yahaan hum sirf names ko slice me collect kar rahe hain. Real-world me ye:

- DB rows hote (`var rows []Row`),
- file chunks (`var buf bytes.Buffer`),
- counters (`var totalBytes int64`),
- ya kuch bhi.

Pattern wahi — har `Recv()` pe state update karo, EOF pe finalize karo.

### Recv loop ka anatomy

```go
for {
    req, err := stream.Recv()
    if err == io.EOF {
        result := "hello, " + strings.Join(names, ", ")
        return stream.SendAndClose(&pb.UserResponse{
            Result: result,
        })
    }
    if err != nil {
        log.Printf("recv error: %v", err)
        return err
    }

    log.Printf("Received name: %s", req.GetName())
    names = append(names, req.GetName())
}
```

#### `stream.Recv()`

Ye block karta hai jab tak:

- ek naya `*pb.UsersRequest` aaye → `req` me, `err = nil`
- client ne `CloseSend()` ya `CloseAndRecv()` call kar diya → `req = nil`, `err = io.EOF`
- error aa jaaye (network drop, context timeout) → `req = nil`, `err = some error`

**Yahan key insight**: client streaming me **server ko EOF milta hai**, server streaming me client ko milta tha. Direction reverse. Detail [conversation/streaming-direction-explained.md](../conversation/streaming-direction-explained.md) me.

#### `if err == io.EOF` — yahi response bhejne ka time hai

Client streaming me **EOF == "ab final response do"**. Tumhare handler me yahi pattern hai:

```go
if err == io.EOF {
    result := "hello, " + strings.Join(names, ", ")
    return stream.SendAndClose(&pb.UserResponse{
        Result: result,
    })
}
```

`stream.SendAndClose(...)` 2 cheez ek saath karta hai:

1. Ek `*pb.UserResponse` wire pe bhejta hai (downstream DATA frame).
2. Stream gracefully close kar deta hai (HTTP/2 trailers `grpc-status: 0` ke saath).

**Important rule**: `SendAndClose` ko **sirf ek baar** call karo. Multiple times call karne pe error aayega. Yahi reason hai ki client streaming me sirf ek hi response milta hai.

> **Better pattern** (production): error check bhi karo —
> ```go
> if err == io.EOF {
>     result := "hello, " + strings.Join(names, ", ")
>     if err := stream.SendAndClose(&pb.UserResponse{Result: result}); err != nil {
>         return err
>     }
>     return nil
> }
> ```

#### `if err != nil` — non-EOF error

```go
if err != nil {
    log.Printf("recv error: %v", err)
    return err
}
```

Agar `err` non-nil hai aur `io.EOF` nahi hai, kuch galat hua:

- Client ne mid-stream disconnect kar diya
- Network drop ho gaya
- Context timeout ya cancel

Tumne sahi kiya — `return err` use karke gRPC framework ko error propagate karne diya. gRPC client side ko `CloseAndRecv()` me yahi error milega.

> **Pichle code me bug tha**: Tumne pehle `log.Fatalf("recv error: %v", err)` likha tha — vo `os.Exit(1)` kar deta, server crash. Ab `log.Printf` + `return err` hai — correct pattern. (Pichle conversation me tumne yahi fix kiya tha — accha kaam.)

#### Successful Recv ke baad

```go
log.Printf("Received name: %s", req.GetName())
names = append(names, req.GetName())
```

- `req.GetName()` — nil-safe getter (recommended over `req.Name`).
- `append` se slice me add. Go ka slice automatically grow karta hai.

Loop chalta rahega jab tak EOF na aaye.

---

## `func main()` — bootstrap (server streaming version se identical)

### Step 1: Listener

```go
lis, err := net.Listen("tcp", addr)
if err != nil {
    log.Fatalf("Not able to initiate the server: %v", err)
}
```

`net.Listen` ek **`net.Listener`** return karta hai. Ye object kuch nahi karta, sirf port pe baitha rehta hai aur incoming TCP connections accept karne ke liye ready hota hai. Naam `lis` rakha — convention.

Agar port already kisi aur process ne le rakha hai (`address already in use`), ya permission nahi hai, to error aayega. `log.Fatalf` print karta hai aur `os.Exit(1)` call kar deta hai.

### Step 2: gRPC runtime banao

```go
s := grpc.NewServer()
```

`grpc.NewServer()` ek **fresh gRPC server object** deta hai. Ye:

- HTTP/2 framing handle karta hai
- Protobuf encoding/decoding karta hai
- Method dispatch karta hai (jaise routing)
- Concurrency manage karta hai (har RPC apni goroutine me)

Lekin abhi tak ye **kuch nahi jaanta** ki kaunse RPCs handle karne hain. Khaali brain hai. Knowledge agle step me daalenge.

### Step 3: Service register karo

```go
pb.RegisterUserServiceServer(s, &server{})
```

Ye **wiring** step hai. Iska matlab:

> "Hey gRPC runtime `s`, agar koi `/clientStream.UserService/SendUser` ko call kare, to is `&server{}` instance ke `SendUser` method ko client-streaming-handler bridge ke through trigger karna."

#### `&server{}` vs `server{}` — pointer ya value?

Tumne `&server{}` (pointer) pass kiya — **sahi pattern**. Reasons:

1. Tumhara `SendUser` method `*server` pointer receiver pe define hai (`func (s *server) SendUser(...)`). Bina pointer pass kiye method set match nahi karega.
2. Embedded `UnimplementedUserServiceServer` ke methods value receiver pe hain, but **embed by value** ki recommendation file me likhi hai — to address-of value ek pointer banata hai jo dono method sets ko satisfy karta hai.

`server{}` (value) pass karte to compile error aata: `server does not implement UserServiceServer (SendUser method has pointer receiver)`.

> Agar tum `RegisterUserServiceServer` line bhul gaye, to server start to ho jaayega lekin har RPC pe client ko `unknown service: clientStream.UserService` error milega.

### Step 4: Serve karo

```go
log.Printf("Server listening on %s", addr)
if err := s.Serve(lis); err != nil {
    log.Fatalf("Failed to serve: %v", err)
}
```

`s.Serve(lis)` ka kaam:

1. `lis.Accept()` ka loop chalata hai — har incoming TCP connection le leta hai.
2. Connection ko HTTP/2 me upgrade karta hai.
3. RPC requests ko parse karke registered handlers ko dispatch karta hai.
4. **Block karta hai** — yaani function return nahi karta jab tak server crash na ho ya `s.Stop()` call na ho.

Isiliye `s.Serve(lis)` `main()` ki **last** line hota hai — iske baad kuch likh bhi nahi sakte.

**Smart move tumne kiya** — `log.Printf("Server listening on %s", addr)` ko `Serve` ke **pehle** rakha. Agar `Serve` ke baad rakhte to ye line kabhi nahi chalti (Serve block karta hai forever). Pehle log dikhna better UX hai.

---

## Mental model

```
   port 8080 (localhost)
       |
       v
+--------------+
|   net.Listen | <- Step 1
+--------------+
       |
       v
+--------------+
|  grpc.Server | <- Step 2 (empty brain)
+--------------+
       |
       |  Step 3: register
       v
+----------------------+
|  Service descriptor  |
|  + &server{} pointer |
+----------------------+
       |
       v
+--------------+
|   s.Serve()  | <- Step 4 (block forever)
+--------------+

When request comes:
       |
       v
+--------------------------------+
|  _UserService_SendUser_Handler |  <- generated bridge
+--------------------------------+
       |
       v
+--------------------------------+
|  YOUR SendUser handler         |
|    for { Recv() } until EOF    |
|    SendAndClose(&Res{...})     |
+--------------------------------+
```

---

## Common galtiyaan (jo tumne already karli aur fix ki!)

| Bug | Lakshan | Fix |
|---|---|---|
| Embed `pb.UsersRequest` instead of `UnimplementedUserServiceServer` | Compile error: interface satisfy nahi | Embed `pb.UnimplementedUserServiceServer` |
| Dono `github.com/golang/protobuf/protoc-gen-go/grpc` aur `google.golang.org/grpc` import karna | Compile error: `grpc redeclared` | Sirf `google.golang.org/grpc` rakho |
| `server{}` (value) pass karna `Register` me | Compile error: pointer receiver mismatch | `&server{}` use karo |
| `log.Fatalf` in Recv error handler | Server crash on first client disconnect | `log.Printf` + `return err` |
| `log.Printf` server start ke baad | `Serve` block karta hai, log kabhi nahi dikhta | Pehle log, baad me Serve |
| `SendAndClose` ko multiple baar call | Runtime error | Sirf ek baar — usually EOF case me |
| `Recv()` ka loop without EOF check | Infinite block ya panic | Always `if err == io.EOF { ... }` first |

---

## Production-version handler (better practices)

```go
func (s *server) SendUser(stream grpc.ClientStreamingServer[pb.UsersRequest, pb.UserResponse]) error {
    ctx := stream.Context()
    var names []string

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        req, err := stream.Recv()
        if err == io.EOF {
            if len(names) == 0 {
                return status.Error(codes.InvalidArgument, "at least one name required")
            }
            result := "hello, " + strings.Join(names, ", ")
            if err := stream.SendAndClose(&pb.UserResponse{Result: result}); err != nil {
                return err
            }
            return nil
        }
        if err != nil {
            return err
        }

        if req.GetName() == "" {
            return status.Error(codes.InvalidArgument, "name cannot be empty")
        }

        names = append(names, req.GetName())
    }
}
```

Differences vs current:

1. **`stream.Context()`** se ctx nikala, `ctx.Done()` check har iteration me — client disconnect pe immediately exit.
2. **Input validation** — empty names reject.
3. **At-least-one-message check** — empty stream pe error.
4. **`SendAndClose` ka error check** — pipeline broken to early return.
5. **`return nil`** clean exit.

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `package main` + `func main()` | Executable binary |
| `type server struct { pb.Unimplemented... }` | Forced embedding for forward compat |
| `var addr = "localhost:8080"` | Listen address (⚠️ client se match karna chahiye) |
| `net.Listen("tcp", addr)` | TCP listener |
| `grpc.NewServer()` | gRPC brain |
| `pb.RegisterUserServiceServer(s, &server{})` | Wiring |
| `s.Serve(lis)` | Accept loop, blocks forever |
| `func (s *server) SendUser(stream)` | Tumhara client-streaming handler |
| `for { stream.Recv() }` | Multiple requests padhna |
| `err == io.EOF` | Client ne `CloseAndRecv` call kar diya → ab response do |
| `stream.SendAndClose(&Res{...})` | Single aggregate response + close (sirf ek baar call) |

> **Client-streaming server ka one-liner**: "ek-ek request padho jab tak client done na bole (EOF), fir ek aggregate response bhejo aur band karo."
