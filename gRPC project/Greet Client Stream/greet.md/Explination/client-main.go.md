# `client/main.go` — gRPC client (Client Streaming)

Server toh ban gaya, ab usko **call** karne ke liye client chahiye. Iss file ka kaam: server se connect karo, `SendUser` RPC ke liye stream open karo, **multiple names ko stream me Send karo**, fir `CloseAndRecv()` se single aggregate response receive karo.

## Pura file (tumhare current code)

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "example.com/clientStream/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr string = "localhost:3000"

func main() {
	connc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Not able to stablish the connection %v", err)
	}
	defer connc.Close()

	client := pb.NewUserServiceClient(connc)
	coxt, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	stream, err := client.SendUser(coxt)
	if err != nil {
		log.Fatalf("streaming didn't started %v", err)
	}

	names := []string{"Rahul Bisht", "Sheetal Bisht", "Kamal Bisht", "Pareshwari Bisht"}
	for _, name := range names {
		log.Printf("Sending the name : %v", name)
		err := stream.Send(&pb.UsersRequest{
			Name: name,
		})
		if err != nil {
			log.Printf("Not able to send the Request : %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Failed Request : %v", err)
	}

	fmt.Println("Successful Client Stream Done : %v", resp.GetResult())
}
```

---

## Client ke 6 steps (client streaming version)

Unary client me 4 steps the, server streaming me 5 (Recv loop). **Client streaming me 6** — Send loop + CloseAndRecv:

1. **Connection banao** (`grpc.NewClient`).
2. **Typed stub banao** (`pb.NewUserServiceClient`).
3. **Context banao** (deadline/timeout ke saath).
4. **Stream open karo** — `client.SendUser(ctx)` — **note: koi `req` parameter nahi!** Sirf `ctx`. Stream object return milta hai.
5. **Send loop chalao** — `stream.Send(&pb.UsersRequest{...})` baar-baar, har item ke liye ek.
6. **`CloseAndRecv()` call karo** — bata diya "ab aur kuch nahi bhejunga, response do". Ek hi single response milta hai.

**Yahi 5th aur 6th steps client streaming ki pehchaan hain.** Unary me ye nahi hota, server streaming me bhi nahi.

---

## Line-by-line breakdown

### Imports

```go
import (
    "context"
    "fmt"
    "log"
    "time"

    pb "example.com/clientStream/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)
```

- `context`, `log`, `time` — standard library.
- `fmt` — final response print karne ke liye.
- `pb` — generated proto package.
- `grpc` — gRPC runtime.
- `credentials/insecure` — TLS off karne ke liye (sirf dev).

> **Compare to server streaming client**: vahan `io` import tha (EOF detect karne ke liye). Yahaan **NAHI** chahiye — kyunki ab EOF server side pe aata hai, client side nahi. Direction reverse. Detail [conversation/streaming-direction-explained.md](../conversation/streaming-direction-explained.md) me.

### `var addr string = "localhost:3000"` — ⚠️ BUG!

```go
var addr string = "localhost:3000"
```

**Tumhare current code me ye galat hai.** Server `localhost:8080` pe listen karta hai:

```17:17:server/main.go
var addr string = "localhost:8080"
```

Client `localhost:3000` pe dial karta hai → kuch listening nahi → **`connection refused` error**:

```
2026/06/07 12:11:05 streaming didn't started rpc error: code = Unavailable desc = connection error:
desc = "transport: Error while dialing: dial tcp 127.0.0.1:3000: connectex:
No connection could be made because the target machine actively refused it."
```

**Fix**: dono files me same port karo. Ya tum client ko `"localhost:8080"` kar do, ya server ko `"localhost:3000"`. Most common pattern — port ko `const` me ek hi jagah rakho (ya env var / flag se inject karo).

#### How to verify port

```powershell
netstat -ano | findstr :8080
```

`LISTENING` state dikhna chahiye. Nahi dikha to server actually start nahi hua.

### Step 1: Connection banao

```go
connc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
```

`grpc.NewClient` modern API hai. Pehle `grpc.Dial` use hota tha — vo ab **deprecated** hai. New code me hamesha `NewClient` use karo.

> **Naming nit**: variable ka naam `connc` thoda awkward hai (extra `c`). Convention `conn` hota hai. Probably tumne `conn` se confused hote dekha hoga `pb.NewUserServiceClient(conn)` ke saath, lekin Go scope clear hai — `conn` chalega.

#### Important: `NewClient` actually connect nahi karta turant

Surprising fact: `grpc.NewClient` sirf ek **lazy** `*ClientConn` deta hai. Actual TCP connection tab tak nahi banta jab tak tum pehla RPC call nahi karte. Iska fayda — agar server temporary down hai, client crash nahi karta startup pe.

**Iss project me proof**: tumhara `grpc.NewClient` successfully return karta hai even agar port wrong hai. Actual error tab aata hai jab `client.SendUser(coxt)` call karte ho — vo line me `connection refused` milta hai.

#### `WithTransportCredentials(insecure.NewCredentials())` — TLS off

gRPC by default TLS expect karta hai. Local development me TLS setup karna jhanjat hai, isliye `insecure.NewCredentials()` use karte hain — "encryption mat lagao".

> ⚠️ **Production me kabhi `insecure` mat use karo.** Vahan `credentials.NewTLS(...)` use hota hai with proper certs. Client streaming me ye specially important — upload data unencrypted travel karega.

```go
if err != nil {
    log.Fatalf("Not able to stablish the connection %v", err)
}
```

`grpc.NewClient` se error tab aata hai jab address malformed hai (e.g., port number missing, weird format), ya configuration galat hai. Connection refused **yahaan nahi aata** (lazy hai).

### `defer connc.Close()`

Go ka **`defer`** keyword: jab function return hoga, tab ye line execute hogi. Yaani `main()` end hote hi connection band ho jaayega — resource leak nahi.

### Step 2: Typed stub banao

```go
client := pb.NewUserServiceClient(connc)
```

`connc` ek **raw pipe** hai — vo HTTP/2 frames bhej-le sakta hai but `SendUser` ya `UsersRequest` ke baare me kuch nahi jaanta.

`pb.NewUserServiceClient(connc)` is raw pipe ko ek **type-safe wrapper** me badal deta hai. Ab `client.SendUser(ctx)` available hai.

`NewUserServiceClient` function `protoc-gen-go-grpc` ne generate kiya tha:

```38:40:proto/client_grpc.pb.go
func NewUserServiceClient(cc grpc.ClientConnInterface) UserServiceClient {
	return &userServiceClient{cc}
}
```

Bas internally ek struct banata hai jisme `conn` chhupa hota hai.

### Step 3: Context with timeout

```go
coxt, cancel := context.WithTimeout(context.Background(), time.Second*5)
defer cancel()
```

> **Typo**: `coxt` should be `ctx`. Not a bug, just unusual naming. Convention: `ctx`. Mostly file me jahaan likha hai chalu hai.

Saari gRPC calls **mandatory** rakhti hain context. Yaha hum 5-second timeout daal rahe hain — yaani agar 5 second me **poora client streaming session** complete na ho, automatic cancel.

#### Client streaming me timeout ka matlab

5-second timeout = "**poora session** (stream open + saare `Send`s + `CloseAndRecv()` + final response) 5 sec me complete hona chahiye". Agar 4 sec me 3 names bhej diye aur 4th pe 6 sec lag gaya, to overall timeout exceed hoga aur stream cancel ho jaayega.

Real-world bade uploads me kabhi-kabhi `context.Background()` (no timeout) bhi acceptable hota hai — minutes/hours wale streams ke liye. But fir manual cancellation handle (`context.WithCancel`) zaruri hai.

#### `defer cancel()` kyu zaruri?

Agar tum `cancel()` call nahi karte, to `WithTimeout` ki internal goroutine 5 seconds tak alive rehti hai (memory leak). `defer cancel()` ensure karta hai jaise hi function return ho, cleanup ho jaaye.

> **Habit banao**: jab bhi `context.WithSomething(...)` likho, agli line pe `defer cancel()` likho. Hamesha.

### Step 4: Stream open karo

```go
stream, err := client.SendUser(coxt)
if err != nil {
    log.Fatalf("streaming didn't started %v", err)
}
```

**Yahan unary se sabse bada conceptual difference hai.** Look at the call:

| | Unary | Server streaming | **Client streaming** |
|---|---|---|---|
| Call | `client.Greet(ctx, req)` | `client.GreetManyTimes(ctx, req)` | **`client.SendUser(ctx)`** |
| Request | Parameter me | Parameter me | **PARAMETER ME NAHI!** Send via stream |
| Return | `(*Res, err)` | `(stream, err)` | `(stream, err)` |

**Notice — koi request parameter nahi.** Kyunki tum ek nahi, **multiple** requests bhejne wale ho — vo sab `stream.Send(...)` se baad me jaayenge.

`client.SendUser(coxt)` ki body `client_grpc.pb.go` me hai:

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

Ye internally:

1. HTTP/2 stream open kiya (`NewStream`).
2. Generic wrapper me wrap kiya.
3. **Bas stream object wapas diya — koi `SendMsg` ya `CloseSend` is constructor me nahi**. Server streaming me wo dono yahin ho jaate the (kyunki ek hi request thi). Client streaming me wo kaam caller pe hai.

#### Error kab aata hai

`err` non-nil tab hota hai jab stream open nahi ho saka — connection issue, malformed request, server unreachable. **Yahin tumhe port mismatch wala `connection refused` error milta hai** (kyunki `grpc.NewClient` lazy hai, actual TCP connect yahi attempt hota hai).

### Step 5: Send loop — client streaming ka heart

```go
names := []string{"Rahul Bisht", "Sheetal Bisht", "Kamal Bisht", "Pareshwari Bisht"}
for _, name := range names {
    log.Printf("Sending the name : %v", name)
    err := stream.Send(&pb.UsersRequest{
        Name: name,
    })
    if err != nil {
        log.Printf("Not able to send the Request : %v", err)
    }
}
```

#### Anatomy of one iteration

```go
err := stream.Send(&pb.UsersRequest{Name: name})
```

`stream.Send()`:

- Ek `*pb.UsersRequest` ko protobuf me encode karke wire pe DATA frame ke roop me bhejta hai.
- Server side me ye `stream.Recv()` ko trigger karta hai — server ko ek nayi request milti hai.
- Return karta hai `error`:
  - `nil` — successfully bheji.
  - Non-nil — kuch galat (network drop, server closed stream early, ctx timeout).

#### Important — error pe loop break karna chahiye

Tumhare current code me:

```go
if err != nil {
    log.Printf("Not able to send the Request : %v", err)
}
```

**Sirf log ho raha hai, loop continue ho raha hai.** Ye thoda risky pattern hai. Agar stream broken hai (`io.EOF` ya network error), agle `Send` bhi fail honge. Better:

```go
if err != nil {
    log.Printf("Send failed: %v", err)
    break  // exit loop, fir CloseAndRecv pe actual error pata chalega
}
```

> **Special case**: agar `Send` ne `io.EOF` return kiya, iska matlab **server ne stream early close kar di** (handler returned without reading all messages). Iss case me `CloseAndRecv` se actual error mil sakta hai jo server ne return ki.

#### Server side me kya ho raha hai parallel?

Jab tum `Send` × 4 karte ho, server side me:

```
client.Send("Rahul Bisht")     → server.Recv() returns &UsersRequest{Name: "Rahul Bisht"}
client.Send("Sheetal Bisht")   → server.Recv() returns &UsersRequest{Name: "Sheetal Bisht"}
client.Send("Kamal Bisht")     → server.Recv() returns &UsersRequest{Name: "Kamal Bisht"}
client.Send("Pareshwari Bisht")→ server.Recv() returns &UsersRequest{Name: "Pareshwari Bisht"}
```

Server `names` slice me sab collect kar leta hai, but **`SendAndClose` abhi nahi karta** — wait kar raha hai EOF ka.

### Step 6: `CloseAndRecv` — bata diya done, response lo

```go
resp, err := stream.CloseAndRecv()
if err != nil {
    log.Printf("Failed Request : %v", err)
}
```

`stream.CloseAndRecv()` ek hi call me 2 cheez karta hai:

1. **`CloseSend()`** internally call karta hai — server ko `END_STREAM` flag bhejta hai. Server side `stream.Recv()` ko `io.EOF` milta hai — ye uska "ab aur kuch nahi aayega" signal hai.
2. **Final response read** karta hai — server jab `SendAndClose(&Res)` karega, vahi response yahaan `resp` me aata hai.

Yahi reason hai ki client streaming me `Send` aur `Recv` symmetric nahi dikhte — client `Send` × N karta hai but receive sirf ek baar via `CloseAndRecv` se.

#### Error handling — tumhare code me ek issue

Tumhare current code:

```go
resp, err := stream.CloseAndRecv()
if err != nil {
    log.Printf("Failed Request : %v", err)
}

fmt.Println("Successful Client Stream Done : %v", resp.GetResult())
```

Agar `err` non-nil hai, `resp` nil hoga. **Phir bhi tum aage `resp.GetResult()` call karte ho.** Luckily `GetResult()` nil-safe hai (empty string return karega) — **crash nahi hoga**. But logically galat:

- "Successful" message print hoga even on failure.
- Result empty hoga, confusing log.

Better:

```go
resp, err := stream.CloseAndRecv()
if err != nil {
    log.Fatalf("Failed to receive aggregated response: %v", err)
}

fmt.Printf("Successful Client Stream Done: %s\n", resp.GetResult())
```

### Final print — ⚠️ BUG #2

```go
fmt.Println("Successful Client Stream Done : %v", resp.GetResult())
```

`fmt.Println` does **not** interpret `%v`. Output dikhega:

```
Successful Client Stream Done : %v hello, Rahul Bisht, Sheetal Bisht, Kamal Bisht, Pareshwari Bisht
```

Literal `%v` print hua! `Println` to bas argument-by-argument space se separate karke print karta hai.

**Fix** — `fmt.Printf` use karo:

```go
fmt.Printf("Successful Client Stream Done : %v\n", resp.GetResult())
//  ^^^^^^                                     ^^
//  ye        Printf format verbs interpret      \n manually add karna padta hai Printf me
```

Ya `Println` rakhna hai to format manually banao:

```go
fmt.Println("Successful Client Stream Done :", resp.GetResult())
```

---

## Send loop — common patterns

### Pattern 1: idiomatic loop with error break (recommended)

```go
for _, item := range items {
    if err := stream.Send(item); err != nil {
        log.Printf("send failed: %v", err)
        break
    }
}
```

### Pattern 2: read from channel (real-time producer)

```go
for item := range itemChan {
    if err := stream.Send(item); err != nil {
        return err
    }
}
```

### Pattern 3: from file in chunks (upload)

```go
buf := make([]byte, 4096)
for {
    n, err := file.Read(buf)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    if err := stream.Send(&pb.FileChunk{Data: buf[:n]}); err != nil {
        return err
    }
}
```

### Pattern 4: ctx-aware cancel

```go
for _, item := range items {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    if err := stream.Send(item); err != nil {
        return err
    }
}
```

---

## Server vs Client compared (client streaming)

| Step | Server | Client |
|---|---|---|
| 1 | `net.Listen("tcp", addr)` | `grpc.NewClient(addr, ...)` |
| 2 | `grpc.NewServer()` | `pb.NewUserServiceClient(conn)` |
| 3 | `pb.RegisterUserServiceServer(s, &server{})` | `context.WithTimeout(ctx, 5s)` |
| 4 | `s.Serve(lis)` (blocks forever) | `stream, _ := client.SendUser(ctx)` |
| 5 | Handler with `for { stream.Recv() }` loop | `for { stream.Send(...) }` loop |
| 6 | `stream.SendAndClose(&Res{...})` | `resp, _ := stream.CloseAndRecv()` |

Symmetric structure — client `Send` karta hai jab tak chahiye, server `Recv` karta hai jab tak EOF; fir server `SendAndClose` karta hai, client `CloseAndRecv` se vo response leta hai.

---

## Run order

Dono ko alag-alag terminals me chalao:

```powershell
# Terminal 1 — server (pehle start karo)
cd "gRPC project\Greet Client Stream"
go run ./server

# Terminal 2 — client (server up hone ke baad)
cd "gRPC project\Greet Client Stream"
go run ./client
```

**Pehle port fix karo dono files me!** Server `:8080`, client `:3000` — mismatch hai abhi.

Expected output (port fix ke baad):

```
# server terminal:
2026/06/07 12:30:00 Server listening on localhost:8080
2026/06/07 12:30:05 Received name: Rahul Bisht
2026/06/07 12:30:05 Received name: Sheetal Bisht
2026/06/07 12:30:05 Received name: Kamal Bisht
2026/06/07 12:30:05 Received name: Pareshwari Bisht

# client terminal:
2026/06/07 12:30:05 Sending the name : Rahul Bisht
2026/06/07 12:30:05 Sending the name : Sheetal Bisht
2026/06/07 12:30:05 Sending the name : Kamal Bisht
2026/06/07 12:30:05 Sending the name : Pareshwari Bisht
Successful Client Stream Done : hello, Rahul Bisht, Sheetal Bisht, Kamal Bisht, Pareshwari Bisht
```

(Plus the `%v` literal bug print until fixed.)

---

## Common galtiyaan

| Bug | Lakshan | Fix |
|---|---|---|
| Port mismatch server/client | `connection refused` | Same port dono jagah |
| `fmt.Println` with `%v` | Literal `%v` printed | Use `fmt.Printf(...\n)` |
| `CloseAndRecv` error pe bhi `resp.GetResult()` use | "Successful" message + empty result (luckily no crash due to nil-safe getter) | Error pe `log.Fatalf` ya `return` |
| `Send` loop me error pe continue | Multiple failed sends, confusing logs | `break` on first error |
| `client.SendUser(ctx, req)` likhna | Compile error: extra argument | Sirf `client.SendUser(ctx)` — no req parameter |
| `defer cancel()` bhulna | Goroutine leak | Hamesha `defer cancel()` |
| `CloseAndRecv` bhulna | Server EOF nahi milta, hang ho jaata hai | Must call `CloseAndRecv` to signal done |
| Try to call `Send` after `CloseAndRecv` | `EOF` error | Order matters — `Send` × N, then **exactly one** `CloseAndRecv` |

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `grpc.NewClient(...)` | Lazy connection, not actual TCP yet |
| `insecure.NewCredentials()` | TLS off (dev only) |
| `defer conn.Close()` | Cleanup connection |
| `pb.NewUserServiceClient(conn)` | Typed stub on top of raw conn |
| `context.WithTimeout(...)` | Deadline for the entire stream session |
| `defer cancel()` | Free context resources |
| `client.SendUser(ctx)` | **Stream open** — no request parameter! Returns stream object |
| `stream.Send(&Req{...})` | Ek request bhejna; multiple times call karte ho |
| `stream.CloseAndRecv()` | "Done sending" signal + single final response read |
| `resp.GetResult()` | nil-safe response field access |

> **Client-streaming client ka mental model**: `client.SendUser(ctx)` "stream open kiya (no request yet)", phir `for { Send(req) }` "messages bhej diye ek-ek karke", phir `CloseAndRecv()` "done bola + single response liya". Many calls → one response, instead of one call → one response (unary) ya one call → many responses (server streaming).
