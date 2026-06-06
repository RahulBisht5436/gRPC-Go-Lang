# `Makefile` — Build automation

## Pura file

```make
PROTO_DIR = proto

ifeq ($(OS), Windows_NT)
    OS = windows
    SHELL := powershell.exe
    .SHELLFLAGS := -NoProfile -Command
    PACKAGE = $(shell Get-Content go.mod -head 1 | Foreach-Object { $$data = $$_ -split " "; "{0}" -f $$data[1]})
    BIN = proto-go-course.exe
else
    UNAME := $(shell uname -s)
    ifeq ($(UNAME),Darwin)
        OS = macos
    else ifeq ($(UNAME),Linux)
        OS = linux
    else
    $(error OS not supported by this Makefile)
    endif
    PACKAGE = $(shell head -1 go.mod | awk '{print $$2}')
    BIN = proto-go-course
endif

.PHONY: build generate greet bump clean

build:  generate
    go build -o ${BIN} .

generate:
    protoc -I${PROTO_DIR} --go_opt=module=${PACKAGE} --go_out=. --go-grpc_out=. --go-grpc_opt=module=${PACKAGE} ${PROTO_DIR}/*.proto

greet: generate

bump: generate
    go get -u ./...

clean:
    rm ${PROTO_DIR}/*.pb.go
    rm ${BIN}
```

## Makefile kya hota hai?

Makefile ek **build automation** tool hai. Tum commands ko **targets** ke under define karte ho, fir `make <target>` se chala lete ho. Hand-typing avoid karne ke liye.

Tumhare project me 5 useful targets hain: `build`, `generate`, `greet`, `bump`, `clean`.

---

## Variables (top of file)

### `PROTO_DIR = proto`

Ek variable jisme `proto` folder ka naam hai. `${PROTO_DIR}` syntax se baad me reference hota hai. Iss tarah agar kabhi folder rename karna ho, ek hi jagah change karoge.

### OS detection block

```make
ifeq ($(OS), Windows_NT)
    OS = windows
    SHELL := powershell.exe
    .SHELLFLAGS := -NoProfile -Command
    PACKAGE = $(shell Get-Content go.mod -head 1 | ...)
    BIN = proto-go-course.exe
else
    ...
endif
```

`make` Linux/Mac pe `bash` use karta hai by default, lekin Windows pe shell `cmd.exe` ya `powershell.exe` hota hai. Iss block ka kaam:

1. **OS detect karo** — Windows hai ya Linux/Mac.
2. **Shell set karo** — Windows pe `powershell.exe`.
3. **`PACKAGE` variable** — go.mod ki pehli line se module name extract.
4. **`BIN` variable** — output binary ka naam (Windows me `.exe`).

### `PACKAGE = $(shell ...)` — go.mod se module name

```powershell
Get-Content go.mod -head 1 | Foreach-Object { $$data = $$_ -split " "; "{0}" -f $$data[1] }
```

Ye PowerShell command:

1. `go.mod` ki pehli line padhta hai (`module example.com/greet`).
2. Space pe split karta hai (`["module", "example.com/greet"]`).
3. Doosra element return karta hai (`example.com/greet`).

Linux/Mac equivalent:

```bash
head -1 go.mod | awk '{print $2}'
```

Iska use `protoc` command me hota hai — `--go_opt=module=${PACKAGE}` me. Ye `protoc` ko batata hai output files kaha generate karna.

> **Why?** Imagine module name `example.com/greet` hai aur tum folder `proto/` me proto files rakhe ho. `module=example.com/greet` flag se `protoc` smart ho jaata hai — vo absolute import path ko module ke relative me convert kar deta hai.

---

## `.PHONY` declaration

```make
.PHONY: build generate greet bump clean
```

`.PHONY` ek special declaration hai. Iska matlab:

> "Ye targets file names nahi hain — ye sirf naam hain commands ke."

Bina `.PHONY` ke, agar koi `generate` naam ki file folder me ho, to `make generate` confused ho jaata ki "build karna hai ya skip?". `.PHONY` se bolta hai "always rebuild".

---

## Targets — main meat

### Target syntax

```make
target_name: dependency1 dependency2
    command1
    command2
```

- Pehli line: target name + colon + dependencies (jo pehle banane chahiye).
- Subsequent lines: **TAB se start** (spaces NOT allowed).

### `generate`

```make
generate:
    protoc -I${PROTO_DIR} --go_opt=module=${PACKAGE} --go_out=. --go-grpc_out=. --go-grpc_opt=module=${PACKAGE} ${PROTO_DIR}/*.proto
```

Ye **proto files se Go code generate karta hai**. Hum pehle yahi command discuss kar chuke hain. Variables expand hone ke baad ye banta hai:

```bash
protoc -Iproto --go_opt=module=example.com/greet --go_out=. --go-grpc_out=. --go-grpc_opt=module=example.com/greet proto/*.proto
```

Result:
- `proto/greet.pb.go` ban jaata hai (messages).
- `proto/greet_grpc.pb.go` ban jaata hai (service interfaces).

Run kab karo? **Jab bhi `.proto` file edit karo.**

```powershell
make generate
```

### `greet`

```make
greet: generate
```

Notice — yaha **koi command nahi hai**, sirf dependency. Iska matlab:

> "make greet" basically `make generate` ka alias hai. Kuch extra nahi karta.

Tumne pehle pucha tha "make greet ke baad bin file kyu nahi banti" — yahi reason hai. Ye sirf proto regenerate karta hai, koi binary nahi banata.

Probably author ne `greet` rakha legacy reasons se. Agar tum chaho to is target ko kuch useful banane ke liye:

```make
greet: generate
    go run ./server
```

Ya kuch aisa.

### `build`

```make
build:  generate
    go build -o ${BIN} .
```

Logic:

1. Pehle `generate` chalao (dependency).
2. Phir `go build -o proto-go-course.exe .` chalao.

⚠️ **Issue**: ye target current setup me **fail karega**. Reason — `go build .` current folder (`greet/`) ko ek package maan ke build karta hai. But `greet/` me kuch `main.go` direct nahi hai — `main.go` to `server/` aur `client/` me hai.

Fix kya hoga? Targets ko split karo:

```make
server: generate
    go build -o server.exe ./server

client: generate
    go build -o client.exe ./client
```

Phir `make server` aur `make client` actual binaries banayenge.

### `bump`

```make
bump: generate
    go get -u ./...
```

Saari dependencies ko latest version pe upgrade karta hai. Phir `generate` bhi karta hai (because possibly grpc plugin generation pattern badal gaya hoga). Use sparingly — naye versions me breaking changes ho sakte hain.

### `clean`

```make
clean:
    rm ${PROTO_DIR}/*.pb.go
    rm ${BIN}
```

Generated files aur binary delete karta hai. Reset to scratch.

⚠️ Windows pe `rm` command nahi hota by default. PowerShell me `Remove-Item` ya alias `rm` chal sakta hai (since shell `powershell.exe` set hai upar). Kabhi-kabhi issue ho sakta hai.

---

## Common workflow

```powershell
# 1. Edit proto file
notepad proto\greet.proto

# 2. Regenerate Go code
make generate

# 3. Update server/client code
# (manual coding)

# 4. Run server
go run ./server

# 5. (separate terminal) Run client
go run ./client
```

---

## Troubleshooting

### "make: command not found" on Windows

Windows me `make` install karna padta hai. Options:
- **Chocolatey**: `choco install make`
- **Git Bash**: GNU Make pre-installed
- **Manual**: just run the protoc command directly without make

### "protoc: command not found"

`protoc` install karna padega. https://github.com/protocolbuffers/protobuf/releases

### "protoc-gen-go: program not found"

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Aur ensure that `$GOPATH/bin` (typically `~/go/bin`) PATH me hai.

### Tab vs spaces error

```
Makefile:25: *** missing separator.  Stop.
```

Make me commands **TAB se start hone chahiye**, spaces se nahi. Editor agar tabs ko spaces me convert kar de, ye error ata hai.

---

## TL;DR

| Target | Kaam |
|---|---|
| `make generate` | proto → Go code regenerate |
| `make greet` | Same as generate (alias) |
| `make build` | generate + binary build (currently buggy) |
| `make bump` | deps upgrade + regenerate |
| `make clean` | Generated files + binary delete |

**Sabse common command tum chalaoge: `make generate`.**
