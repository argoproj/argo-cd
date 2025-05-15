# Tilt Development

> Tilt provides a real-time web UI that offers better visibility into logs, health status, and dependencies, making debugging easier compared to relying solely on terminal outputs. With a single `tilt up` command, developers can spin up all required services without managing multiple processes manually, simplifying the local development workflow. Tilt also integrates seamlessly with Docker and Kubernetes, allowing for efficient container-based development. Unlike goreman, which lacks dynamic config reloading, Tilt can detect and apply changes to Kubernetes YAML and Helm charts without full restarts, making it more efficient for iterative development.

### Prerequisites
* kubernetes environment (kind, minikube, k3d, etc.)
* tilt (`brew install tilt`)
* kustomize

### Running
Spin up environment by running `tilt up` in the root directory of the repo

Spin down and remove deployment manifests: `tilt down`
