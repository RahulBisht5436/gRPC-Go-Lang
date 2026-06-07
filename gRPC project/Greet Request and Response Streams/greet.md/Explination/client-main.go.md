# `client/main.go` — gRPC client (Bidirectional Streaming)

Yahaan client streaming wala client-side se zyada complex ho jaata hai. Reason — **dono Send aur Recv parallel** chalane hain, isliye **2 goroutines** chahiye. Ek background goroutine `Recv` loop chalati hai, main goroutine `Send` loop. Aur dono ko sync karna ke liye ek `waitc` channel use hota hai.

## Pura file (tumhare current code)

```go
package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "example.com/bidirectional/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = "localhost:50051"

func main() {

	names := []string{"Rahul bisht", "Sheetal Bisht", "kamal bisht", "Pareshwari Bishr"}
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
	stream, err := client.GreetEveryone(ctx)
	if err != nil {
		log.Printf("Unable to send the request , Reason : %v", err)
	}
	//Need to have more clear understanding of this part
	waitc := make(chan struct{})

	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("Recv error : %v ", err)
			}
			log.Printf("Response: %s", res.GetResult())
		}
	}()

	for _, name := range names {
		log.Printf("Sending: %s", name)
		if err := stream.Send(&pb.GreetRequest{FirstName: name}); err != nil {
			log.Fatalf("Send error: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("CloseSend error: %v", err)
	}
	<-waitc
	log.Println("Done")
}
```

---

## Client ke 7 steps (bidirectional streaming version)

Char modes me steps badhte gaye:

- Unary: 4 (conn → stub → ctx → call)
- Server streaming: 5 (+ Recv loop)
- Client streaming: 6 (+ Send loop + CloseAndRecv)
- **Bidirectional: 7** (+ goroutine spawn + Send loop + CloseSend + wait)

1. **Connection banao** (`grpc.NewClient`).
2. **Typed stub banao** (`pb.NewGreetServiceClient`).
3. **Context banao** (deadline/timeout ke saath).
4. **Stream open karo** — `client.GreetEveryone(ctx)` — no request parameter.
5. **Background goroutine spawn karo** jisme `Recv` loop chalega.
6. **Main goroutine me Send loop** chalao, har item ke liye ek `stream.Send(...)`.
7. **`CloseSend()` call karo** + **`<-waitc` se wait karo** background goroutine done hone ka.

---

## Line-by-line breakdown

### Imports

```go
import (
    "context"
    "io"        // <-- io.EOF detect karne ke liye (client side me bhi chahiye bidirectional me)
    "log"
    "time"

    pb "example.com/bidirectional/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)
```

- `io` — **bidirectional me dono sides pe `io.EOF` mil sakta hai**. Server side ko milta jab client `CloseSend` kare, client side ko milta jab server `return nil` kare handler se. To client me bhi `io` chahiye.
- `time` — `time.Sleep` se pacing ke liye.
- Baaki standard.

### `var addr = "localhost:50051"` — ⚠️ BUG #1!

**Tumhare current code me ye galat hai.** Server `localhost:8080` pe listen karta hai:

```16:16:server/main.go
var addr = "localhost:8080"
```

Client `localhost:50051` pe dial karta → kuch listening nahi → **`connection refused` error**:

```
rpc error: code = Unavailable desc = connection error:
desc = "transport: Error while dialing: dial tcp 127.0.0.1:50051: connectex:
No connection could be made because the target machine actively refused it."
```

**Fix**: dono files me same port. Mostly client wali value (`50051` — gRPC conventional) behtar hai, to server ko `localhost:50051` kar do.

### `names := []string{...}`

```go
names := []string{"Rahul bisht", "Sheetal Bisht", "kamal bisht", "Pareshwari Bishr"}
```

Test data — 4 names jo Send loop me ek-ek karke jaayenge.

### Step 1: Connection banao

```go
conn, err := grpc.NewClient(
    addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    log.Fatalf("Failed to create gRPC client for %s: %v", addr, err)
}
defer conn.Close()
```

Standard. `grpc.NewClient` lazy hai — actual connect tab hota jab pehli RPC call hoti hai (step 4).

`defer conn.Close()` cleanup ensure karta hai `main()` exit pe.

### Step 2: Typed stub banao

```go
client := pb.NewGreetServiceClient(conn)
```

Raw conn ko type-safe wrapper me badla.

### Step 3: Context with timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

5-second timeout = **poora bidirectional session** 5 sec me complete hona chahiye. Real chat apps me typically `context.Background()` (no timeout) use hota hai with manual cancellation via `context.WithCancel`.

### Step 4: Stream open karo — ⚠️ BUG #2 ka root!

```go
stream, err := client.GreetEveryone(ctx)
if err != nil {
    log.Printf("Unable to send the request , Reason : %v", err)
}
```

**Yahaan critical bug hai**: `log.Printf` use hua hai, `log.Fatalf` nahi.

Agar `client.GreetEveryone(ctx)` fail hua (jaise abhi connection refused se), to:

1. `err != nil`, `stream = nil`
2. `log.Printf(...)` sirf print karta hai — **program continue ho jaata hai**
3. Code aage chala jaata hai aur `stream` ka use karne lagta hai
4. Line 42 me background goroutine `stream.Recv()` call karta hai — **`stream` nil hai** → **`invalid memory address or nil pointer dereference` panic**

Stack trace exactly yahi dikhaata hai:

```
panic: runtime error: invalid memory address or nil pointer dereference
goroutine 14 [running]:
main.main.func1()
        client/main.go:42 +0xc4
created by main.main in goroutine 1
        client/main.go:40 +0x375
```

**Fix**: `log.Printf` → `log.Fatalf` (ya `return`):

```go
stream, err := client.GreetEveryone(ctx)
if err != nil {
    log.Fatalf("Unable to send the request, Reason: %v", err)
}
```

> **Rule of thumb**: agar `err != nil` ke baad agla code uss variable pe **depend** karta hai (yahaan `stream`), to error handling me **execution rokna mandatory** hai — `return`, `log.Fatalf`, ya `panic`. `log.Printf` se sirf log hota hai aur nil dereference se cascade crash hota hai.

### Step 5: `waitc` channel + background goroutine

```go
waitc := make(chan struct{})

go func() {
    for {
        res, err := stream.Recv()
        if err == io.EOF {
            close(waitc)
            return
        }
        if err != nil {
            log.Fatalf("Recv error : %v ", err)
        }
        log.Printf("Response: %s", res.GetResult())
    }
}()
```

#### `waitc := make(chan struct{})` — ye kya hai?

Ye ek **"done signal" channel** hai. Detail [conversation/goroutines-and-waitc-explained.md](../conversation/goroutines-and-waitc-explained.md) me — but quick summary:

- `chan struct{}` — value-less channel. **Sirf signal ke liye, data ke liye nahi.**
- `struct{}` zero-byte type hai — memory efficient.
- Use case: "ek goroutine doosri goroutine ko bata de ki main done hu" — bina koi value transfer.

Pattern:

```go
waitc := make(chan struct{})   // create unbuffered signal channel

// In goroutine A:
close(waitc)   // signal "I'm done"

// In goroutine B:
<-waitc        // block until A signals
```

Yahi tumne use kiya hai.

#### Background goroutine ka kaam

```go
go func() {
    for {
        res, err := stream.Recv()         // <-- block until server sends or EOF
        if err == io.EOF {
            close(waitc)                  // <-- "done" signal main goroutine ko
            return                        // <-- goroutine exit
        }
        if err != nil {
            log.Fatalf("Recv error : %v ", err)
        }
        log.Printf("Response: %s", res.GetResult())
    }
}()
```

Step-by-step:

1. Background goroutine spawn hua (`go func() { ... }()`).
2. Infinite loop me `stream.Recv()` block karta hai jab tak server message bheje.
3. Server message bheje (`stream.Send(&Res{...})` server side me) → `Recv()` return karta `res, nil` — print karte hain.
4. Server handler return karta (`return nil`) → trailers wire pe → client side `Recv()` ko `io.EOF` milta.
5. `close(waitc)` se main goroutine ko signal: "I am done, you can exit now".
6. `return` se background goroutine exit.

#### Kyu zaruri hai goroutine?

**Sabse important question** — yahaan goroutine kyu chahiye? Sequential code se kaam nahi hota?

Sequential code:

```go
// Sequential — WRONG for bidirectional
for _, name := range names {
    stream.Send(&pb.GreetRequest{FirstName: name})
    res, _ := stream.Recv()      // <-- block here
    log.Printf("Response: %s", res.GetResult())
}
```

Ye **theoretically chal sakta hai** echo pattern me (kyunki har Send ke baad ek Recv ka guarantee hai). LEKIN:

1. Server pattern non-echo ho — e.g., server N requests ke baad 1 response bheje (aggregate). Tum har iteration me Recv block ho jaaoge while server abhi tak response nahi bhej raha.
2. Server ek pehli "welcome" message bheje pehle koi request mile (subscription style). Tum sequential code me first Send karne se pehle Recv kabhi nahi karoge.
3. **Deadlock scenarios** — agar Send buffer full ho aur server response wait kar raha ho — sequential code dono Send aur Recv ek hi goroutine me hone se hang ho jaayega.

**Bidirectional me dono goroutines ka pattern is the safe, general-purpose way.** Detail [conversation/goroutines-and-waitc-explained.md](../conversation/goroutines-and-waitc-explained.md) me.

#### Notice — `log.Fatalf` inside goroutine

```go
if err != nil {
    log.Fatalf("Recv error : %v ", err)
}
```

`log.Fatalf` `os.Exit(1)` call karta hai — **pure program exit hota hai**, sirf goroutine nahi. Iss case me acceptable hai (fatal error pe kuch karna nahi), but better pattern:

```go
if err != nil {
    log.Printf("Recv error: %v", err)
    close(waitc)
    return
}
```

Ye main goroutine ko bhi gracefully exit karne deta hai (defer cleanup chalega).

### Step 6: Main goroutine — Send loop

```go
for _, name := range names {
    log.Printf("Sending: %s", name)
    if err := stream.Send(&pb.GreetRequest{FirstName: name}); err != nil {
        log.Fatalf("Send error: %v", err)
    }
    time.Sleep(300 * time.Millisecond)
}
```

Main goroutine alag se `Send` chala raha hai parallel to background `Recv`. Step-by-step:

1. Har name ke liye ek `stream.Send(&pb.GreetRequest{FirstName: name})` call.
2. Server side me `stream.Recv()` ko ye message milega.
3. Server handler `stream.Send(&Res{...})` se echo bhejega.
4. Background goroutine me `stream.Recv()` ko vo response milega — print hoga.
5. **All this happens parallel** — main Send karta jaata, background Recv karta jaata.

#### `time.Sleep(300 * time.Millisecond)` — pacing

```go
time.Sleep(300 * time.Millisecond)
```

Har Send ke beech 300ms gap. Real-world streaming me ye natural delays hote hain (user typing speed, sensor sampling rate, etc.). **Yahaan demo ke liye visible interleaving dikhane ke liye add kiya** — bina sleep ke 4 sends ek dum ho jaate, fir 4 responses bhi ek dum, parallel demonstration kam visible.

Expected interleaved output:

```
12:30:00.000 Sending: Rahul bisht
12:30:00.001 Response: Hello, Rahul bisht
12:30:00.301 Sending: Sheetal Bisht
12:30:00.302 Response: Hello, Sheetal Bisht
12:30:00.602 Sending: kamal bisht
12:30:00.603 Response: Hello, kamal bisht
12:30:00.903 Sending: Pareshwari Bishr
12:30:00.904 Response: Hello, Pareshwari Bishr
12:30:00.905 Done
```

**Sending aur Response interleaved hain — ye prove karta hai 2 goroutines parallel chal rahi hain.**

### Step 7: `CloseSend()` + `<-waitc`

```go
if err := stream.CloseSend(); err != nil {
    log.Fatalf("CloseSend error: %v", err)
}
<-waitc
log.Println("Done")
```

#### `stream.CloseSend()` — "ab aur kuch nahi bhejunga"

Ye **mandatory call hai bidirectional client side me**. Iska kaam:

- Wire pe HTTP/2 `END_STREAM` flag bhejta hai upstream direction me.
- Server side `stream.Recv()` ko `io.EOF` milta hai.
- Server handler `return nil` kar deta hai.
- Server side se trailers aate hain downstream.
- Client side `stream.Recv()` ko `io.EOF` milta hai (background goroutine me).

Bina `CloseSend` ke:

- Server forever `Recv` block rahega — kabhi `return nil` nahi karega.
- Background goroutine forever `Recv` block rahegi — kabhi EOF nahi milega.
- `<-waitc` forever block rahega.
- Program hang ho jaayega.

**`CloseSend` skip karna sabse common bidirectional bug hai. Always remember.**

#### `<-waitc` — wait for background goroutine

```go
<-waitc
```

Ye line **main goroutine ko block karti hai** jab tak background goroutine `close(waitc)` na call kare.

Kyu zaruri:

1. `CloseSend()` ke baad bhi server ke responses aana baki ho sakte hain (server ne queue me daal rakhe ho).
2. Background goroutine vo responses receive karke print karegi.
3. Agar tum `main()` return karte ho `<-waitc` ke bina, **pura program exit ho jaayega** including background goroutine — un print messages kabhi nahi dikhenge.
4. `<-waitc` ensure karta hai main tab tak wait kare jab tak background "done" bole.

#### `log.Println("Done")` — final log

Background goroutine clean exit ho gayi → `waitc` close ho gaya → main goroutine unblock → "Done" print hota.

---

## Diagram: 2 goroutines ka coordination

```
MAIN GOROUTINE                          BACKGROUND GOROUTINE
─────────────────                       ────────────────────
                                        (not yet started)
client.GreetEveryone(ctx) → stream
waitc := make(chan struct{})
go func() { ... }()  ─────spawn────►   for { stream.Recv() ... }
                                        (blocks on first Recv)

for _, name := range names {
   stream.Send(req) ──wire──►
                                        ←──wire── res from server
                                        stream.Recv() unblocks
                                        log.Printf("Response: ...")
                                        (loop: blocks on next Recv)
   sleep 300ms

   stream.Send(req) ──wire──►
                                        ←──wire── res from server
                                        Recv unblocks
                                        print
   sleep 300ms
   ... (4 names total) ...

   stream.Send(req) ──wire──►
                                        ←── res
                                        print
}

stream.CloseSend() ──wire──►            (server gets EOF, returns nil)
                                        (server sends trailers)
                                        ←──wire── EOF
                                        stream.Recv() returns io.EOF
                                        close(waitc) ──signal──►
<-waitc unblocks    ◄───────signal──────
log.Println("Done")
return → main() exit                    (goroutine returned already)
```

---

## Common galtiyaan

| Bug | Lakshan | Fix |
|---|---|---|
| Port mismatch server/client | `connection refused` | Same port dono jagah |
| `log.Printf` on stream-open error | Nil panic in goroutine on line 42 | `log.Fatalf` ya `return` |
| `CloseSend` bhulna | Program hangs forever | Always call after Send loop |
| `<-waitc` bhulna | Last few responses kabhi print nahi hote (main exits first) | Always wait |
| Sequential code (no goroutine) | Deadlock in non-echo patterns | Use goroutine for Recv |
| `stream.Send` after `CloseSend` | Returns error | Order matters — Send all, then CloseSend |
| Goroutine inside main panic (e.g., `Recv` on nil stream) | Whole program crash | Validate stream creation success first |
| `defer cancel()` bhulna | Context leak | Hamesha `defer cancel()` |
| `close(waitc)` ko 2 baar call karna | Panic: close of closed channel | Sirf ek hi exit path se close karo |

---

## Production version

```go
func main() {
    conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("dial: %v", err)
    }
    defer conn.Close()

    client := pb.NewGreetServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stream, err := client.GreetEveryone(ctx)
    if err != nil {
        log.Fatalf("open stream: %v", err)        // <-- Fatalf, not Printf
    }

    errc := make(chan error, 1)
    go func() {
        for {
            res, err := stream.Recv()
            if err == io.EOF {
                errc <- nil
                return
            }
            if err != nil {
                errc <- err
                return
            }
            log.Printf("Response: %s", res.GetResult())
        }
    }()

    names := []string{"Rahul bisht", "Sheetal Bisht", "kamal bisht", "Pareshwari Bishr"}
    for _, name := range names {
        select {
        case <-ctx.Done():
            log.Fatalf("ctx done while sending: %v", ctx.Err())
        default:
        }

        log.Printf("Sending: %s", name)
        if err := stream.Send(&pb.GreetRequest{FirstName: name}); err != nil {
            log.Fatalf("send: %v", err)
        }
        time.Sleep(300 * time.Millisecond)
    }

    if err := stream.CloseSend(); err != nil {
        log.Fatalf("CloseSend: %v", err)
    }

    if err := <-errc; err != nil {
        log.Fatalf("recv loop: %v", err)
    }
    log.Println("Done")
}
```

Improvements:

1. `errc chan error` instead of `chan struct{}` — error bhi propagate hota hai goroutine se main tak.
2. `select { case <-ctx.Done() }` me Send loop — context cancel pe bail.
3. All `Printf` → `Fatalf` for `err != nil` paths.

---

## TL;DR

| Cheez | Purpose |
|---|---|
| `grpc.NewClient(...)` | Lazy connection |
| `pb.NewGreetServiceClient(conn)` | Typed stub |
| `context.WithTimeout(...)` | Deadline for entire session |
| `defer cancel()` | Free context resources |
| `client.GreetEveryone(ctx)` | **Stream open** — no request param |
| `waitc := make(chan struct{})` | Signal channel for goroutine sync |
| `go func() { for { Recv() } }()` | Background Recv loop |
| `close(waitc)` (inside goroutine) | Signal "done" to main |
| `stream.Send(req)` (in main loop) | Send each request |
| `stream.CloseSend()` | "Done sending" — server gets EOF |
| `<-waitc` (in main) | Block until background goroutine done |

> **Bidirectional client ka mental model**: "stream khol, ek goroutine ko Recv loop pe chhod, main me Send loop chala, end pe `CloseSend` + `<-waitc` se sync. Dono goroutines independently kaam karte hain — yahi power of bidirectional streaming."
