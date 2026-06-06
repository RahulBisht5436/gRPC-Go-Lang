# `context.Context` — Detailed study notes

> Ye file Go aur gRPC me `context.Context` ke baare me **complete study material** hai. Conversation se nikla — agar baad me revise karna ho to seedha yaha aao.

## Index

1. [Big picture: context kyu exist karta hai?](#1-big-picture-context-kyu-exist-karta-hai)
2. [`context.Background()` kya hai?](#2-contextbackground-kya-hai)
3. [`context.TODO()` aur Background ka difference](#3-contexttodo-aur-background-ka-difference)
4. [4 Problems jo context solve karta hai](#4-4-problems-jo-context-solve-karta-hai)
   - [Problem 1: Cancellation propagation](#problem-1-cancellation-propagation)
   - [Problem 2: Deadlines / timeouts](#problem-2-deadlines--timeouts)
   - [Problem 3: Resource cleanup](#problem-3-resource-cleanup)
   - [Problem 4: Request-scoped values](#problem-4-request-scoped-values)
5. [Context tree — wrapping pattern](#5-context-tree--wrapping-pattern)
6. [Saare context constructors](#6-saare-context-constructors)
7. [Context interface — pura contract](#7-context-interface--pura-contract)
8. [`defer cancel()` pattern — kabhi mat bhulna](#8-defer-cancel-pattern--kabhi-mat-bhulna)
9. [gRPC me context kaise wire pe travel karta hai?](#9-grpc-me-context-kaise-wire-pe-travel-karta-hai)
10. [Common patterns aur best practices](#10-common-patterns-aur-best-practices)
11. [Common galtiyaan — kya nahi karna](#11-common-galtiyaan--kya-nahi-karna)
12. [Real-world examples](#12-real-world-examples)
13. [TL;DR cheat sheet](#13-tldr-cheat-sheet)

---

## 1. Big picture: context kyu exist karta hai?

`context.Context` Go ka **standard interface** hai jo network/IO operations ke liye 4 problems solve karta hai:

1. **Cancellation propagation** — kaam rok do, bhulna mat
2. **Deadline enforcement** — kab tak intezaar
3. **Resource cleanup** — leaks rokna
4. **Request-scoped values** — auth tokens, trace IDs, etc.

Bina context ke distributed systems me ye saari cheezein **manually** karni padti — bohot dard, bohot bugs.

> **Ek line me**: Context ek "control channel" hai jo function calls ke chain me deadline + cancellation + metadata propagate karta hai automatically.

---

## 2. `context.Background()` kya hai?

`context.Background()` standard library ka function hai jo ek **empty, never-canceled, no-deadline** context return karta hai.

Iski properties:

| Property | Value |
|---|---|
| Deadline | None (kabhi expire nahi hoga) |
| Done channel | `nil` (kabhi cancel nahi hoga) |
| Err() | `nil` always |
| Values | Khali |

Iska matlab — **agar tum sirf `context.Background()` use karte ho**:

> RPC tab tak hang ho sakta hai jab tak server jawab na de ya network hi tut na jaaye.

Server agar 5 minute le, client 5 minute wait karega. Server agar response **kabhi nahi** bheje, client **forever** stuck.

### Implementation (standard library)

```go
func Background() Context {
    return backgroundCtx{}
}

type backgroundCtx struct{}

func (backgroundCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (backgroundCtx) Done() <-chan struct{}        { return nil }
func (backgroundCtx) Err() error                   { return nil }
func (backgroundCtx) Value(key any) any            { return nil }
```

Bilkul empty struct — **zero memory cost**. Ye literally singleton type ka instance hai.

### Kab use karna hai?

- `main()` me top-level pe — "ye root context hai"
- Tests me — "test code ka koi parent context nahi"
- Background goroutines me — "ye long-running background work hai"

### Kab NAHI use karna?

- Inside HTTP handlers — `r.Context()` use karo
- Inside gRPC handlers — passed `ctx` parameter forward karo
- Jab koi parent context milta ho — usse forward karo

---

## 3. `context.TODO()` aur Background ka difference

Functionality **bilkul same**, sirf **intent** alag:

| Function | Kab use karo |
|---|---|
| `context.Background()` | Top-level / `main()` me — confident ho ki "ye root hai" |
| `context.TODO()` | Placeholder — "abhi pata nahi kaunsa context use karu, baad me decide" |

```go
// In main.go — root context
ctx := context.Background()

// Inside a function where you don't know what to pass yet
func processData() {
    ctx := context.TODO()  // FIXME: receive ctx as parameter
    db.Query(ctx, ...)
}
```

`context.TODO()` linter ke liye useful — `staticcheck` jaise tools attention dete hain "yahan refactor needed".

---

## 4. 4 Problems jo context solve karta hai

### Problem 1: Cancellation propagation

**Story**: Tumhara gRPC server `Greet` handler me ek slow DB query chala raha hai.

#### Bina context (broken)

```go
func (s *Server) Greet(in *pb.GreetRequest) (*pb.GreetResponse, error) {
    user := s.db.Query("SELECT * FROM users WHERE name = ?", in.FirstName)
    // ↑ ye 10 second laga sakta hai
    return &pb.GreetResponse{Result: "Hello " + user.Name}, nil
}
```

5 problematic scenarios:

| Scenario | Bina context kya hota |
|---|---|
| Client ne disconnect kar diya 2 sec baad | Server poori 10 sec DB load karta rahega — wasted work |
| Client ne 3 sec timeout set kiya tha | Server pe still 10-second query — wasted resources |
| Load balancer ne retry kiya | Dono nodes pe same query — DB pe double load |
| 1000 clients flood kar diye | 1000 useless queries chalu — DB DOS |
| Process shutdown ho raha hai | Goroutines hang, graceful exit asambhav |

#### Context ke saath (fixed)

```go
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
    user, err := s.db.QueryContext(ctx, "SELECT ...", in.FirstName)
    // ↑ DB driver ctx.Done() pe listen karta hai
    if err != nil {
        return nil, err
    }
    return &pb.GreetResponse{Result: "Hello " + user.Name}, nil
}
```

Cancel cascade:

```
Client cancels
     │
     ▼
gRPC framework detects (HTTP/2 RST_STREAM frame)
     │
     ▼
ctx.Done() channel closes  ← magic moment
     │
     ▼
DB driver sees Done() → cancels query mid-execution
     │
     ▼
Goroutine cleanly exits → memory freed → DB connection freed
```

#### Multi-hop cancellation chain

```
HTTP request → HTTP server → gRPC client → gRPC server → DB
   |             |              |              |          |
   ctx.0       ctx.1         ctx.2          ctx.3     ctx.4
   (parent)    (child)       (child)         (child)   (child)
```

Agar `ctx.0` cancel ho jaaye, **automatic** `ctx.1, ctx.2, ctx.3, ctx.4` sab cancel ho jaayenge.

### Problem 2: Deadlines / timeouts

**Story**: Tumne gRPC client likha jo agar server response na de to forever hang.

#### Manual approach (without context) — painful

```go
done := make(chan result)
go func() {
    res, err := client.Greet(req)
    done <- result{res, err}
}()

select {
case r := <-done:
    // got response
case <-time.After(5 * time.Second):
    // timeout — but goroutine still running in background!
}
```

Problems:
- Goroutine leak — `client.Greet` chalu rahega even after timeout
- Bahut boilerplate har RPC ke liye
- Cancellation propagate nahi hoti

#### Context approach — clean

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

res, err := client.Greet(ctx, req)
```

5 second baad:
- Client side: error returns automatic with `codes.DeadlineExceeded`
- Server side: `ctx.Done()` close ho gaya — handler khud ko cancel kar sakta
- DB layer: query abort ho jaata
- **Pura chain free**

#### Distributed deadline propagation

```
client side:
    ctx with 5s deadline
            │
            │  HTTP/2 header: grpc-timeout: 5S
            ▼
server side:
    ctx with 5s deadline (same!)
            │
            │  forward to next service
            ▼
internal microservice:
    ctx with ~4.8s deadline (kuch travel time gaya)
```

gRPC framework automatically deadline ko HTTP/2 headers me serialize karta hai (`grpc-timeout: 5S`) aur server side reconstruct karta hai. **Yahi distributed deadline propagation hai**.

### Problem 3: Resource cleanup

`defer cancel()` pattern:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

`cancel` function internal goroutines aur timer free karta hai. Agar tum `cancel()` skip kar do:

```go
ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
res, err := client.Greet(ctx, req)
// res 1 second me aa gayi, but context ke andar timer 5 second tak chalega
// Goroutine leak!
```

1000 RPC calls = 1000 leaked timer goroutines = memory leak.

`defer cancel()` ye guarantee karta hai cleanup hamesha hoga, even if function panic kare.

### Problem 4: Request-scoped values

**Story**: Auth token aur request ID har downstream function tak pahuchana.

#### Bad solution: function arguments

```go
func handler(token string, requestID string) {
    callServiceA(token, requestID, ...)
    callServiceB(token, requestID, ...)
}
```

Har function me extra parameters. Naye field add karna mean hundreds of function signatures change.

#### Context solution

```go
func handler(ctx context.Context) {
    ctx = context.WithValue(ctx, "token", "Bearer xyz")
    ctx = context.WithValue(ctx, "request_id", "abc-123")

    callServiceA(ctx, ...)
    callServiceB(ctx, ...)
}

func callServiceA(ctx context.Context, ...) {
    token := ctx.Value("token").(string)
}
```

Ek hi `ctx` parameter — **infinite metadata** carry kar sakta. Plus gRPC me ye automatically wire pe headers me convert hota hai.

> ⚠️ **`context.WithValue` ko abuse mat karo.** Sirf request-scoped cross-cutting data ke liye (auth, tracing, request IDs). Business data ke liye function arguments use karo.

---

## 5. Context tree — wrapping pattern

Context ek **tree structure** hai. Always start from `Background()` aur upar features add karte jao:

```
                  context.Background()
                  (root, no deadline)
                          │
            ┌─────────────┼──────────────┐
            ▼             ▼              ▼
       WithTimeout   WithCancel     WithValue
       (5 sec)       (manual)       ("traceID", "abc")
            │
            ▼
       client.Greet(ctx, req)   ← this RPC will respect 5s deadline
```

Har "wrapper" function naya **child** context banata hai. Child apne parent ki properties **inherit** karta hai aur apni add karta hai. Parent cancel hua, **saare children automatic cancel**.

### Stacking example

```go
ctx := context.Background()

ctx = context.WithValue(ctx, "user_id", "rahul")

ctx, cancel1 := context.WithTimeout(ctx, 10*time.Second)
defer cancel1()

ctx, cancel2 := context.WithCancel(ctx)
defer cancel2()

// Iss ctx me hai:
// - 10-second deadline
// - Manual cancellation handle
// - Value "user_id" = "rahul"
```

Inheritance:
- Agar `cancel2()` call kar do → sirf ye child cancel
- Agar `cancel1()` call ho jaaye → cancel2 child bhi automatic cancel
- Agar parent (Background) cancel ho jaaye → poora tree cancel

---

## 6. Saare context constructors

| Function | Returns | Purpose |
|---|---|---|
| `context.Background()` | Root context | Top-level starting point |
| `context.TODO()` | Root context | Placeholder ("abhi nahi pata") |
| `context.WithCancel(parent)` | `(ctx, cancel)` | Manual cancellation handle |
| `context.WithTimeout(parent, d)` | `(ctx, cancel)` | Deadline = now + d |
| `context.WithDeadline(parent, t)` | `(ctx, cancel)` | Specific deadline time |
| `context.WithValue(parent, key, val)` | `ctx` | Attach key-value pair |
| `context.WithCancelCause(parent)` | `(ctx, cancel-with-error)` | Cancellation with reason (Go 1.20+) |
| `context.WithoutCancel(parent)` | `ctx` | Detach cancellation but keep values (Go 1.21+) |
| `context.AfterFunc(ctx, f)` | `stop func` | Run f when ctx is done (Go 1.21+) |

### Detailed examples

#### `WithCancel` — manual cancellation

```go
ctx, cancel := context.WithCancel(context.Background())

go someBackgroundWork(ctx)

// Kabhi bhi cancel call karke kaam rok do
time.Sleep(5*time.Second)
cancel()
```

#### `WithTimeout` — relative duration

```go
// 5 seconds from now
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()
```

#### `WithDeadline` — absolute time

```go
deadline := time.Now().Add(5*time.Second)
ctx, cancel := context.WithDeadline(parent, deadline)
defer cancel()
```

`WithTimeout` actually `WithDeadline(parent, now + d)` ka shortcut hai.

#### `WithValue` — key-value attach

```go
ctx := context.WithValue(parent, userIDKey, "rahul-123")

// Later:
userID := ctx.Value(userIDKey).(string)
```

> ⚠️ Key kabhi `string` type ka raw nahi rakhna. **Type-safe key** banao:
>
> ```go
> type ctxKey int
> const userIDKey ctxKey = iota
> ```
>
> Reason: agar do packages same string key use kare to collision ho jaayegi.

#### `WithCancelCause` — cancellation with reason

```go
ctx, cancel := context.WithCancelCause(parent)

// Later, with explicit reason
cancel(fmt.Errorf("user logged out"))

// Inside the called function:
if err := ctx.Err(); err != nil {
    cause := context.Cause(ctx)
    log.Printf("canceled because: %v", cause)
}
```

---

## 7. Context interface — pura contract

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
```

### `Deadline()`

```go
deadline, ok := ctx.Deadline()
if ok {
    fmt.Printf("Will cancel at %v", deadline)
} else {
    fmt.Println("No deadline set")
}
```

### `Done()` — sabse important method

Returns a channel that **closes** when the context is canceled. Tumhare goroutines me iss pattern me listen karte ho:

```go
select {
case <-ctx.Done():
    return ctx.Err() // canceled or timed out
case result := <-workChannel:
    return result
}
```

> Closed channel se receive karna **immediate non-blocking** hota hai. Ye property iss design ka core hai.

### `Err()`

Returns:
- `nil` — agar context still active hai
- `context.Canceled` — agar manually cancel hua
- `context.DeadlineExceeded` — agar timeout hua

```go
if err := ctx.Err(); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // timeout
    } else if errors.Is(err, context.Canceled) {
        // explicit cancel
    }
}
```

### `Value(key)`

Key associated value retrieve karta hai. Type assertion karna padta hai:

```go
userID, ok := ctx.Value(userIDKey).(string)
if !ok {
    // value not present or wrong type
}
```

---

## 8. `defer cancel()` pattern — kabhi mat bhulna

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()
```

### Kyu zaruri?

`WithTimeout` (aur `WithCancel`, `WithDeadline`) internally goroutines aur timers create karte hain. Agar tum `cancel()` nahi call karte, vo resources tab tak alive rehte hain jab tak parent cancel na ho. Yaani:

```go
// BAD — leak
ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
doSomething(ctx)
// returns in 1ms, but timer goroutine ALIVE for 5 seconds
```

```go
// GOOD
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
doSomething(ctx)
// returns in 1ms, cancel() immediately frees timer
```

### Idempotency

`cancel()` ko multiple times call karna **safe** hai. Pehli call effective, baaki no-op.

```go
ctx, cancel := context.WithCancel(parent)
defer cancel()  // panic-safe even if cancel was already called
cancel()        // explicit cancel
// ...
// defer ke time pe cancel() again, but no harm
```

### `go vet` warning

```go
func badPattern() context.Context {
    ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
    return ctx  // ⚠️ go vet: lostcancel
}
```

`go vet` automatic warning dega. Yeh follow karo.

---

## 9. gRPC me context kaise wire pe travel karta hai?

gRPC ka context-aware nature **HTTP/2 headers** ke through implement hota hai.

### Outgoing (client → wire)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer xyz")
defer cancel()

client.Greet(ctx, req)
```

Wire pe ye HTTP/2 frame banta hai:

```
HEADERS:
  :method: POST
  :path: /greet.GreetService/Greet
  content-type: application/grpc
  grpc-timeout: 5S            ← deadline serialized
  authorization: Bearer xyz   ← metadata serialized
DATA:
  [protobuf bytes]
```

### Incoming (wire → server)

Server side gRPC framework:

1. Headers parse karta — `grpc-timeout: 5S` dekha
2. New context banaya `WithTimeout(parent, 5*time.Second)`
3. Metadata extract kiya
4. Tumhara handler call kiya is fresh `ctx` ke saath

```go
func (s *Server) Greet(ctx context.Context, in *pb.GreetRequest) (*pb.GreetResponse, error) {
    md, _ := metadata.FromIncomingContext(ctx)
    token := md.Get("authorization")
    // verify token...

    select {
    case <-ctx.Done():
        return nil, ctx.Err()  // client canceled or timed out
    default:
    }

    // ...
}
```

### Disconnect detection

Agar client ne TCP connection close ki, gRPC framework **HTTP/2 RST_STREAM** detect karta hai. Andar fir vo handler ka `ctx` cancel kar deta hai. Tumhare handler ko `ctx.Done()` se signal milta.

---

## 10. Common patterns aur best practices

### Pattern 1: Function signature — ctx pehla parameter

```go
// ✅ Good
func DoWork(ctx context.Context, input string) (Result, error)

// ❌ Bad
func DoWork(input string, ctx context.Context) (Result, error)

// ❌ Worse — ctx as struct field
type Worker struct {
    ctx context.Context
}
```

`ctx` **hamesha** pehla parameter. Go community strict hai is convention pe.

### Pattern 2: Context inherit karo, mat banao

```go
// ✅ Good — inherit from parent
func handler(ctx context.Context) {
    callDownstream(ctx)
}

// ❌ Bad — discard parent
func handler(ctx context.Context) {
    callDownstream(context.Background())  // loses parent's deadline/cancel!
}
```

### Pattern 3: HTTP handler me r.Context() use karo

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // ← user disconnect ke saath cancel hoga
    
    res, err := client.Greet(ctx, req)
    // ...
}
```

### Pattern 4: Per-RPC timeout (not global)

```go
// ✅ Good — har call ka apna timeout
for _, item := range items {
    ctx, cancel := context.WithTimeout(parentCtx, 1*time.Second)
    res, err := client.Process(ctx, item)
    cancel()
    // ...
}

// ❌ Bad — saari calls ek hi 1-second me complete hone chahiye
ctx, cancel := context.WithTimeout(parentCtx, 1*time.Second)
defer cancel()
for _, item := range items {
    client.Process(ctx, item)  // 5 items? 4th onwards fail honge
}
```

### Pattern 5: Type-safe context keys

```go
// ✅ Good
type ctxKey int
const userIDKey ctxKey = iota

ctx = context.WithValue(ctx, userIDKey, "rahul")
userID := ctx.Value(userIDKey).(string)

// ❌ Bad — collisions possible
ctx = context.WithValue(ctx, "user_id", "rahul")
```

### Pattern 6: `select` with `ctx.Done()` in long-running work

```go
func processItems(ctx context.Context, items []Item) error {
    for _, item := range items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err := process(item); err != nil {
            return err
        }
    }
    return nil
}
```

---

## 11. Common galtiyaan — kya nahi karna

### ❌ Galti 1: ctx ko struct me store karna

```go
type Service struct {
    ctx context.Context  // BAD
}
```

Reason: ctx **per-request** hai, struct **long-lived**. Mismatch.

### ❌ Galti 2: nil context pass karna

```go
client.Greet(nil, req)  // panic guaranteed
```

`context.Background()` ya `context.TODO()` use karo.

### ❌ Galti 3: cancel() bhul jaana

```go
ctx, _ := context.WithTimeout(parent, 5*time.Second)
// goroutine leak!
```

Always `defer cancel()`.

### ❌ Galti 4: parent context discard karna

```go
func handler(ctx context.Context) {
    newCtx := context.Background()  // BAD — parent ki deadline gayi
    callDownstream(newCtx)
}
```

### ❌ Galti 5: WithValue ko data dump banana

```go
// BAD
ctx = context.WithValue(ctx, "user", user)
ctx = context.WithValue(ctx, "order", order)
ctx = context.WithValue(ctx, "preferences", prefs)
```

`ctx.Value` cross-cutting concerns ke liye hai (auth, request ID, tracing). Business data **function arguments** ke through pass karo.

### ❌ Galti 6: ctx.Done() check skip karna long work me

```go
// BAD — loop kabhi cancel detect nahi karega
func processBigData(ctx context.Context, data []byte) {
    for _, chunk := range chunks(data) {
        heavyWork(chunk)  // never checks ctx
    }
}
```

```go
// GOOD
for _, chunk := range chunks(data) {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    heavyWork(chunk)
}
```

### ❌ Galti 7: Raw string keys

```go
// BAD — different package me same key define ho sakti
ctx = context.WithValue(ctx, "token", "...")
```

Type-safe keys use karo.

---

## 12. Real-world examples

### Example 1: Microservice with budget

```go
func userDashboard(w http.ResponseWriter, r *http.Request) {
    // 1 second total budget
    ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
    defer cancel()

    profile, err := userService.GetProfile(ctx, userID)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    orders, err := orderService.GetOrders(ctx, userID)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    inbox, err := notifService.GetInbox(ctx, userID)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(map[string]any{
        "profile": profile,
        "orders":  orders,
        "inbox":   inbox,
    })
}
```

Saari calls **same** `ctx` use karti hain. Total time 1 second cap. Agar 1st call 800ms le, 2nd ke paas sirf 200ms. Agar user disconnect kare, sab cancel.

### Example 2: Graceful shutdown

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    server := &http.Server{Addr: ":8080"}

    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    <-ctx.Done()
    log.Println("Shutting down...")
    
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()
    
    server.Shutdown(shutdownCtx)
}
```

Ctrl+C dabai — `ctx.Done()` close — graceful shutdown trigger.

### Example 3: Worker pool with cancellation

```go
func runWorkers(ctx context.Context, jobs <-chan Job) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case job, ok := <-jobs:
                    if !ok {
                        return
                    }
                    process(ctx, job)
                }
            }
        }()
    }
    wg.Wait()
}
```

ctx cancel ho jaaye — 10 workers all gracefully exit.

### Example 4: gRPC streaming with timeout

```go
func (s *Server) GreetManyTimes(req *pb.GreetRequest, stream pb.GreetService_GreetManyTimesServer) error {
    ctx := stream.Context()
    
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()  // client disconnected
        default:
        }
        
        if err := stream.Send(&pb.GreetResponse{Result: fmt.Sprintf("Hello %d", i)}); err != nil {
            return err
        }
        
        time.Sleep(100 * time.Millisecond)
    }
    return nil
}
```

Streaming RPC — har iteration pe ctx check karna important hai.

---

## 13. TL;DR cheat sheet

### Kab kya use karu?

| Situation | Use |
|---|---|
| `main()`, top-level | `context.Background()` |
| Function jisme abhi context pata nahi | `context.TODO()` |
| Naya RPC client call with timeout | `context.WithTimeout(parent, d)` + `defer cancel()` |
| Manual cancellation chahiye | `context.WithCancel(parent)` + `defer cancel()` |
| Specific time pe expire | `context.WithDeadline(parent, t)` + `defer cancel()` |
| Cross-cutting metadata | `context.WithValue(parent, key, val)` |
| HTTP handler | `r.Context()` from request |
| gRPC server handler | passed `ctx` parameter |
| Ctrl+C handling | `signal.NotifyContext(...)` |

### Sabse important rules

1. **`ctx` always pehla parameter** — Go convention strict.
2. **Always `defer cancel()`** when using `WithCancel`/`WithTimeout`/`WithDeadline`.
3. **Inherit parent ctx**, mat discard karo.
4. **`ctx.Value` sirf cross-cutting concerns** ke liye, business data ke liye nahi.
5. **Long loops me `ctx.Done()` check** karte raho.
6. **nil context kabhi pass mat karna** — Background/TODO use karo.
7. **Type-safe context keys** banao (`type ctxKey int`).

### Mental model

> Context ek "control + metadata channel" hai jo function call chain me **deadline, cancellation, aur cross-cutting data** automatically propagate karta hai. Bina iske distributed systems me leaks aur orphan goroutines guarantee hain.

---

## Further reading

Standard library docs:
- https://pkg.go.dev/context

Official Go blog posts:
- "Go Concurrency Patterns: Context" (Sameer Ajmani, 2014)

gRPC-specific:
- gRPC `metadata` package: how context becomes wire headers
- HTTP/2 `grpc-timeout` header spec

Related concepts:
- `errgroup` package — context + sync.WaitGroup combined
- `context.AfterFunc` (Go 1.21+) — cleanup callbacks
- `context.WithCancelCause` (Go 1.20+) — cancellation reasons
