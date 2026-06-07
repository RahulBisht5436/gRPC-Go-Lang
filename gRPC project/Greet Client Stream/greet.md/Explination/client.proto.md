# `proto/client.proto` — Service ki blueprint (Client Streaming)

## Pura file

```proto
syntax ="proto3";
//convention package name should be all small case
package clientStream ;

// package name for the Go file need to be all in smalll case
option go_package="example.com/clientStream/proto";


message UsersRequest{
    string name =1;
}

message UserResponse{
    string Result =1;
}


service UserService{
    rpc SendUser(stream UsersRequest) returns (UserResponse);
}
```

Ye **Protocol Buffers** language me likhi gayi hai (Google ka IDL — Interface Definition Language). Ye file pure project ka **source of truth** hai. Yahan se `protoc` Go code generate karta hai. Iss ek file me 4 cheezein declared hain — version, package, output mapping, aur actual contract.

> **Iss project me ek important badlaav** hai pichle (server streaming) version ke mukable: ab `stream` keyword **request side** pe laga hai, response side pe nahi. Yaani ye **Client-Side Streaming RPC** hai — client multiple `UsersRequest` messages stream me bhejta hai, server ek **single `UserResponse`** wapas deta hai jab client `CloseSend` signal de.

---

## Line-by-line breakdown

### `syntax = "proto3";`

Ye proto language ka **version** declare karta hai. Do options hain:

- `proto2` — purana, fields me `required`/`optional` keywords hote hain.
- `proto3` — modern, simpler. Saare fields by default optional hain. Aaj kal sab use karte hain.

> Pehli line me likhna **mandatory** hai. Bhul gaye to `protoc` warning dega aur proto2 maan lega.

### `package clientStream;`

Ye **proto-level** package hai (Go ka nahi!). Iska kaam:

1. Naam-collision rokta hai. Agar do alag protos me dono `UsersRequest` message hai, to ek `clientStream.UsersRequest` aur doosra `auth.UsersRequest` ban jaata hai.
2. Wire-level identifiers banata hai. Tumhara service is package ka use karke address hota hai — yaad karo grpc file me ye line:

```24:24:proto/client_grpc.pb.go
	UserService_SendUser_FullMethodName = "/clientStream.UserService/SendUser"
```

   Vo `clientStream.` prefix sidha is package declaration se aaya hai.

> **Note**: Tumne `clientStream` (camelCase) likha hai. Proto convention kehta hai package naam **lowercase + underscores** rakho (`client_stream`), lekin technically camelCase chalta hai. Sirf style issue hai, working nahi tutgi.

> **Important**: ye Go ke `package` se alag hai. Vo neeche `option go_package` me set hota hai.

### `option go_package = "example.com/clientStream/proto";`

Ye batata hai ki generated `.pb.go` file kis **Go import path** ke under banegi.

Formula:

```
go.mod ka module name + folder path = go_package
```

Tumhare project me:

- `go.mod` me module: `example.com/clientStream`
- proto file folder: `proto/`
- → `option go_package = "example.com/clientStream/proto"`

Agar ye match nahi karta to `pb "example.com/clientStream/proto"` import line **kabhi** kaam nahi karegi.

### `message UsersRequest { string name = 1; }`

Ye ek **structured data type** hai — Go me struct ban jaayega.

- `message` keyword = "ye ek data shape hai".
- `UsersRequest` = type ka naam (PascalCase convention). **Plural "Users"** isliye ki client multiple bhej raha hai — but actually har individual message me sirf ek user ka naam hai. Pluralization confusing hai; better naam `UserRequest` (singular) hota.
- `string name = 1;` = field declaration.

Field declaration ka format:

```
[type]   [name]   = [field-number] ;
string   name     = 1            ;
```

**Field number (`= 1`) sabse important cheez hai** — ye wire pe protobuf encoding me use hota hai. Naam (`name`) sirf source code ke liye hai; wire pe sirf number jaata hai.

> ⚠️ **Field number ek baar fix ho gaya, kabhi mat badlo.** Tum naam `name` se `userName` kar sakte ho safely (compatible), lekin `= 1` ko `= 2` kar diya to saare purane clients break ho jaayenge.

#### Field number ranges (yaad rakho)

- `1`–`15` → 1 byte on wire (frequent fields ke liye reserve)
- `16`–`2047` → 2 bytes
- `19000`–`19999` → reserved by protobuf, mat use karo

### `message UserResponse { string Result = 1; }`

Same logic — server ka response type. Field `Result` (capital R, string) hai.

> **Streaming context me note**: Client streaming me **client ki taraf se** multiple `UsersRequest` instances aate hain (loop me `Send`), lekin server **sirf ek baar** `UserResponse` bheji jaati hai (`SendAndClose` se). To `UserResponse` ek aggregate / summary type ban jaata hai. Tumhare server code me `result := "hello, " + strings.Join(names, ", ")` — yahi aggregate hai.

> **Naming convention warning**: Tumne `Result` (PascalCase) likha hai field me. Proto guide kehta hai `snake_case` use karo (`result`). Kaam karega, bas Google style ke against hai. Generated Go code me to `Result` hi rahega (already capital).

### `service UserService { ... }`

Ye actual **gRPC service definition** hai. `service` block grpc-specific hai (sirf message hota to bas data hota — service = data + behavior).

```proto
service UserService {
  rpc SendUser(stream UsersRequest) returns (UserResponse);
}
```

Anatomy:

```
rpc      SendUser   (stream UsersRequest)  returns  (UserResponse) ;
^^^^     ^^^^^^^^    ^^^^^^^^^^^^^^^^^^^^   ^^^^   ^^^^^^^^^^^^^^
keyword  method      STREAM of request       kw     single response
```

Rules:

- `rpc` keyword **lowercase** hota hai.
- Method naam **PascalCase** (kyunki Go me method naam wahi banega).
- Request aur response **dono message types hone chahiye**, primitive nahi.
- Last me `;` mandatory.

#### Yahan `stream` keyword ki **position** hi sab kuch decide karti hai

`SendUser(stream UsersRequest) returns (UserResponse)` me jo `stream` likha hai — **yahi pure RPC ka mode badal deta hai**. Ek hi keyword se:

- Client side ka generated method ab `(ctx, *Req) (*Res, error)` nahi, balki **`(ctx) (grpc.ClientStreamingClient[UsersRequest, UserResponse], error)`** return karta hai. **Notice — request parameter call me hai hi nahi!** Wo `stream.Send(...)` se baad me jaayega.
- Server side ka handler ab `(*Req) (*Res, error)` nahi, balki **`(stream grpc.ClientStreamingServer[UsersRequest, UserResponse]) error`** lega. Request parameter bhi nahi hai — sab kuch `stream.Recv()` se aata hai.
- Wire pe ek hi HTTP/2 stream pe **multiple DATA frames upstream** (client → server) chalte hain, fir trailers ke pehle ek aakhri downstream message (server → client).

Ye sab tumhara hath se nahi, **`protoc-gen-go-grpc`** automatic karta hai jab `stream` keyword dekhta hai uss specific position pe.

#### 4 RPC modes (compare karo)

1. **Unary** — `rpc X (Req) returns (Res);`
2. **Server streaming** — `rpc X (Req) returns (stream Res);` ← pichla project
3. **Client streaming** — `rpc X (stream Req) returns (Res);` ← **iss project me yahi hai**
4. **Bidirectional** — `rpc X (stream Req) returns (stream Res);`

**Sirf `stream` keyword ki position se mode change ho jaata hai.** Aage bidirectional bhi aayega — vo dono jagah `stream` likhne se ban jaayega.

#### Client streaming kab use karte ho?

Real-world examples:

- **File upload (chunked)** — bade file ko chote chunks me todke ek-ek karke bhejna; server final hash/URL return kare.
- **Bulk insert API** — agent thousands of rows stream kare, server `{inserted_count: 9821}` jaisa summary de.
- **IoT sensor batch** — device har second ek reading bheje, server aggregate stats (avg, min, max) return kare.
- **Log shipping** — agent log lines stream kare, server `{accepted: N}` return kare.

Tumhare current handler me artificial example hai (4 names → concatenated greeting), but pattern wahi hota hai.

---

## Iss file se kya generate hota hai?

Jab `protoc` chalao to:

- `message UsersRequest` → `client.pb.go` me `type UsersRequest struct { Name string ... }`
- `message UserResponse` → `client.pb.go` me `type UserResponse struct { Result string ... }`
- `service UserService` → `client_grpc.pb.go` me 2 cheezein:
  - `UserServiceClient` interface jisme `SendUser(ctx, opts...) (grpc.ClientStreamingClient[UsersRequest, UserResponse], error)` — **note: koi `in *UsersRequest` parameter nahi**.
  - `UserServiceServer` interface jisme `SendUser(grpc.ClientStreamingServer[UsersRequest, UserResponse]) error` — **note: koi `*UsersRequest` parameter nahi, sirf stream**.

Notice — **dono signatures unary aur server streaming se alag hain** — request as parameter chala gaya, stream object aa gaya.

Yaani is **22-line file** se hi tumhare 100+ lines wale Go files paida hote hain. Tumhara kaam bas proto edit karna aur regenerate karna hai.

---

## Server streaming vs Client streaming — proto level diff

| Cheez | Server streaming (pichli) | Client streaming (current) |
|---|---|---|
| RPC line | `rpc GreetManyTimes (Req) returns (stream Res);` | `rpc SendUser (stream Req) returns (Res);` |
| `stream` keyword kahan | Response side | **Request side** |
| Client side generated method | `(ctx, in, opts) (StreamReceiver, error)` | `(ctx, opts) (StreamSender, error)` |
| Server side handler | `(in, stream) error` | `(stream) error` |
| Service descriptor flag | `ServerStreams: true` | `ClientStreams: true` |

Tumhare generated file me dekho:

```108:113:proto/client_grpc.pb.go
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SendUser",
			Handler:       _UserService_SendUser_Handler,
			ClientStreams: true,
		},
	},
```

`ClientStreams: true` hi vo flag hai jo gRPC runtime ko batata hai "ye method client-side streaming hai".

---

## TL;DR table

| Line | Kya hai | Kyu zaruri |
|---|---|---|
| `syntax = "proto3"` | proto language version | Compiler ko grammar batata |
| `package clientStream` | proto-level namespace | Wire format me service ka prefix |
| `option go_package = "..."` | Generated Go ka import path | Bina iske Go side me import nahi banega |
| `message X { ... }` | Data structure | Go struct ban jaata hai |
| `string field = N` | Field declaration | `N` wire number, kabhi mat badlo |
| `service X { rpc ... }` | gRPC contract | Client + server interfaces ban jaate |
| `stream` keyword (request side) | RPC mode marker | Unary se **client streaming** me badal deta hai (ek line se) |
