# findlock
Parses Go tracebacks and finds possible deadlocks

This works by checking how many locks have the same memory address. If there are multiple goroutines with the same lock but have the same stack trace, then only one entry is shown. The tool is not perfect but helped me to find locks where I would have over 10k goroutines.

Example usage:

```
 $ ~/projects/findlock/findlock stack.txt
DETECTED 2 POSSIBLE DEADLOCK(S)
- 1 call(s) to Lock() for 0xc03d73ba84, 1 unique:
  ┌┤ sync.runtime_SemacquireMutex @ /usr/local/go/src/runtime/sema.go:71
  ├ 0: sync.runtime_SemacquireMutex             @ /usr/local/go/src/runtime/sema.go:71
  ├ 1: sync.(*Mutex).lockSlow                   @ /usr/local/go/src/sync/mutex.go:138
  ├ 2: sync.(*Mutex).Lock                       @ /usr/local/go/src/sync/mutex.go:81
  ├ 3: main.clientDatabaseChange                @ /tmp/projects/go/src/proj1/cmd/runner/database-changes.go:22
  ├ 4: main.refresh                             @ /tmp/projects/go/src/proj1/cmd/runner/sync.go:53
  ├ 5: proj1/pkg/db.(*PostgresDB).HandleChanges @ /tmp/projects/go/src/proj1/pkg/db/postgresql.go:271
  └ 6: main.initOrDie.func2                     @ /tmp/projects/go/src/proj1/cmd/runner/sync.go:111
- 1 call(s) to Lock() for 0xc0003b2028, 1 unique:
  ┌┤ sync.runtime_Semacquire @ /usr/local/go/src/runtime/sema.go:56
  ├ 0: sync.runtime_Semacquire                  @ /usr/local/go/src/runtime/sema.go:56
  ├ 1: sync.(*WaitGroup).Wait                   @ /usr/local/go/src/sync/waitgroup.go:130
  ├ 2: google.golang.org/grpc.(*Server).serveStreams @ /root/go/pkg/mod/google.golang.org/grpc@v1.27.1/server.go:731
  └ 3: google.golang.org/grpc.(*Server).handleRawConn.func1 @ /root/go/pkg/mod/google.golang.org/grpc@v1.27.1/server.go:679

```
