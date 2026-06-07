# `server/main.go` — Server bootstrap + `GreetEveryone` handler (Bidirectional)

Iss project me **server ka bootstrap aur handler dono ek hi file me** hain (jaise client streaming wala project tha). Production me usually `server/handler.go` me handler shift kar dete hain.

## Pura file (tumhare current code)

```go
package main

import (
	"io"
	"log"
	"net"

	pb "example.com/bidirectional/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedGreetServiceServer
}

var addr = "localhost:8080"

func (s *server) GreetEveryone(stream grpc.BidiStreamingServer[pb.GreetRequest, pb.GreetResponse]) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("recv error: %v", err)
			return err
		}

		respondMessage := "Hello, " + req.GetFirstName()
		if err := stream.Send(&pb.GreetResponse{Result: respondMessage}); err != nil {
			log.Printf("send error: %v", err)
			return err
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("unable to start listener: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGreetServiceServer(s, &server{})

	log.Printf("Server started at %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("unable to start server: %v", err)
	}
}
```

---

## Iss file me 2 cheezein hain — bootstrap + handler

### Part 1 — Bootstrap (4 mandatory steps, same as always)

1. **TCP listener kholo** — port pe baith jao.
2. **`*grpc.Server` banao** — gRPC ka brain.
3. **Service register karo** — runtime ko batao "ye RPC mere paas aaye".
4. **`Serve()` call karo** — connections accept karna shuru.

### Part 2 — `GreetEveryone` handler (bidirectional echo pattern)

1. **Infinite for loop** chalao.
2. Har iteration me **`Recv()` karo** — ek request padhne ka wait.
3. **EOF aane pe `return nil`** — client ne `CloseSend` call kar diya, handler done.
4. **Successful Recv pe immediately `Send()`** — echo back.

Yahi **Echo Pattern** sabse simple bidirectional implementation hai. Aur bhi patterns hain (next section me).

---

## Imports breakdown

```go
import (
    "io"        // <-- io.EOF detect karne ke liye (client side se EOF aata)
    "log"
    "net"
    pb "example.com/bidirectional/proto"
    "google.golang.org/grpc"
)
```

- `io` — bidirectional me **server side ko EOF milta** hai (kyunki client `CloseSend` karega first). Client streaming me bhi server side me lagta tha, ab bhi.
- `pb` — generated proto package.
- `grpc` — gRPC runtime.

Notice — `strings` import nahi hai (client streaming wale me tha — vo `Join` ke liye). Bidirectional me usually har Recv ka individual response banta hai, aggregate nahi.

---

## `var addr = "localhost:8080"`

Server kaha listen karega, vo address. **Yahan ek warning hai**:

> ⚠️ **Tumhare current `client/main.go` me `addr = "localhost:50051"` hai**. Mismatch ke wajah se client `connection refused` deta hai. Detail [client-main.go.md](./client-main.go.md) me.

`localhost` ka matlab — sirf same machine se accessible.

---

## `type server struct { pb.UnimplementedGreetServiceServer }`

Standard pattern — `pb.UnimplementedGreetServiceServer` embed karna mandatory hai forward compatibility ke liye. Detail [bideirectionalStreams_grpc.pb.go.md](./bideirectionalStreams_grpc.pb.go.md) me.

---

## `GreetEveryone` handler — line-by-line

### Method signature

```go
func (s *server) GreetEveryone(stream grpc.BidiStreamingServer[pb.GreetRequest, pb.GreetResponse]) error
```

Compare karo char modes:

| | Unary | Server stream | Client stream | **Bidirectional** |
|---|---|---|---|---|
| Signature | `(ctx, *Req) (*Res, err)` | `(*Req, SSS[Res]) err` | `(CSS[Req,Res]) err` | **`(BSS[Req,Res]) err`** |
| Request param | Pehla | Pehla | NAHI | **NAHI** |
| Response mechanism | `return &Res, nil` | `Send` × N + `return nil` | `return SendAndClose(&Res)` | **`Send` × N + `return nil`** |
| `ctx` | Pehla param | `stream.Context()` | `stream.Context()` | **`stream.Context()`** |
| Stream methods | n/a | `Send` | `Recv, SendAndClose` | **`Recv, Send`** |

**Bidirectional sabse symmetric hai** — `Recv` aur `Send` dono milte hain, request param nahi.

### Infinite for loop — bidirectional ka core

```go
for {
    req, err := stream.Recv()
    if err == io.EOF {
        return nil
    }
    if err != nil {
        log.Printf("recv error: %v", err)
        return err
    }

    respondMessage := "Hello, " + req.GetFirstName()
    if err := stream.Send(&pb.GreetResponse{Result: respondMessage}); err != nil {
        log.Printf("send error: %v", err)
        return err
    }
}
```

#### Step-by-step

1. **`stream.Recv()`** — block karta hai jab tak:
   - Naya `*pb.GreetRequest` aaye → `req` me, `err = nil`
   - Client ne `CloseSend()` call kar diya → `req = nil`, `err = io.EOF`
   - Error aa jaaye (network drop, ctx cancel) → `req = nil`, `err = some error`

2. **`if err == io.EOF { return nil }`** — clean exit. Handler return karte hi gRPC stream close kar deta hai (trailers bhejta hai). Client side `Recv()` ko `io.EOF` milta hai → uska goroutine `close(waitc)` se signal deta hai.

3. **`if err != nil`** — non-EOF error. `log.Printf` + `return err` se gRPC framework client ko error propagate karta hai (status code + message ke saath).

4. **`stream.Send(&pb.GreetResponse{...})`** — turant response bhejna. Yahi bidirectional ka magic — Recv ke baad **same handler me** Send kar sakte ho, alag goroutine ki zarurat nahi.

#### Echo pattern ka flow (har request → har response)

```
Time →
client.Send(req1) ----→
                       server.Recv() returns req1
                       respondMessage := "Hello, " + req1.GetFirstName()
                       server.Send(res1) ----→
                                              ← client receives res1
client.Send(req2) ----→
                       server.Recv() returns req2
                       server.Send(res2) ----→
                                              ← client receives res2
...
client.CloseSend() ----→
                       server.Recv() returns io.EOF
                       return nil
                       (gRPC sends trailers)
                                              ← client.Recv() returns io.EOF
```

Notice — **request aur response 1:1 mapped hain** is pattern me. Lekin bidirectional me ye **mandatory nahi**. Server N requests pe 1 response bhi bhej sakta hai (batch), ya 1 request pe N responses (subscription), ya completely asynchronous.

### `req.GetFirstName()` — nil-safe getter

```go
respondMessage := "Hello, " + req.GetFirstName()
```

Tumne `req.GetFirstName()` use kiya — **sahi pattern**. Direct `req.FirstName` access nil pointer dereference de sakta hai (theoretical case, gRPC framework normally valid struct deta hai).

### `stream.Send(...)` ka error check

Tumne `Send` ka return value check kiya — **good practice**:

```go
if err := stream.Send(&pb.GreetResponse{Result: respondMessage}); err != nil {
    log.Printf("send error: %v", err)
    return err
}
```

Agar client ne stream prematurely close kar diya, `Send` non-nil error return karega. Bina check ke loop continue karta rahega even after client gone — wasted work.

---

## Echo pattern vs other bidirectional patterns

Tumhara handler **echo pattern** use karta hai (har Recv → ek Send, sequential, same goroutine). Bidirectional me aur 3 common patterns hain:

### Pattern 1: Echo (current — sequential)

```go
for {
    req, err := stream.Recv()
    if err == io.EOF { return nil }
    if err != nil { return err }

    stream.Send(&Res{...})
}
```

**Kab use karte**: real-time transformations (translate, transcribe, encrypt-on-the-fly). Simple aur deadlock-safe.

### Pattern 2: Aggregate (batch responses)

```go
var batch []string
for {
    req, err := stream.Recv()
    if err == io.EOF {
        if len(batch) > 0 {
            stream.Send(&Res{Result: aggregate(batch)})
        }
        return nil
    }
    if err != nil { return err }

    batch = append(batch, req.GetFirstName())
    if len(batch) == 10 {
        stream.Send(&Res{Result: aggregate(batch)})
        batch = nil
    }
}
```

**Kab use karte**: bulk processing, log shipping with batch flush, IoT telemetry rollups.

### Pattern 3: Subscription (1 request, ongoing pushes)

```go
req, err := stream.Recv()
if err != nil { return err }

ch := subscribe(req.GetTopic())
for event := range ch {
    if err := stream.Send(&Res{Event: event}); err != nil {
        return err
    }
}
return nil
```

**Kab use karte**: notifications, stock ticker, chat room subscribe.

### Pattern 4: Fully async (2 goroutines in handler)

```go
errc := make(chan error, 2)

// Recv goroutine
go func() {
    for {
        req, err := stream.Recv()
        if err == io.EOF { errc <- nil; return }
        if err != nil { errc <- err; return }
        process(req)   // queue, broadcast, etc.
    }
}()

// Send goroutine
go func() {
    for event := range eventChan {
        if err := stream.Send(&Res{Event: event}); err != nil {
            errc <- err
            return
        }
    }
    errc <- nil
}()

return <-errc
```

**Kab use karte**: real-time chat, multiplayer game state sync, jahaan Recv aur Send completely independent ho.

> ⚠️ **Important rule for goroutines in server handler**: gRPC docs kehte hain "**`SendMsg` and `RecvMsg` can be called concurrently from different goroutines**, but `SendMsg` calls cannot be concurrent with each other, and `RecvMsg` calls cannot be concurrent with each other." Yaani Recv aur Send alag goroutines me chal sakte, lekin do `Send` calls parallel galat hai.

---

## `func main()` — bootstrap

### Step 1: Listener

```go
lis, err := net.Listen("tcp", addr)
if err != nil {
    log.Fatalf("unable to start listener: %v", err)
}
```

`net.Listen` ek **`net.Listener`** return karta hai. Naam `lis` rakha — convention.

### Step 2: gRPC runtime banao

```go
s := grpc.NewServer()
```

`grpc.NewServer()` ek **fresh gRPC server object** deta hai. Abhi tak ye kuch nahi jaanta. Knowledge agle step me.

### Step 3: Service register karo

```go
pb.RegisterGreetServiceServer(s, &server{})
```

Wiring. `&server{}` pointer pass kiya — sahi pattern (kyunki `SendUser` method `*server` pointer receiver pe define hai).

### Step 4: Serve karo

```go
log.Printf("Server started at %s", addr)
if err := s.Serve(lis); err != nil {
    log.Fatalf("unable to start server: %v", err)
}
```

**Tumne `log.Printf` pehle rakha aur `Serve` baad me — sahi**. `Serve` block karta hai forever, agar baad me log likhte to kabhi dikhta nahi.

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
+----------------------------------+
|  _GreetService_GreetEveryone_    |  <- generated bridge (1 line)
|  Handler                         |
+----------------------------------+
       |
       v
+----------------------------------+
|  YOUR GreetEveryone handler      |
|    for {                         |
|      Recv()                      |
|      Send()                      |
|    }                             |
+----------------------------------+
```

---

## Common galtiyaan

| Bug | Lakshan | Fix |
|---|---|---|
| Port mismatch with client | `connection refused` on client side | Same port dono jagah |
| `Recv()` ka `io.EOF` check skip | Server infinite loop | Always check EOF first |
| `Send()` ka error ignore | Wasted work after client gone | Check `err` from `Send` and return |
| `log.Fatalf` in Recv error handler | Server crash on first client disconnect | `log.Printf` + `return err` |
| Send aur Recv ko 2 goroutines me chala dena jab Echo pattern hai | Unnecessary complexity, possible race | Echo pattern me single goroutine kaafi |
| Do `Send()` calls parallel goroutines me bina lock | Data race / corrupted stream | gRPC me Send se Send concurrent allowed nahi |
| Embed `pb.GreetRequest` instead of `Unimplemented...` | Compile error: interface satisfy nahi | Embed `pb.UnimplementedGreetServiceServer` |

---

## Production-version handler (better practices)

```go
func (s *server) GreetEveryone(stream grpc.BidiStreamingServer[pb.GreetRequest, pb.GreetResponse]) error {
    ctx := stream.Context()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        req, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }

        if req.GetFirstName() == "" {
            return status.Error(codes.InvalidArgument, "first_name required")
        }

        respondMessage := "Hello, " + req.GetFirstName()
        if err := stream.Send(&pb.GreetResponse{Result: respondMessage}); err != nil {
            return err
        }
    }
}
```

Differences vs current:

1. **`stream.Context()` se ctx**, `ctx.Done()` check har iteration me — client disconnect pe immediately exit.
2. **Input validation** — empty name reject with `codes.InvalidArgument`.
3. **`log.Printf` removed** — error already return ho raha, gRPC framework handle karega logging.

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `package main` + `func main()` | Executable binary |
| `type server struct { pb.Unimplemented... }` | Forced embedding for forward compat |
| `var addr = "localhost:8080"` | Listen address (⚠️ client se match karna chahiye) |
| `net.Listen("tcp", addr)` | TCP listener |
| `grpc.NewServer()` | gRPC brain |
| `pb.RegisterGreetServiceServer(s, &server{})` | Wiring |
| `s.Serve(lis)` | Accept loop, blocks forever |
| `func (s *server) GreetEveryone(stream)` | Tumhara bidirectional handler |
| `for { stream.Recv() / stream.Send() }` | Echo loop — bidirectional ka core |
| `err == io.EOF` | Client ne `CloseSend` call kar diya → handler done |
| `return nil` | Stream close hota gracefully (trailers) |

> **Bidirectional server ka one-liner**: "Loop me Recv karo, EOF aaye to return karo, warna Send karo response. Dono methods same handler me available hain — no goroutine zaruri Echo pattern me."
