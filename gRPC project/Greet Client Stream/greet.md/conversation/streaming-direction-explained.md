# Streaming direction — server streaming vs client streaming ka **direction reversal**

> Iss page ka motive: gRPC streaming me ek subtle but **bahut important** concept hai — "kaun Send karta hai, kaun Recv karta hai, kaun close karta hai, kaun EOF receive karta hai". Server streaming aur client streaming dono me ye **mirror image** dikhte hain. Ek baar clear ho gaya, fir bidirectional streaming free me samajh aa jaata hai.

## TL;DR (poora gist ek table me)

| Action | Unary | Server Streaming | **Client Streaming (current project)** | Bidirectional |
|---|---|---|---|---|
| Client transmit | Request as parameter | Request as parameter | **`stream.Send(req)` × N** | `stream.Send(req)` × N |
| Server receive | Request as parameter | Request as parameter | **`stream.Recv()` loop** | `stream.Recv()` loop |
| Server transmit | `return &Res{}, nil` | `stream.Send(res)` × N | **`return stream.SendAndClose(&Res{})`** | `stream.Send(res)` × N |
| Client receive | Returned `(*Res, err)` | `stream.Recv()` loop | **`stream.CloseAndRecv()`** (single) | `stream.Recv()` loop |
| Who closes the upstream? | n/a | gRPC auto (1 request done) | **Client explicitly via `CloseSend`/`CloseAndRecv`** | Client explicitly |
| Who closes the downstream? | gRPC auto | Server returns from handler | Server returns from handler (`SendAndClose` triggers it) | Server returns |
| Who gets `io.EOF`? | n/a | **Client** (from `Recv()`) | **Server** (from `Recv()`) | Both (each on their own Recv) |

**Yaad rakhne ka tareeka**: jo side **send karta hai stream me**, vo "owner" hota hai close ka. Aur jis side ka Recv stream end pe call ho raha, **uss side ko `io.EOF` milta hai**.

---

## Server streaming — direction (pichla project)

```
CLIENT                                  SERVER
  |                                       |
  | client.GreetManyTimes(ctx, req)       |
  | ───── request (single) ──────────────►|
  |                                       |  <handler chalu>
  |                                       |
  |◄──── res 1 (DATA frame) ──────────────| stream.Send(res1)
  |◄──── res 2 (DATA frame) ──────────────| stream.Send(res2)
  | ...                                   | ...
  |◄──── res N (DATA frame) ──────────────| stream.Send(resN)
  |◄──── trailers (END_STREAM) ───────────| return nil  // handler done
  |                                       |
  | stream.Recv() → io.EOF  ◄──── EOF arrives HERE on client side
```

- **Client** ek hi request bhejta hai (constructor ke andar auto)
- **Server** loop me `Send` karta hai
- **Server return** karta hai → gRPC trailers bhejta hai
- **Client `Recv()`** ko `io.EOF` milta hai

EOF **client side** pe land karta hai. `io` import client me chahiye tha.

---

## Client streaming — direction (current project)

```
CLIENT                                  SERVER
  |                                       |
  | client.SendUser(ctx)                  |
  | ───── stream open (no req yet) ──────►|
  |                                       |  <handler chalu>
  | stream.Send(req1) ───── DATA ────────►|  stream.Recv() → req1
  | stream.Send(req2) ───── DATA ────────►|  stream.Recv() → req2
  | ...                                   |  ...
  | stream.Send(reqN) ───── DATA ────────►|  stream.Recv() → reqN
  |                                       |
  | stream.CloseAndRecv()                 |
  |   - sends END_STREAM upstream ───────►|  stream.Recv() → io.EOF ◄── EOF arrives HERE on server side
  |   - waits for response                |  <build aggregate response>
  |                                       |
  |◄──── single Response (DATA) ──────────|  stream.SendAndClose(&Res{...})
  |◄──── trailers ────────────────────────|  (return from handler)
  |                                       |
  | resp, _ := stream.CloseAndRecv() done |
```

- **Client** loop me `Send` karta hai
- **Client** `CloseAndRecv` se "done" signal deta hai
- **Server `Recv()`** ko `io.EOF` milta hai (EOF abhi yahaan land kiya — server pe)
- **Server** ek aggregate response banata aur `SendAndClose` se bhejta hai

EOF **server side** pe land karta hai. `io` import ab server me chahiye, client me nahi. **Yahi reversal hai.**

---

## Confusion: alternate "close" methods

gRPC me 3 different "close" methods hain — kab kya use kare, samajhna zaruri hai:

| Method | Side | Kya karta hai | Kab use karte ho |
|---|---|---|---|
| `stream.CloseSend()` | **Client** (any streaming mode) | Sirf upstream close — server ke `Recv()` ko EOF deta hai. **Response wait nahi karta.** | Bidirectional streaming me, jab client ne sab bhej diya but server abhi bhi messages bhej raha hai aur tum receive karte rahoge. |
| `stream.CloseAndRecv()` | **Client (client-streaming only)** | `CloseSend()` + final response read in one call. | Client streaming me, jab tumhe aakhri single response chahiye. |
| `stream.SendAndClose(&Res)` | **Server (client-streaming only)** | Single response bhejta + handler ka return signal. | Client streaming server handler me, EOF detect hone ke baad. |

### Visual: kaun ye methods use kar sakta hai?

```
                    | CloseSend | CloseAndRecv | SendAndClose
--------------------|-----------|--------------|-------------
Unary client        | NO        | NO           | NO
Unary server        | NO        | NO           | NO
Server-stream client| NO*       | NO           | NO
Server-stream server| NO        | NO           | NO**
Client-stream client| YES       | YES          | NO
Client-stream server| NO        | NO           | YES
Bidirec. client     | YES       | NO           | NO
Bidirec. server     | NO        | NO           | NO
```

\* server-stream client me `CloseSend` available hai (interface me hai) but rarely use hota — request constructor ke andar already auto-close ho jaata hai.
\** server-stream server me `Send()` loop ke baad `return nil` se hi close hota.

### `CloseAndRecv` vs `CloseSend` + manual `Recv` — same?

Almost. Ye dono equivalent hain client streaming me:

```go
res, err := stream.CloseAndRecv()
```

Yaani:

```go
if err := stream.CloseSend(); err != nil {
    return err
}
var res pb.UserResponse
if err := stream.RecvMsg(&res); err != nil {
    return nil, err
}
```

`CloseAndRecv` bas convenience method hai jo dono kaam ek line me kar deta. Idiomatic version is `CloseAndRecv`.

---

## Server streaming → Client streaming — code-level diff

Side-by-side me dono ke server handlers (essential parts):

### Server streaming server handler

```go
func (s *Server) GreetManyTimes(in *pb.GreetRequest, stream grpc.ServerStreamingServer[pb.GreetResponse]) error {
    for i := 0; i < 10; i++ {
        stream.Send(&pb.GreetResponse{Result: "..."})    // <-- handler SENDS
    }
    return nil   // <-- gRPC auto-closes
}
```

### Client streaming server handler (current project)

```go
func (s *server) SendUser(stream grpc.ClientStreamingServer[pb.UsersRequest, pb.UserResponse]) error {
    var names []string
    for {
        req, err := stream.Recv()                       // <-- handler RECEIVES
        if err == io.EOF {
            return stream.SendAndClose(&pb.UserResponse{Result: ...})  // <-- single send + close
        }
        if err != nil { return err }
        names = append(names, req.GetName())
    }
}
```

| Feature | Server streaming handler | Client streaming handler |
|---|---|---|
| Request access | Parameter `in *Req` | `stream.Recv()` loop |
| Response send | Loop me `stream.Send(res)` × N | Sirf ek baar `stream.SendAndClose(res)` |
| EOF check | Nahi (server EOF generate karta) | Haan (EOF se loop exit) |
| Close mechanism | `return nil` se gRPC auto-close | `SendAndClose` se explicit |

### Server streaming client

```go
stream, _ := client.GreetManyTimes(ctx, &pb.GreetRequest{...})    // <-- request param me
for {
    res, err := stream.Recv()                                      // <-- client RECEIVES
    if err == io.EOF { break }
    log.Println(res.GetResult())
}
```

### Client streaming client (current project)

```go
stream, _ := client.SendUser(ctx)                                  // <-- no request param!
for _, name := range names {
    stream.Send(&pb.UsersRequest{Name: name})                      // <-- client SENDS loop
}
resp, _ := stream.CloseAndRecv()                                   // <-- single recv + close
log.Println(resp.GetResult())
```

| Feature | Server streaming client | Client streaming client |
|---|---|---|
| Request | Constructor parameter | Loop me `stream.Send(req)` × N |
| Response | `stream.Recv()` loop | Sirf ek baar `stream.CloseAndRecv()` |
| EOF check | Haan (client side) | Nahi (server side pe aata) |
| `io` import zaruri? | Haan (client side) | Nahi (server side me) |

---

## Mnemonic: "Stream keyword ki side me Send hota hai"

Proto file ko dekho:

```proto
rpc GreetManyTimes (Req) returns (stream Res);     // <-- stream RESPONSE side pe
                                  ^^^^^^^^^^^^
                                  server SENDS multiple Res

rpc SendUser (stream Req) returns (Res);           // <-- stream REQUEST side pe
              ^^^^^^^^^^^
              client SENDS multiple Req

rpc Bidirec (stream Req) returns (stream Res);     // <-- stream BOTH sides pe
             ^^^^^^^^^^^          ^^^^^^^^^^^^
             client SENDS         server SENDS
```

**Rule**: `stream` keyword jis side hai, vahi side **multiple Send** karega. Doosri side **single message** ke saath kaam karti hai (constructor parameter ya SendAndClose).

---

## Bidirectional ka jhilmilanaa preview

Ab dono mil ke karenge:

```proto
rpc Chat (stream ChatMessage) returns (stream ChatMessage);
```

Generated handler:

```go
func (s *server) Chat(stream grpc.BidiStreamingServer[pb.ChatMessage, pb.ChatMessage]) error {
    for {
        in, err := stream.Recv()
        if err == io.EOF { return nil }                    // <-- client side se EOF
        if err != nil { return err }

        // server bhi Send kar sakta hai jab chahe
        if err := stream.Send(&pb.ChatMessage{Text: "echo: " + in.GetText()}); err != nil {
            return err
        }
    }
}
```

Notice:

- `stream.Recv()` aur `stream.Send()` **dono yahaan available hain** (kyunki dono side me streaming hai).
- EOF still server side pe milta hai (jab client `CloseSend` kare).
- Server `return nil` se hi close hota hai (no `SendAndClose` — vo client streaming-only hai).

**Yaani bidirectional me dono streams independently chal sakte hain**. Pattern ka extension hai — naya kuch nahi.

---

## Common galtiyaan (direction wali)

| Galti | Mode | Lakshan | Fix |
|---|---|---|---|
| Client streaming me `stream.Send` ke baad `Send` continue karna jab `Send` ne error diya | Client-stream | Multiple failed sends | Error pe `break`, then `CloseAndRecv` |
| Client streaming me `CloseAndRecv` bhul jaana | Client-stream | Server hang ho jaata (EOF nahi milta) | Hamesha loop ke baad `CloseAndRecv` call karo |
| Server streaming server me `stream.Send` ke baad `SendAndClose` call karna | Server-stream | Compile error (method exist nahi karta) | Sirf `return nil` se close karo |
| Client streaming server me `Send` × multiple call karna | Client-stream | Runtime error: "stream already closed" | Sirf `SendAndClose` ek baar |
| Server streaming me `io` import client me bhul jaana | Server-stream | Compile error: `io` undefined | Import karo |
| Client streaming me `io` import client side me karna (galat side) | Client-stream | Unused import error | Server side me chahiye, client me nahi |
| `stream.Recv()` loop me `io.EOF` ko `errors.Is` se nahi check karna | Any | Mostly fine, but wrapped errors me fail | `errors.Is(err, io.EOF)` more robust |

---

## TL;DR — Direction ka one-pager

```
SERVER STREAMING:
  client → 1 req → server
  client ← N res ← server (Send loop)
  EOF lands on CLIENT (after server returns)

CLIENT STREAMING (this project):
  client → N req → server (Send loop)
  EOF lands on SERVER (after client CloseSend/CloseAndRecv)
  client ← 1 res ← server (SendAndClose)

BIDIRECTIONAL:
  client → N req → server (Send loop)
  client ← N res ← server (Send loop)
  EOF lands on the side whose peer called CloseSend
```

**Ek baar ye picture mind me set ho gayi, fir kabhi confuse nahi hoge ki "io.EOF kis side pe aayega" ya "SendAndClose kab use karu". Yahi knowledge bidirectional me 1:1 apply hoti hai.**
