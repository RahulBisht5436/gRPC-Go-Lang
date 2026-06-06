# `server/main.go` — Server ka bootstrap

Iss file ka **ek hi kaam** hai: gRPC server ko khada karna aur connections accept karne lagana. Actual business logic (yaani `Greet` ka kya kaam hai) is file me **nahi** hai — vo [`server/greet.go`](./server-greet.go.md) me hai. Ye separation jaan-bujh ke ki gayi hai: bootstrap alag, handlers alag.

## Pura file (clean version)

```go
package main

import (
    "log"
    "net"

    pb "example.com/greet/proto"
    "google.golang.org/grpc"
)

var addr = "0.0.0.0:50051"

type Server struct {
    pb.UnimplementedGreetServiceServer
}

func main() {
    lis, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatalf("Failed to listen on %s: %v", addr, err)
    }
    log.Printf("Listening on %s", addr)

    s := grpc.NewServer()
    pb.RegisterGreetServiceServer(s, &Server{})

    if err := s.Serve(lis); err != nil {
        log.Fatalf("Failed to serve: %v", err)
    }
}
```

---

## Ye file gRPC server ke 4 mandatory steps follow karti hai

1. **TCP listener kholo** — port pe baith jao.
2. **`*grpc.Server` banao** — gRPC ka brain.
3. **Service register karo** — runtime ko batao "ye RPC mere paas aaye".
4. **`Serve()` call karo** — connections accept karna shuru.

In char me se koi bhi miss ho gaya, server kaam nahi karega.

---

## Line-by-line breakdown

### `package main`

Go me jo package `main` hota hai aur usme `func main()` hota hai, vahi **executable binary** banta hai. Iska matlab — `go build ./server` chalega to ek `.exe` banega, jabki `proto/` package compile hokar bas library banti hai.

### Imports

```go
import (
    "log"   // logging
    "net"   // net.Listen
    pb "example.com/greet/proto"
    "google.golang.org/grpc"
)
```

- `log` — standard library, errors print karne ke liye.
- `net` — TCP socket kholne ke liye (`net.Listen`).
- `pb "example.com/greet/proto"` — **tumhari** generated proto package. Alias `pb` rakha taaki `pb.GreetRequest` likhne me chhota lage.
- `google.golang.org/grpc` — gRPC runtime library. Ye `grpc.NewServer()` aur `s.Serve()` deti hai.

### `var addr = "0.0.0.0:50051"`

Server kaha listen karega, vo address.

- `0.0.0.0` — "saare network interfaces". Yaani localhost se bhi access hoga, aur LAN ke doosre machines se bhi (firewall agar allow kare to). Compare karo `127.0.0.1` se — vo sirf same machine se access deta.
- `50051` — gRPC examples ka **convention** port. Koi magic nahi, kuch bhi rakh sakte ho (jaise 8080, 9000). Bas client aur server dono pe match hona chahiye.

### `type Server struct { pb.UnimplementedGreetServiceServer }`

Ye chhoti si line gRPC ka sabse important "trick" hai. Do reasons:

#### Reason 1 — Interface satisfaction

`greet_grpc.pb.go` me ek interface hai:

```go
type GreetServiceServer interface {
    Greet(context.Context, *GreetRequest) (*GreetResponse, error)
    mustEmbedUnimplementedGreetServiceServer()  // <-- ye lowercase hai, unexported
}
```

Notice — `mustEmbed...` method **unexported** hai. Yaani tum apne package se isse implement hi nahi kar sakte directly. Sirf `UnimplementedGreetServiceServer` ko **embed** karke ye method automatic mil jaata hai.

Yeh **forced embedding pattern** kehlata hai — gRPC team ne specifically isliye banaya taaki tum bhul se `UnimplementedGreetServiceServer` ko embed kiye bina server na bana lo.

#### Reason 2 — Forward compatibility

Aaj proto me sirf `Greet` RPC hai. Kal agar proto me `GreetManyTimes` add ho gaya, to:

- **Bina embedding** ke: tumhara server compile fail kar dega kyunki interface me ab 2 methods hain aur tumne sirf 1 implement kiya.
- **With embedding**: `UnimplementedGreetServiceServer` ke andar default `GreetManyTimes` already hai jo `codes.Unimplemented` return karta hai. Tumhara server compile chalu rahega; bas us nayi RPC pe call karne pe error milega — jo logical hai.

> Iska summary: **embedding karna pure project me sabse zaruri 1 line hai**.

### `func main()` — Step 1: Listener

```go
lis, err := net.Listen("tcp", addr)
```

`net.Listen` ek **`net.Listener`** return karta hai. Ye object kuch nahi karta, sirf port pe baitha rehta hai aur incoming TCP connections accept karne ke liye ready hota hai. Naam `lis` rakha — convention. (Tumne pehle `conn` rakha tha jo galat tha kyunki connection nahi listener hai.)

```go
if err != nil {
    log.Fatalf("Failed to listen on %s: %v", addr, err)
}
```

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
pb.RegisterGreetServiceServer(s, &Server{})
```

Ye **wiring** step hai. Iska matlab:

> "Hey gRPC runtime `s`, agar koi `/greet.GreetService/Greet` ko call kare, to is `&Server{}` instance ke `Greet` method ko trigger karna."

`RegisterGreetServiceServer` function `protoc-gen-go-grpc` ne automatically generate kiya tha tumhare proto se. Iska body:

```83:87:gRPC project/greet/proto/greet_grpc.pb.go
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&GreetService_ServiceDesc, srv)
```

Internally vo `GreetService_ServiceDesc` use karta hai jo method-name → handler ka mapping hai.

> Agar tum **ye line bhul gaye**, to server start to ho jaayega lekin har RPC pe client ko `unknown service: greet.GreetService` error milega.

### Step 4: Serve karo

```go
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

---

## Common galtiyaan (jo tumne already karli aur fix ki!)

| Bug | Lakshan | Fix |
|---|---|---|
| Embed `pb.GreetRequest` instead of `Unimplemented...` | Compile error: interface satisfy nahi |  Embed `pb.UnimplementedGreetServiceServer` |
| `s.Server(conn)` likhna | Compile error: `Server` undefined method | `s.Serve(lis)` |
| Variable naam `conn` rakhna | Confusion, koi crash nahi | Rakho `lis` (it's a listener) |
| `Greet` method ko **dono** `main.go` aur `greet.go` me likhna | Compile error: `method already declared` | Ek hi jagah rakho (we kept it in greet.go) |
| `RegisterGreetServiceServer` skip kar dena | Server chalu, lekin RPC pe `Unimplemented` | Step 3 mat bhulo |

---

## Mental model

```
   port 50051
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
|  Server descriptor   |
|  + &Server{} pointer |
+----------------------+
       |
       v
+--------------+
|   s.Serve()  | <- Step 4 (block forever)
+--------------+
```

In char box ke saath complete server baith jaata hai. Bas implementation file (`server/greet.go`) chahiye actual logic ke liye.
