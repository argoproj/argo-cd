# Kustomize

!!! warning Kustomize 1 vs 2
    Argo CD supports both versions, and auto-detects then by looking for `apiVersion/kind` is `kustomize.yaml`. 
    You're probably using version 2 now, so make sure you you have those fields.
    
You have three configuration options for Kustomize:

* `namePrefix` is a prefix appended to resources for Kustomize apps
* `imageTags` is a list of Kustomize 1.0 image tag overrides
* `images` is a list of Kustomize 2.0 image overrides
    
To use Kustomize with an overlay, point your path to the overlay.

!!! tip
    If you're generating resources, you should read up how to ignore those generated resources using the [`IgnoreExtraneous` compare option](compare-options.md).

## Private Remote Bases

If you have remote bases that are either (a) HTTPS and need username/password (b) SSH and need SSH private key, then they'll inherit that from the app's repo. 

This will work if the remote bases uses the same credentials/private key. It will not work if they use different ones. For security reasons your app only ever knows about it's own repo (not other team's or users repos), and so you won't be be able to access other private repo, even if Argo CD knows about them.

Read more about [private repos](private-repositories.md).