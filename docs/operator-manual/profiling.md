# Profiling with `pprof`

[`pprof`](https://go.dev/blog/pprof) is a powerful tool for profiling Go applications. It is part of the Go standard library and provides rich insights into the performance characteristics of Go programs, such as CPU usage, memory allocation, and contention.

## Basic Usage

Enable profiling endpoints by setting the environment variable `ARGO_PPROF` with your preferred port. For instance, `ARGO_PPROF=8888` will start profiling endpoints on port `8888`.

`pprof` has two modes: interactive and non-interactive. Non-interactive mode generates profiling data for future analysis. Interactive mode launches a web server to visualize the profiling data in real-time.

!!! Note "Port Forward"
    [Port forwarding](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/) is a more secure approach to access debug level information than exposing these endpoints via an Ingress.
    The below examples assume you have an Argo component forwarded to `http://localhost:6060`, but you can replace that with your preferred local port.

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
