#### Required for argo-repo-server
When running telepresence, expose all ports used by the application!
```
 telepresence --swap-deployment argocd-repo-server --method=vpn-tcp \
    --namespace argocd --env-file .envrc.remote \
    --expose 8082:8082 --run zsh
export ARGOCD_GPG_ENABLED=false
dlv  --listen=:8086 --headless=true \
    --api-version=2 --accept-multiclient exec ./argocd-repo-server
```
### Required for argocd-application-controller
```
telepresence --swap-deployment argocd-application-controller \
    --method=vpn-tcp --namespace argocd \
    --env-file .envrc.remote --expose 8082:8082 --run zsh
export ARGOCD_FAKE_IN_CLUSTER=true
export KUBECONFIG=/home/sfa/Documents/xnsgdev01/config \
 dlv  --listen=:8086 --headless=true --api-version=2 
      --accept-multiclient exec ./argocd-application-controller -- \ 
      --kubeconfig ~/Documents/xnsgdev01/config \
      --redis 100.67.117.207:6379 --repo-server 100.77.120.131:8081
```