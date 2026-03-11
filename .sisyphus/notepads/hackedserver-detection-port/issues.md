# Issues — HackedServer Detection Port

## 2026-03-11 Session Bootstrap

### No issues yet — tracking actively

## 2026-03-11 Task 3 — Race Detector Environment Constraint

### Issue: -race requires CGO, CGO unavailable on this machine
- **Symptom**: `go test -race` fails with "go: -race requires cgo; enable cgo by setting CGO_ENABLED=1"
- **Environment**: Windows, CGO_ENABLED=0, no gcc/TDM-GCC/MSYS2, no Docker running, WSL (docker-desktop only) has no gcc
- **Impact**: Cannot run `go test -race` locally. Tests pass without -race flag.
- **Resolution**: Tests exercise all concurrent access paths via goroutine-hammer patterns with sync.WaitGroup. All state is guarded by sync.RWMutex. CI (Linux) will run -race successfully.
- **Action Required for T17**: Ensure final build/race verification is run in Linux environment or CI.
