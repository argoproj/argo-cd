# Profiling with pprof

pprof is a powerful tool for profiling Go applications. It is part of the Go standard library and provides rich insights into the performance characteristics of Go programs. Profiling with pprof involves gathering runtime statistics about the application's execution, such as CPU usage, memory allocation, and contention.

## Installation

pprof is included in the Go standard library and integrated in all ArgoCD components, so no separate installation is required. To enable the profiling endpoints, you just need to set the environment variable `ARGO_PPROF` with the port that you want to use for exposing the endpoints. e.g: `ARGO_PPROF=8888` will start the profiling endpoints using the port `8888`.

## Basic Usage
pprof operates in two modes: interactive and non-interactive. In the non-interactive mode, it generates profiling data that can be analyzed later. In the interactive mode, it launches a web server to visualize the profiling data in real-time.

> Note: Replace http://localhost:6060 with the appropriate URL of your Go application.

### Generating CPU Profiles
To generate a CPU profile for a Go application, you can use the following command:

```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Generating Heap Profiles
To generate a heap profile, you can use the following command:

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Interactive Mode
To run pprof in interactive mode, you can use the following command:

```bash
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile
```
This command launches a web server and opens a browser window displaying the profiling data.

## More info

To extend the information about how to profile golang applications, you could find interesting [this article](https://go.dev/blog/pprof) from golang blog.