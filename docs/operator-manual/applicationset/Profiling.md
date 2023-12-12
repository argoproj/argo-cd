# Profiling applicationset-controller using pprof

## Step 1: Update Deployment's Arguments

Modify the deployment configuration for applicationset-controller to include the pprof-addr argument. This will set the address and port for the pprof profiling server. Update your deployment YAML to include the following:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: applicationset-controller
spec:
  template:
    spec:
      containers:
      - name: applicationset-controller
        image: your/applicationset-controller-image:tag
        args:
        - "--pprof-addr=:YOUR_PORT"

```

## Step 2: Port Forward the Pod's Port
After updating the deployment, port forward the specified port to your local machine. Use the following command:

```bash
kubectl port-forward pod/applicationset-controller-pod YOUR_LOCAL_PORT:YOUR_PORT
```

## Step 3: Use pprof to Profile
Open your browser or use a tool like go tool pprof to access the profiling endpoints. For example, to profile memory allocations, you can use the following command:

```bash
go tool pprof http://localhost:YOUR_LOCAL_PORT/debug/pprof/allocs
```