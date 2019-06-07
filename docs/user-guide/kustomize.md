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
