# `proto/greet.proto` — Service ki blueprint

## Pura file

```proto
syntax ="proto3";
package greet;
option go_package = "example.com/greet/proto" ;

message GreetRequest{
    string firstName =1;
}

message GreetResponse {
    string result =1;
}

service GreetService{
 rpc Greet (GreetRequest) returns (GreetResponse);
}
```

Ye **Protocol Buffers** language me likha gaya hai (Google ka IDL — Interface Definition Language). Ye file pure project ka **source of truth** hai. Yahan se `protoc` Go code generate karta hai. Iss ek file me 4 cheezein declared hain — version, package, output mapping, aur actual contract.

---

## Line-by-line breakdown

### `syntax = "proto3";`

Ye proto language ka **version** declare karta hai. Do options hain:

- `proto2` — purana, fields me `required`/`optional` keywords hote hain.
- `proto3` — modern, simpler. Saare fields by default optional hain. Aaj kal sab use karte hain.

> Pehli line me likhna **mandatory** hai. Bhul gaye to `protoc` warning dega aur proto2 maan lega.

### `package greet;`

Ye **proto-level** package hai (Go ka nahi!). Iska kaam:

1. Naam-collision rokta hai. Agar do alag protos me dono `User` message hai, to ek `greet.User` aur doosra `auth.User` ban jaata hai.
2. Wire-level identifiers banata hai. Tumhara service is package ka use karke address hota hai — yaad karo grpc file me ye line:

```22:22:gRPC project/greet/proto/greet_grpc.pb.go
	GreetService_Greet_FullMethodName = "/greet.GreetService/Greet"
```

   Vo `greet.` prefix sidha is package declaration se aaya hai.

> **Important**: ye Go ke `package` se alag hai. Vo neeche `option go_package` me set hota hai.

### `option go_package = "example.com/greet/proto";`

Ye batata hai ki generated `.pb.go` file kis **Go import path** ke under banegi.

Formula:

```
go.mod ka module name + folder path = go_package
```

Tumhare project me:

- `go.mod` me module: `example.com/greet`
- proto file folder: `proto/`
- → `option go_package = "example.com/greet/proto"`

Agar ye match nahi karta to `pb "example.com/greet/proto"` import line **kabhi** kaam nahi karegi.

### `message GreetRequest { string firstName = 1; }`

Ye ek **structured data type** hai — Go me struct ban jaayega.

- `message` keyword = "ye ek data shape hai".
- `GreetRequest` = type ka naam (PascalCase convention).
- `string firstName = 1;` = field declaration.

Field declaration ka format:

```
[type]   [name]         = [field-number] ;
string   firstName      = 1            ;
```

**Field number (`= 1`) sabse important cheez hai** — ye wire pe protobuf encoding me use hota hai. Naam (`firstName`) sirf source code ke liye hai; wire pe sirf number jaata hai.

> ⚠️ **Field number ek baar fix ho gaya, kabhi mat badlo.** Tum naam `firstName` se `firstname` kar sakte ho safely (compatible), lekin `= 1` ko `= 2` kar diya to saare purane clients break ho jaayenge.

#### Field number ranges (yaad rakho)

- `1`–`15` → 1 byte on wire (frequent fields ke liye reserve)
- `16`–`2047` → 2 bytes
- `19000`–`19999` → reserved by protobuf, mat use karo

#### Naming convention

Proto guide kehta hai `snake_case` use karo (`first_name`), lekin tumne `firstName` likha — kaam karega, bas Google style ke against hai. `protoc` automatically PascalCase me convert kar deta hai Go ke liye:

| Proto field | Go field |
|---|---|
| `first_name` | `FirstName` |
| `firstName` | `FirstName` |
| `FirstName` | `FirstName` |

### `message GreetResponse { string result = 1; }`

Same logic — server ka response type. Field `result` (string) hai.

> **Convention tip**: Har RPC ka **apna alag** request aur response message banao, even agar fields same hain. Future me agar `Greet` me kuch field add karna ho aur `OtherRPC` me nahi, to shared message problem deti hai.

### `service GreetService { ... }`

Ye actual **gRPC service definition** hai. `service` block grpc-specific hai (sirf message hota to bas data hota — service = data + behavior).

```proto
service GreetService {
  rpc Greet (GreetRequest) returns (GreetResponse);
}
```

Anatomy:

```
rpc      Greet         (GreetRequest)  returns  (GreetResponse) ;
^^^^     ^^^^^^^^^^    ^^^^^^^^^^^^^^^   ^^^^   ^^^^^^^^^^^^^^^
keyword  method-name   request-type   keyword   response-type
```

Rules:

- `rpc` keyword **lowercase** hota hai.
- Method naam **PascalCase** (kyunki Go me method naam wahi banega).
- Request aur response **dono message types hone chahiye**, primitive nahi (yaani `string`, `int` direct nahi).
- Last me `;` mandatory.

#### 4 RPC modes (advanced — abhi sirf jaan lo)

1. **Unary** — `rpc Greet (Req) returns (Res);` ← tumhara wala
2. **Server streaming** — `rpc X (Req) returns (stream Res);`
3. **Client streaming** — `rpc X (stream Req) returns (Res);`
4. **Bidirectional** — `rpc X (stream Req) returns (stream Res);`

Sirf `stream` keyword add karne se mode change ho jaata hai. Aage course me ye sab aayenge.

---

## Iss file se kya generate hota hai?

Jab `make generate` chalao to:

- `message GreetRequest` → `greet.pb.go` me `type GreetRequest struct { FirstName string ... }`
- `message GreetResponse` → `greet.pb.go` me `type GreetResponse struct { Result string ... }`
- `service GreetService` → `greet_grpc.pb.go` me 2 cheezein:
  - `GreetServiceClient` interface (caller ke liye)
  - `GreetServiceServer` interface (implementer ke liye)

Yaani is **17-line file** se hi tumhare 100+ lines wale Go files paida hote hain. Tumhara kaam bas proto edit karna aur regenerate karna hai.

---

## TL;DR table

| Line | Kya hai | Kyu zaruri |
|---|---|---|
| `syntax = "proto3"` | proto language version | Compiler ko batata hai kaunsa grammar use karna |
| `package greet` | proto-level namespace | Wire format me service ka prefix |
| `option go_package = "..."` | Generated Go ka import path | Bina iske Go side me import nahi banega |
| `message X { ... }` | Data structure | Go struct ban jaata hai |
| `string field = N` | Field declaration | `N` wire number, kabhi mat badlo |
| `service X { rpc ... }` | gRPC contract | Client + server interfaces ban jaate hain |
