# `go.mod` — Module declaration aur dependencies

## Pura file

```go
module example.com/clientStream

go 1.26.2

require (
	github.com/golang/protobuf v1.5.4
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
)
```

## `go.mod` ka role

Ye Go module system ka **manifest file** hai. Iss file ki 3 main jimmedariyaan:

1. Module ka **naam** declare karna (import paths ka root).
2. Required Go **version** specify karna.
3. **Dependencies** track karna (direct + indirect).

Ye file `go mod init` se ban hoti hai aur `go get` / `go mod tidy` se update hoti hai. Manual edit kar sakte ho but normally tools ko karne do.

---

## Line-by-line

### `module example.com/clientStream`

Ye **module ka naam** hai. Pure project ka import root.

#### Important rule

Yeh naam baaki sab import paths ka **prefix** hota hai. Tumhare project me:

| Folder | Import path |
|---|---|
| `proto/` | `example.com/clientStream/proto` |
| `server/` | `example.com/clientStream/server` |
| `client/` | `example.com/clientStream/client` |

Yaad karo `server/main.go` me:

```go
import pb "example.com/clientStream/proto"
//        ^^^^^^^^^^^^^^^^^^^^^^^ ye module name
//                                ^^^^^^^ + folder path
```

Aur `client.proto` me:

```proto
option go_package = "example.com/clientStream/proto";
//                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
//                   match karna chahiye go.mod ke saath
```

> ⚠️ Agar `go.mod` ka module aur `go_package` mismatch ho, to import path nahi banega. Ye sabse common bug hai beginners ka.

#### `clientStream` (camelCase) — kyu?

Tumne `clientStream` likha hai. Go module names me typically dash use hota hai (`client-stream`) ya lowercase compact (`clientstream`). camelCase kaam karta hai but unusual hai.

Real projects me typically:

- `github.com/yourusername/projectname` (open source)
- `gitlab.example.com/team/repo` (private)
- `mycompany.com/team/service` (proprietary)

`example.com` learning/example me chalta hai, but agar published library banani ho to apna real domain rakhna padega.

### `go 1.26.2`

Minimum Go version requirement. Iska matlab:

- Tumhara compiler is se naya hona chahiye.
- Compiler is version ke language features assume karta hai.

Go versioning policy: minor releases (1.x) backward-compatible hote hain. Yaani Go 1.26 ka code Go 1.27 me bhi chalta hai.

### `require` blocks

Do separate `require` blocks dikh rahe hain:

```go
require (
    github.com/golang/protobuf v1.5.4
    google.golang.org/grpc v1.81.1
    google.golang.org/protobuf v1.36.11
)

require (
    golang.org/x/net v0.51.0 // indirect
    ...
)
```

#### Direct dependencies (pehla block)

Yeh wo packages hain jo tumne **khud import** kiye hain code me:

| Package | Kahan use ho raha |
|---|---|
| `github.com/golang/protobuf` | Legacy protobuf API. **Actually iss project me directly use nahi ho raha** — tumne pehle galti se import kar liya tha. `go mod tidy` chalao to ye remove ho jaayega aur indirect ban jaayega (kyunki kuch transitive dep ko chahiye hota hai). |
| `google.golang.org/grpc` | `import "google.golang.org/grpc"` server/client me |
| `google.golang.org/protobuf` | Generated code (`client.pb.go`) me indirectly required |

> **`golang/protobuf` vs `google.golang.org/protobuf`** — confusing similar names hain. Pehla **purana** (v1) API hai, doosra **naya** (v2). Modern code me sirf `google.golang.org/protobuf` chahiye. Agar tum `go mod tidy` chalao to galat wala automatic remove ho jaayega.

#### Indirect dependencies (doosra block, `// indirect` comment ke saath)

Ye wo packages hain jo tumhari direct deps ko **chahiye hote hain** but tum khud import nahi karte:

| Package | Kis ke liye |
|---|---|
| `golang.org/x/net` | grpc HTTP/2 ke liye use karta hai |
| `golang.org/x/sys` | low-level OS calls |
| `golang.org/x/text` | unicode handling |
| `google.golang.org/genproto/googleapis/rpc` | shared proto types (jaise `status.proto`, `codes`) |

Tumhe inse directly matlab nahi, but compile time pe present hone chahiye.

> **`// indirect` comment** Go ke tools automatic add/remove karte hain. Tum manual edit karoge to `go mod tidy` reset kar dega.

---

## Version pinning kaise kaam karti hai?

Look:

```
google.golang.org/grpc v1.81.1
```

`v1.81.1` semantic version hai (`MAJOR.MINOR.PATCH`):

- `MAJOR` change → breaking changes (1.x → 2.x me upgrade karna safe nahi)
- `MINOR` change → naye features, compatible
- `PATCH` change → bug fixes only

Go modules **minimum version selection** use karta hai. Agar tumhara module `v1.81.1` chahiye aur dependency `v1.79.0` chahiye, to `v1.81.1` win karta hai (higher).

---

## `go.sum` file kya hai?

Tumhare folder me `go.sum` bhi hai. Vo file har dependency ke **cryptographic hash** rakhta hai. Iska kaam:

- Reproducibility — same `go.sum` matlab same exact bytes har machine pe.
- Security — agar koi attacker dependency ka content badle, hash mismatch hoga aur build fail.

**Kabhi mat edit karo `go.sum`.** Vo automatic manage hoti hai.

---

## Common commands jo `go.mod` change karte hain

| Command | Kya karta hai |
|---|---|
| `go mod init example.com/clientStream` | Naya `go.mod` banata hai |
| `go get <pkg>@<version>` | Dependency add/upgrade |
| `go get <pkg>@latest` | Latest version |
| `go get -u ./...` | Saari deps upgrade |
| `go mod tidy` | Unused deps remove + missing add (sabse useful) |
| `go mod download` | Saari deps download (CI me kaam aata hai) |

---

## Iss project me sabse pehle kya karna chahiye

```powershell
cd "gRPC project\Greet Client Stream"
go mod tidy
```

Ye command:

1. **`github.com/golang/protobuf` ko remove kar dega** direct se (kyunki tumhare code me use nahi hota). Indirect ban jaayega ya bilkul drop ho jaayega.
2. Saari indirect deps verify karega.
3. Unused entries clean karega.

**Habit banao**: source files badle ke baad `go mod tidy` chalao.

---

## Practical scenarios

### Scenario 1: Naya package use karna chahta hu

```powershell
go get google.golang.org/grpc/credentials/insecure
```

Ye `go.mod` me automatically add ho jaayega.

### Scenario 2: Imports clean karna

```powershell
go mod tidy
```

Sare unused remove, missing add. Ye habit banao.

### Scenario 3: Saari deps upgrade

```powershell
go get -u ./...
go mod tidy
```

---

## TL;DR

| Cheez | Matlab |
|---|---|
| `module example.com/clientStream` | Project ka naam, import paths ka root |
| `go 1.26.2` | Minimum compiler version |
| `require ( ... )` direct | Tumne import kiye hue packages |
| `require ( ... )` indirect | Tumhari deps ki deps |
| `go.sum` | Cryptographic hashes (mat edit karo) |
| `go mod tidy` | Auto-cleanup command (use karte raho) |
