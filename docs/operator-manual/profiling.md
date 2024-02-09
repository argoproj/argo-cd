# Profiling with `pprof`

[`pprof`](https://go.dev/blog/pprof) is a powerful tool for profiling Go applications. It is part of the Go standard library and provides rich insights into the performance characteristics of Go programs, such as CPU usage, memory allocation, and contention.

## Basic Usage

Enable profiling endpoints by setting the environment variable `ARGO_PPROF` with your preferred port. For instance, `ARGO_PPROF=8888` will start profiling endpoints on port `8888`.

`pprof` has two modes: interactive and non-interactive. Non-interactive mode generates profiling data for future analysis. Interactive mode launches a web server to visualize the profiling data in real-time.

!!! Note
    You should use port-forward for below commands, replacing http://localhost:6060 with the appropriate URL of your Argo component. Don't expose pprof server publically!

### Generate CPU Profile

Generate a CPU profile with the following command:

```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Generate Heap Profiles

Generate a heap profile with the following command:

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Interactive Mode

Use interactive mode with the following command:

```bash
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile
```

This starts a web server and opens a browser window displaying the profiling data.
