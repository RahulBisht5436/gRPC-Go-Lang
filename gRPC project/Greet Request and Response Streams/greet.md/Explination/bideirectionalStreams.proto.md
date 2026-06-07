# `proto/bideirectionalStreams.proto` — Service ki blueprint (Bidirectional Streaming)

## Pura file

```proto
syntax = "proto3";
package bidirectional;
option go_package="example.com/bidirectional/proto";

message GreetRequest {
    string first_name =1;
}

message GreetResponse{
    string result =1;
}

service GreetService {
    rpc GreetEveryone(stream GreetRequest)  returns (stream GreetResponse);
}
```

Ye **Protocol Buffers** language me likhi gayi hai (Google ka IDL — Interface Definition Language). Ye file pure project ka **source of truth** hai. Yahan se `protoc` Go code generate karta hai.

> **Iss project me ek important badlaav** hai pichle teen versions ke mukable: ab `stream` keyword **dono sides** pe laga hai. Yaani ye **Bidirectional Streaming RPC** hai — client multiple `GreetRequest` messages stream me bhej sakta hai, aur server bhi independently multiple `GreetResponse` messages stream me bhej sakta hai. **Order independent** hai — server koi response client ki request se pehle bhi bhej sakta hai theoretically (real-world me usually Recv → Send pattern follow karte hain).

> **File naam typo**: file ka naam `bideirectionalStreams.proto` hai (notice — `bideirectional`, not `bidirectional`). Typo file naam me hai, but proto ke andar `package bidirectional` (correct spelling) hai. Iska impact koi nahi — file naam Go side me dikhne wala nahi (sirf `init()` me debug ke liye hai). Lekin **rename karna better hai** — `bidirectionalStreams.proto`. Agar rename karoge, to generated `.pb.go` files bhi regenerate karni hongi.

---

## Line-by-line breakdown

### `syntax = "proto3";`

Ye proto language ka **version** declare karta hai. Aaj kal sab `proto3` use karte hain. Pehli line me likhna mandatory hai.

### `package bidirectional;`

Ye **proto-level** package hai (Go ka nahi!). Iska kaam:

1. Naam-collision rokta hai.
2. Wire-level identifiers banata hai. Tumhara service is package ka use karke address hota hai:

```22:22:proto/bideirectionalStreams_grpc.pb.go
	GreetService_GreetEveryone_FullMethodName = "/bidirectional.GreetService/GreetEveryone"
```

Vo `bidirectional.` prefix sidha is package declaration se aaya hai.

> **Important**: ye Go ke `package` se alag hai. Vo neeche `option go_package` me set hota hai.

### `option go_package = "example.com/bidirectional/proto";`

Ye batata hai ki generated `.pb.go` file kis **Go import path** ke under banegi.

Formula:

```
go.mod ka module name + folder path = go_package
```

Tumhare project me:

- `go.mod` me module: `example.com/bidirectional`
- proto file folder: `proto/`
- → `option go_package = "example.com/bidirectional/proto"`

> ⚠️ Agar mismatch ho, to import path nahi banega.

### `message GreetRequest { string first_name = 1; }`

Ye ek **structured data type** hai — Go me struct ban jaayega.

- `message` keyword = "ye ek data shape hai".
- `GreetRequest` = type ka naam (PascalCase convention).
- `string first_name = 1;` = field declaration.

> **Naming convention** — yahaan tumne `snake_case` (`first_name`) follow kiya hai, jo Google ka **recommended** style hai (pichle projects me tumne camelCase use kiya tha — `firstName`). Better practice yahin pe achi laagu hui. `protoc` automatic `FirstName` me convert kar dega Go ke liye.

| Proto field | Go field |
|---|---|
| `first_name` | `FirstName` |
| `firstName` | `FirstName` (still PascalCase, but ugly JSON tag) |

Snake_case ki badi advantage — JSON tag bhi `"first_name"` clean dikhta hai REST gateways ke liye.

### `message GreetResponse { string result = 1; }`

Same logic — server ka response type. Field `result` (string).

> **Bidirectional streaming context me note**: Ab dono `GreetRequest` aur `GreetResponse` **dono multiple instances** wire pe travel kar sakte hain. Client multiple `GreetRequest` bhejega `Send` loop me, server multiple `GreetResponse` bhejega `Send` calls me. Message ki shape stream count se independent hai.

### `service GreetService { ... }`

Ye actual **gRPC service definition** hai.

```proto
service GreetService {
  rpc GreetEveryone(stream GreetRequest) returns (stream GreetResponse);
}
```

Anatomy:

```
rpc      GreetEveryone   (stream GreetRequest)   returns  (stream GreetResponse) ;
^^^^     ^^^^^^^^^^^^^    ^^^^^^^^^^^^^^^^^^^^^   ^^^^   ^^^^^^^^^^^^^^^^^^^^^^^
keyword  method-name      STREAM of request       kw     STREAM of response
```

**Notice — `stream` keyword DONO sides pe**. Yahi pure RPC ko bidirectional banata hai.

#### Bidirectional ka kya matlab — wire level

```
Time →

Client:  Send(req1) ----→
                Send(req2) ----→
                                                        ←---- Send(res1) :Server
                Send(req3) ----→
                                                        ←---- Send(res2)
                                                        ←---- Send(res3)
         CloseSend ────────→  (server gets io.EOF)
                                                        return nil → trailers
         <Recv returns io.EOF eventually>
```

- Client aur server **independently** Send aur Recv kar sakte hain.
- **Order is not enforced** — server response client ki request ke pehle bhi bhej sakta hai (initial subscription style), ya har request ke saath ek response (echo style), ya batches me responses (aggregate every N requests style).
- Tumhare current code me **echo pattern** hai — har `Recv` ke baad immediate `Send`.

#### 4 RPC modes (final compare)

1. **Unary** — `rpc X (Req) returns (Res);`
2. **Server streaming** — `rpc X (Req) returns (stream Res);`
3. **Client streaming** — `rpc X (stream Req) returns (Res);`
4. **Bidirectional** — `rpc X (stream Req) returns (stream Res);` ← **iss project me yahi hai**

Char modes complete ho gaye! Course me yahin tak gRPC ki core capabilities cover ho jaati hain.

#### Bidirectional streaming kab use karte ho?

Real-world examples:

- **Real-time chat** — har user messages bhejta + receive karta, parallel.
- **Multiplayer game state** — position updates dono directions.
- **Live transcription / dictation** — audio chunks upstream, partial transcripts downstream as they get processed.
- **WebSocket-style subscriptions** — client subscribe karta, server events push karta jab aate hain.
- **Robotics control loops** — sensor readings upstream, motor commands downstream, both at high frequency.
- **gRPC-based proxies/tunnels** — TCP-over-gRPC, traffic both directions.

Tumhare current handler me artificial example hai — echo (`Hello, <name>`), but pattern wahi hai.

---

## Iss file se kya generate hota hai?

Jab `protoc` chalao to:

- `message GreetRequest` → `bideirectionalStreams.pb.go` me `type GreetRequest struct { FirstName string ... }`
- `message GreetResponse` → `bideirectionalStreams.pb.go` me `type GreetResponse struct { Result string ... }`
- `service GreetService` → `bideirectionalStreams_grpc.pb.go` me 2 cheezein:
  - `GreetServiceClient` interface jisme `GreetEveryone(ctx, opts...) (grpc.BidiStreamingClient[GreetRequest, GreetResponse], error)` — **note: koi `req` parameter nahi**.
  - `GreetServiceServer` interface jisme `GreetEveryone(grpc.BidiStreamingServer[GreetRequest, GreetResponse]) error` — **note: koi `*GreetRequest` parameter nahi, sirf stream**.

Service descriptor me dono flags set hote hain:

```106:113:proto/bideirectionalStreams_grpc.pb.go
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GreetEveryone",
			Handler:       _GreetService_GreetEveryone_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
```

`ServerStreams: true` **aur** `ClientStreams: true` dono — yahi bidirectional ka encoding hai.

Yaani is **15-line file** se hi tumhare 100+ lines wale Go files paida hote hain.

---

## Char modes ka proto-level compare

| Cheez | Unary | Server stream | Client stream | Bidirectional |
|---|---|---|---|---|
| RPC line | `rpc X (Req) returns (Res);` | `rpc X (Req) returns (stream Res);` | `rpc X (stream Req) returns (Res);` | `rpc X (stream Req) returns (stream Res);` |
| `stream` keyword | nowhere | response side | request side | **both sides** |
| Client side method | `(ctx, in, opts) (*Res, err)` | `(ctx, in, opts) (StreamReceiver, err)` | `(ctx, opts) (StreamSender, err)` | `(ctx, opts) (BidiStream, err)` |
| Server side handler | `(ctx, *Req) (*Res, err)` | `(*Req, stream) err` | `(stream) err` | `(stream) err` |
| Service descriptor flag | (neither set) | `ServerStreams: true` | `ClientStreams: true` | **both set** |

---

## TL;DR table

| Line | Kya hai | Kyu zaruri |
|---|---|---|
| `syntax = "proto3"` | proto language version | Compiler ko grammar batata |
| `package bidirectional` | proto-level namespace | Wire format me service ka prefix |
| `option go_package = "..."` | Generated Go ka import path | Bina iske Go side me import nahi banega |
| `message X { ... }` | Data structure | Go struct ban jaata hai |
| `string field = N` | Field declaration | `N` wire number, kabhi mat badlo |
| `service X { rpc ... }` | gRPC contract | Client + server interfaces ban jaate |
| `stream` keyword (both sides) | RPC mode marker | Bidirectional mode set karta hai |
