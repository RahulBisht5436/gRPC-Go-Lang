# `client/main.go` — gRPC client (Server Streaming)

Server toh ban gaya, ab usko **call** karne ke liye client chahiye. Iss file ka kaam: server se connect karo, `GreetManyTimes` RPC call karo, **stream me** aane wali multiple responses ko loop me read karo aur print karo.

## Pura file

```go
package main

import (
    "context"
    "io"
    "log"
    "time"

    pb "example.com/greet/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

var addr = "localhost:50051"

func main() {
    // insecure.NewCredentials() = no TLS (local dev only).
    conn, err := grpc.NewClient(
        addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("Failed to create gRPC client for %s: %v", addr, err)
    }
    defer conn.Close()

    client := pb.NewGreetServiceClient(conn)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    stream, err := client.GreetManyTimes(ctx, &pb.GreetRequest{
        FirstName: "Rahul Bisht",
    })

    if err != nil {
        log.Fatalf("Greet RPC failed: %v", err)
    }

    for {
        res, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatalf("Streaming for the Request Failed")
        }
        log.Printf("Response: %s", res.GetResult())

    }
}
```

---

## Client ke 5 mandatory steps (streaming version)

Unary client me 4 steps the (connection → stub → context → call). Server streaming me **ek aur step add hota hai** — Recv loop:

1. **Connection banao** (`grpc.NewClient`).
2. **Typed stub banao** (`pb.NewGreetServiceClient`).
3. **Context banao** (deadline/timeout ke saath).
4. **Stream open karo** — `client.GreetManyTimes(ctx, req)` — **ye `*GreetResponse` nahi, balki `stream` object return karta hai**.
5. **Recv loop chalao** — `stream.Recv()` ko baar-baar call karke messages padhte raho jab tak `io.EOF` na aaye.

**Yahi 5th step hi server streaming ki pehchaan hai.** Unary me ye nahi hota — vahan ek RPC call = ek response.

---

## Line-by-line breakdown

### Imports

```go
import (
    "context"
    "io"          // <-- naya import: io.EOF detect karne ke liye
    "log"
    "time"
    pb "example.com/greet/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)
```

- `context`, `log`, `time` — standard library.
- **`io`** — server streaming ke liye **specially zaruri**. Stream khatam hone pe `stream.Recv()` `io.EOF` return karta hai, aur tumhe `err == io.EOF` check karna padta hai.
- `pb` — server side ki tarah, generated proto package.
- `grpc` — gRPC runtime.
- `credentials/insecure` — TLS off karne ke liye (sirf dev).

> Pichle (unary) version me `io` import nahi hota tha — wahan ek call → ek response, EOF concept nahi tha. Yahan aaya kyunki "stream khatam" ka signal `io.EOF` hi hai.

### `var addr = "localhost:50051"`

Notice — server me `0.0.0.0:50051` tha (listen everywhere), client me `localhost:50051` (connect to same machine). Difference:

- `0.0.0.0` — "kaha **se** sun raha hu" (server ka point of view: sab interfaces se).
- `localhost` / `127.0.0.1` — "kaha **se** connect kar raha hu" (client ka POV: same machine).

Agar server kisi remote machine pe hota, to client me `192.168.1.5:50051` jaisa kuch hota.

### Step 1: Connection banao

```go
conn, err := grpc.NewClient(
    addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

`grpc.NewClient` modern API hai. Pehle `grpc.Dial` use hota tha — vo ab **deprecated** hai. New code me hamesha `NewClient` use karo.

#### Important: `NewClient` actually connect nahi karta turant

Surprising fact: `grpc.NewClient` sirf ek **lazy** `*ClientConn` deta hai. Actual TCP connection tab tak nahi banta jab tak tum pehla RPC call nahi karte. Iska fayda — agar server temporary down hai, client crash nahi karta startup pe.

#### `WithTransportCredentials(insecure.NewCredentials())` — TLS off

gRPC by default TLS expect karta hai (yaani encrypted connection). Local development me TLS setup karna jhanjat hai, isliye `insecure.NewCredentials()` use karte hain — "encryption mat lagao".

> ⚠️ **Production me kabhi `insecure` mat use karo.** Vahan `credentials.NewTLS(...)` use hota hai with proper certs. Streaming me ye specially important — long-lived connections pe encryption critical hota hai.

```go
if err != nil {
    log.Fatalf("Failed to create gRPC client for %s: %v", addr, err)
}
```

`grpc.NewClient` se error tab aata hai jab address malformed hai (e.g., port number missing, weird format), ya configuration galat hai.

### `defer conn.Close()`

Go ka **`defer`** keyword: jab function return hoga, tab ye line execute hogi. Yaani `main()` end hote hi connection band ho jaayega — resource leak nahi.

### Step 2: Typed stub banao

```go
client := pb.NewGreetServiceClient(conn)
```

`conn` ek **raw pipe** hai — vo HTTP/2 frames bhej-le sakta hai but `GreetManyTimes` ya `GreetRequest` ke baare me kuch nahi jaanta.

`pb.NewGreetServiceClient(conn)` is raw pipe ko ek **type-safe wrapper** me badal deta hai. Ab `client` variable pe har RPC ke liye ek method available hai (yaha sirf `GreetManyTimes`):

```go
client.GreetManyTimes(ctx, req)   // server streaming
```

Agar proto me unary RPC bhi hota, vo bhi yahan available hota.

`NewGreetServiceClient` function `protoc-gen-go-grpc` ne generate kiya tha:

```36:38:proto/greet_grpc.pb.go
func NewGreetServiceClient(cc grpc.ClientConnInterface) GreetServiceClient {
	return &greetServiceClient{cc}
}
```

Bas internally ek struct banata hai jisme `conn` chhupa hota hai.

### Step 3: Context with timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

Saari gRPC calls **mandatory** rakhti hain context. Yaha hum 5-second timeout daal rahe hain — yaani agar 5 second me **poora stream** complete na ho, automatic cancel.

#### Streaming me timeout ka matlab thoda alag hota hai

Unary me 5-second timeout = "single response 5 sec me aana chahiye".

Server streaming me 5-second timeout = "**poora stream** (saare messages + final close) 5 sec me aana chahiye". Agar server 4 messages 1 sec me bhej de aur 5th pe 6 sec laga, to overall timeout exceed hoga aur stream cancel ho jaayega.

Real-world streaming me kabhi-kabhi `context.Background()` (no timeout) bhi acceptable hota hai — long-lived streams (hours, days) ke liye. But fir manual cancellation handle (`context.WithCancel`) zaruri hai.

#### `defer cancel()` kyu zaruri?

Agar tum `cancel()` call nahi karte, to `WithTimeout` ki internal goroutine 5 seconds tak alive rehti hai (memory leak). `defer cancel()` ensure karta hai jaise hi function return ho, cleanup ho jaaye.

> **Habit banao**: jab bhi `context.WithSomething(...)` likho, agli line pe `defer cancel()` likho. Hamesha.

### Step 4: Stream open karo

```go
stream, err := client.GreetManyTimes(ctx, &pb.GreetRequest{
    FirstName: "Rahul Bisht",
})
```

**Yahan unary se sabse bada conceptual difference hai.** Look at the return type:

| | Unary | Server streaming |
|---|---|---|
| Return | `res *pb.GreetResponse, err error` | `stream grpc.ServerStreamingClient[pb.GreetResponse], err error` |
| Result | Direct response struct mil gaya | Sirf ek **stream object** mila — abhi koi data nahi |

`client.GreetManyTimes(...)` return karta hai — ye method `greet_grpc.pb.go` me defined hai:

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

Ye internally:

1. HTTP/2 stream open kiya (`NewStream`).
2. Tumhari request `&pb.GreetRequest{FirstName: "Rahul Bisht"}` ko bytes me convert karke wire pe bheji (`SendMsg`).
3. `CloseSend()` call kiya — server ko bata diya "ab aur kuch nahi bhejunga".
4. Tumhe stream object (`x`) wapas diya — tum is pe `Recv()` chala sakte ho.

Aur ye sab **yahan tak server-side handler shayad chal bhi nahi raha** — server ko request mili, dispatch ho gaya, handler chalu. Lekin tumne kuch padha nahi yet. Padhne ke liye Step 5 hai.

```go
if err != nil {
    log.Fatalf("Greet RPC failed: %v", err)
}
```

`err` non-nil tab hota hai jab stream open nahi ho saka — connection issue, malformed request, server unreachable. **Note**: stream open hone ke baad `err` aane wali hain `Recv()` calls me, isme nahi.

### Step 5: Recv loop — server streaming ka heart

```go
for {
    res, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatalf("Streaming for the Request Failed")
    }
    log.Printf("Response: %s", res.GetResult())
}
```

Ye **infinite for loop** hai jo server se messages padhta jaata hai jab tak EOF na aaye.

#### Anatomy of one iteration

```go
res, err := stream.Recv()
```

`stream.Recv()` block karta hai jab tak:

- ek naya `*pb.GreetResponse` aaye → `res` me, `err = nil`
- stream gracefully close ho jaaye → `res = nil`, `err = io.EOF`
- error aa jaaye → `res = nil`, `err = some error`

#### `if err == io.EOF { break }` — clean exit

Server jab apna handler return karta hai (`return nil`), gRPC framework HTTP/2 trailers me `END_STREAM` flag bhejta hai. Client side `Recv()` is ko detect karke `io.EOF` deta hai — yahi hai "stream khatam, koi aur message nahi aayega" ka standard signal.

> **Important nuance**: `io.EOF` ek **sentinel error** hai (specific reference). `err == io.EOF` comparison kaam karta hai. Lekin agar future me wrapped error use karna ho to `errors.Is(err, io.EOF)` zyada robust pattern hai.

#### `if err != nil { log.Fatalf(...) }` — error case

Yahan agar `err` non-nil hai aur `io.EOF` nahi hai, to **kuch galat hua**:

- Server ne handler me error return kiya (`status.Error(codes.Internal, ...)`)
- Network drop ho gaya
- Context timeout ya cancel hua

Tumhare current code me sirf `log.Fatalf("Streaming for the Request Failed")` likha hai — **error variable pass nahi kiya**. Better:

```go
if err != nil {
    log.Fatalf("Streaming failed: %v", err)
}
```

Iss tarah actual error message dikhega, jisme grpc status code (`code = Internal desc = ...`) bhi included hoga — debugging bohot easy.

#### `log.Printf("Response: %s", res.GetResult())` — print

Har response message ka `Result` field print karta hai. `res.GetResult()` nil-safe getter hai (just like server side ka `in.GetFirstName()`).

10 iterations ke baad output aisa milega:

```
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 0
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 1
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 2
...
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 9
```

(Server me delay nahi hai to sab ek dum se aayenge.)

---

## Recv loop — common patterns

### Pattern 1: idiomatic loop (jaise tumne likha)

```go
for {
    res, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    // process res
}
```

### Pattern 2: errors.Is for safety

```go
for {
    res, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    // process res
}
```

### Pattern 3: collect into slice

```go
var results []*pb.GreetResponse
for {
    res, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return nil, err
    }
    results = append(results, res)
}
return results, nil
```

### Pattern 4: ctx-aware cancel from outside

```go
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    res, err := stream.Recv()
    // ... handling
}
```

(`Recv` already ctx-aware hota hai — ye usually optional check hai.)

---

## Server vs Client compared (server streaming)

| Step | Server | Client |
|---|---|---|
| 1 | `net.Listen("tcp", addr)` | `grpc.NewClient(addr, ...)` |
| 2 | `grpc.NewServer()` | `pb.NewGreetServiceClient(conn)` |
| 3 | `pb.RegisterGreetServiceServer(s, &Server{})` | `context.WithTimeout(ctx, 5s)` |
| 4 | `s.Serve(lis)` (blocks forever) | `stream, _ := client.GreetManyTimes(ctx, req)` |
| 5 | Handler with `stream.Send(...)` loop | Client with `stream.Recv()` loop |

Symmetric structure — server `Send` karta hai jab tak return na ho, client `Recv` karta hai jab tak EOF na aaye.

---

## Run order

Dono ko alag-alag terminals me chalao:

```powershell
# Terminal 1 — server (pehle start karo)
cd "gRPC project\greet Server Streaming"
go run ./server

# Terminal 2 — client (server up hone ke baad)
cd "gRPC project\greet Server Streaming"
go run ./client
```

Output:

```
# server terminal:
2026/06/06 23:30:00 Listening on 0.0.0.0:50051
Stream Function initiated

# client terminal:
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 0
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 1
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 2
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 3
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 4
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 5
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 6
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 7
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 8
2026/06/06 23:30:00 Response: Changes for the User Rahul Bisht and time 9
```

10 lines — ek-ek `Send()` se ek-ek `Recv()` corresponding. Phir client clean exit kar deta hai (loop me `io.EOF` aane se `break`).

---

## Common galtiyaan

| Bug | Lakshan | Fix |
|---|---|---|
| `Recv()` loop me `if err == io.EOF` skip karna | Loop kabhi exit nahi karega, bas blocked rahega ya panic karega `nil` deref pe | Always check `io.EOF` first |
| Error me `err` variable include na karna | Log me sirf "Failed" likha — kya hua nahi pata | `log.Fatalf("...: %v", err)` |
| `defer cancel()` bhulna | Goroutine leak | Hamesha `defer cancel()` |
| `client.GreetManyTimes(ctx, req)` ka return type `*GreetResponse` expect karna | Compile error | Vo stream object hai, `*GreetResponse` nahi |
| Stream pe `Send` call karne ki koshish (client side) | Compile error / runtime error | Server streaming me client sirf ek baar request bhejta hai (vo `client.GreetManyTimes(...)` call ke andar pehle hi ho jaata hai) |

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `grpc.NewClient(...)` | Lazy connection, not actual TCP yet |
| `insecure.NewCredentials()` | TLS off (dev only) |
| `defer conn.Close()` | Cleanup connection |
| `pb.NewGreetServiceClient(conn)` | Typed stub on top of raw conn |
| `context.WithTimeout(...)` | Deadline for the entire stream |
| `defer cancel()` | Free context resources |
| `client.GreetManyTimes(ctx, req)` | Stream **open** karne ki call (returns stream object, not data) |
| `stream.Recv()` | Ek message padhna; multiple times call hota hai |
| `err == io.EOF` | Stream khatam ka signal |
| `res.GetResult()` | nil-safe response field access |

> **Server streaming client ka mental model**: `GreetManyTimes(...)` "stream open kiya", phir `for { Recv() }` "messages padhe jab tak EOF". Ek call → many responses, instead of ek call → ek response.
