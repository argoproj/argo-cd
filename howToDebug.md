#### Required for argo-repo-server
When running telepresence, expose all ports used by the application!
```
 telepresence --swap-deployment argocd-repo-server --method=vpn-tcp \
    --namespace argocd --env-file .envrc.remote \
    --expose 8082:8082 --run zsh
export ARGOCD_GPG_ENABLED=false
```
Copy file `hack/git-ask-pass.sh` to directory where repo-serve is started and replace 
the username and password with your credentials.
Then set the environment Variable GIT_ASKPASS by executing the following statement.
 ` export GIT_ASKPASS=/home/path/to/git-ask-pass.sh`
```
dlv  --listen=:8086 --headless=true \
    --api-version=2 --accept-multiclient exec ./argocd-repo-server \
    -- --redis 100.67.117.207:6379
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