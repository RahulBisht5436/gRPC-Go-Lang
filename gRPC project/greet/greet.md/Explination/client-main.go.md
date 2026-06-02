# `client/main.go` — gRPC client

Server toh ban gaya, ab usko **call** karne ke liye client chahiye. Iss file ka kaam: server se connect karo, `Greet` RPC call karo, response print karo.

## Pura file

```go
package main

import (
    "context"
    "log"
    "time"

    pb "example.com/greet/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

var addr = "localhost:50051"

func main() {
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

    res, err := client.Greet(ctx, &pb.GreetRequest{
        FirstName: "Rahul",
    })
    if err != nil {
        log.Fatalf("Greet RPC failed: %v", err)
    }

    log.Printf("Greet response: %s", res.GetResult())
}
```

---

## Client ke 4 mandatory steps

Server me 4 steps the (listen → server → register → serve). Client me bhi 4 hote hain:

1. **Connection banao** (`grpc.NewClient`).
2. **Typed stub banao** (`pb.NewGreetServiceClient`).
3. **Context banao** (deadline/timeout ke saath).
4. **RPC call karo** (jaise normal Go function).

---

## Line-by-line breakdown

### Imports

```go
import (
    "context"
    "log"
    "time"
    pb "example.com/greet/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)
```

- `context`, `log`, `time` — standard library.
- `pb` — server side ki tarah, generated proto package.
- `grpc` — gRPC runtime.
- `credentials/insecure` — TLS off karne ke liye (sirf dev).

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

> ⚠️ **Production me kabhi `insecure` mat use karo.** Vahan `credentials.NewTLS(...)` use hota hai with proper certs.

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

`conn` ek **raw pipe** hai — vo HTTP/2 frames bhej-le sakta hai but `Greet` ya `GreetRequest` ke baare me kuch nahi jaanta.

`pb.NewGreetServiceClient(conn)` is raw pipe ko ek **type-safe wrapper** me badal deta hai. Ab `client` variable pe har RPC ke liye ek method available hai (yaha sirf `Greet`):

```go
client.Greet(ctx, req)        // unary
client.GreetManyTimes(ctx, req)   // streaming, agar add karte
```

`NewGreetServiceClient` function bhi `protoc-gen-go-grpc` ne generate kiya tha:

```36:38:gRPC project/greet/proto/greet_grpc.pb.go
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

Saari gRPC calls **mandatory** rakhti hain context. Yaha hum 5-second timeout daal rahe hain — yaani agar 5 second me response nahi aaya, automatic cancel.

#### Anatomy:

- `context.Background()` — empty/root context. "Kuch deadline nahi, kuch cancellation nahi".
- `context.WithTimeout(parent, 5s)` — root pe 5-second deadline lagai.
- Returns 2 cheezein:
  - `ctx` — naya context jisme deadline hai.
  - `cancel` — ek function jo manually cancellation trigger karta hai.

#### `defer cancel()` kyu zaruri?

Agar tum `cancel()` call nahi karte, to `WithTimeout` ki internal goroutine 5 seconds tak alive rehti hai (memory leak). `defer cancel()` ensure karta hai jaise hi function return ho, cleanup ho jaaye. Even if RPC 1 second me return ho gayi, fir bhi `cancel()` call hoga — ye safe hai (idempotent).

> **Habit banao**: jab bhi `context.WithSomething(...)` likho, agli line pe `defer cancel()` likho. Hamesha.

### Step 4: RPC call

```go
res, err := client.Greet(ctx, &pb.GreetRequest{
    FirstName: "Rahul",
})
```

Ye **dikhne me** normal Go function call lagta hai. Lekin actually:

1. `client.Greet` generated stub method hai.
2. Vo `&pb.GreetRequest{FirstName: "Rahul"}` ko protobuf bytes me serialize karta hai.
3. `conn` ke through HTTP/2 request bhejta hai server ko, path = `/greet.GreetService/Greet`.
4. Server response ke bytes wapas aate hain.
5. Stub unhe `*pb.GreetResponse` me deserialize karke deta hai.

Tumhe ye saari magic dikhti nahi — bas function call jaisa feel hota hai. **Yahi gRPC ka asli charm hai.**

```go
if err != nil {
    log.Fatalf("Greet RPC failed: %v", err)
}
```

`err` non-nil hota hai jab:

- Network problem (server down, connection drop)
- Server ne `status.Error(codes.X, ...)` return kiya
- Timeout hua (`codes.DeadlineExceeded`)
- Cancellation (`codes.Canceled`)

### Final print

```go
log.Printf("Greet response: %s", res.GetResult())
```

Note `res.GetResult()` — same nil-safe getter pattern. Output milega:

```
Greet response: Hello Rahul
```

---

## Server vs Client compared

| Step | Server | Client |
|---|---|---|
| 1 | `net.Listen("tcp", addr)` | `grpc.NewClient(addr, ...)` |
| 2 | `grpc.NewServer()` | `pb.NewGreetServiceClient(conn)` |
| 3 | `pb.RegisterGreetServiceServer(s, &Server{})` | `context.WithTimeout(ctx, 5s)` |
| 4 | `s.Serve(lis)` | `client.Greet(ctx, req)` |

Symmetric structure hai — yaad rakhna easy.

---

## Run order

Dono ko alag-alag terminals me chalao:

```powershell
# Terminal 1 — server (pehle start karo)
cd "gRPC project\greet"
go run ./server

# Terminal 2 — client (server up hone ke baad)
cd "gRPC project\greet"
go run ./client
```

Output:

```
# server terminal:
2026/06/01 09:00:00 Listening on 0.0.0.0:50051
2026/06/01 09:00:05 Greet invoked with: Rahul

# client terminal:
2026/06/01 09:00:05 Greet response: Hello Rahul
```

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `grpc.NewClient(...)` | Lazy connection, not actual TCP yet |
| `insecure.NewCredentials()` | TLS off (dev only) |
| `defer conn.Close()` | Cleanup connection |
| `pb.NewGreetServiceClient(conn)` | Typed stub on top of raw conn |
| `context.WithTimeout(...)` | Deadline for the RPC |
| `defer cancel()` | Free context resources |
| `client.Greet(ctx, req)` | Actual network call (looks local) |
| `res.GetResult()` | nil-safe response field access |
