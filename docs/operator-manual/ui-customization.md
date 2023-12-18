# UI Customization

## Default Application Details View

By default, the Application Details will show the `Tree` view.

This can be configured on an Application basis, by setting the `pref.argocd.argoproj.io/default-view` annotation, accepting one of: `tree`, `pods`, `network`, `list` as values. A summary view is also available to brief the overall report of the applications.
![image](https://github.com/argoproj/argo-cd/assets/57581240/a8ed959c-c387-46ac-a05e-d194c6a73321)
For the Pods view, the default grouping mechanism can be configured using the `pref.argocd.argoproj.io/default-pod-sort` annotation, accepting one of: `node`, `parentResource`, `topLevelResource` as values.
