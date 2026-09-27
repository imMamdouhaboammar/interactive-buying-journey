# Runtime Metrics, Benchmarking & Memory Accounting

---

## 1. Heap Memory Metric Underflow During Benchmark Runs

### Context
Executing the 10,000 variants performance benchmark in `internal/catalog/benchmark_test.go` and reporting memory statistics.

### What happened
The benchmark harness logged an absurdly high memory allocation number:
```text
benchmark_test.go:120: Heap Memory Allocated: 17592186044413.17 MB
```

### Observable symptom
The test harness claimed the ingestion of 10,000 variants allocated over 17 petabytes of RAM on an Apple M4 laptop with 16GB of physical memory.

### Impact
Medium. Misleading benchmark reports and potential assertions failing due to unsigned integer underflow.

### Incorrect assumption
The test assumed that `runtime.MemStats.Alloc` was monotonic across the benchmark duration:
```go
var memBefore runtime.MemStats
runtime.ReadMemStats(&memBefore)
// ... run 8-second ingest ...
var memAfter runtime.MemStats
runtime.ReadMemStats(&memAfter)
allocMB := float64(memAfter.Alloc - memBefore.Alloc) / (1024 * 1024)
```

### Root cause
**Confirmed**. In the Go runtime, `runtime.MemStats.Alloc` is a gauge representing currently allocated, live heap objects. It is **not** monotonic. If the Go garbage collector runs during a multi-second benchmark, memory from before the benchmark is freed. If `memAfter.Alloc < memBefore.Alloc`, subtracting two `uint64` values results in an unsigned integer underflow:
$$\text{uint64}(10\text{ MB}) - \text{uint64}(15\text{ MB}) = 18,446,744,073,704,306,616 \approx 17.5 \times 10^{12}\text{ KB} \approx 17.5\text{ PB}$$

### Why the architecture allowed it
The benchmark author conflated *instantaneous heap in use* (`Alloc`) with *cumulative bytes allocated* (`TotalAlloc`).

### Fix
1. Used `TotalAlloc` (strictly monotonic cumulative counter of bytes allocated for heap objects, even if freed):
   ```go
   allocMB := float64(memAfter.TotalAlloc - memBefore.TotalAlloc) / (1024 * 1024)
   ```
2. Reported instantaneous active heap separately as a gauge:
   ```go
   heapInUseMB := float64(memAfter.Alloc) / (1024 * 1024)
   ```

### Verification
Reran `TestBenchmark_10kVariants`. Reported clean, realistic values:
```text
Cumulative Heap Alloc: 156.32 MB
Active Heap In-Use:    6.11 MB
```

### Prevention rule
**Monotonic Allocation Delta Accounting:** Never calculate memory deltas by subtracting gauge metrics like `Alloc`. Always compute deltas from monotonically increasing counters (`TotalAlloc`), and log gauge metrics as point-in-time snapshots.

### Reusable lesson
Whenever measuring resource consumption over time in managed runtimes (Go, Java, .NET, V8), metrics that can be reduced by garbage collection or compaction will underflow unsigned calculations. Distinguish gauges from monotonic counters.

### Related code
- `internal/catalog/benchmark_test.go`
- `docs/benchmarks/T02-10k-variants.md`

### Related tests
- `internal/catalog/benchmark_test.go:TestBenchmark_10kVariants`

### Status
Resolved.
