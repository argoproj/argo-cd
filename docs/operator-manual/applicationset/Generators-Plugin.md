# Plugin Generator

Plugins allow you to provide your own generator.

- You can write in any language
- Simple: a plugin just responds to RPC HTTP requests.
- You can use it in a sidecar, or standalone deployment.
- You can get your plugin running today, no need to wait 3-5 months for review, approval, merge and an Argo software
  release.
- You can combine it with Matrix or Merge.

To start working on your own plugin, you can generate a new repository based on the example
[applicationset-hello-plugin](https://github.com/argoproj-labs/applicationset-hello-plugin).

## Simple example

Using a generator plugin without combining it with Matrix or Merge.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myplugin
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    - plugin:
        # Specify the configMap where the plugin configuration is located.
        configMapRef:
          name: my-plugin
        # You can pass arbitrary parameters to the plugin. `input.parameters` is a map, but values may be any type.
        # These parameters will also be available on the generator's output under the `generator.input.parameters` key.
        input:
          parameters:
            key1: "value1"
            key2: "value2"
            list: ["list", "of", "values"]
            boolean: true
            map:
              key1: "value1"
              key2: "value2"
              key3: "value3"

        # You can also attach arbitrary values to the generator's output under the `values` key. These values will be
        # available in templates under the `values` key.
        values:
          value1: something

        # When using a Plugin generator, the ApplicationSet controller polls every `requeueAfterSeconds` interval (defaulting to every 30 minutes) to detect changes.
        requeueAfterSeconds: 30
  template:
    metadata:
      name: myplugin
      annotations:
        example.from.input.parameters: "{{ index .generator.input.parameters.map "key1" }}"
        example.from.values: "{{ .values.value1 }}"
        # The plugin determines what else it produces.
        example.from.plugin.output: "{{ .something.from.the.plugin }}"
```

- `configMapRef.name`: A `ConfigMap` name containing the plugin configuration to use for RPC call.
- `input.parameters`: Input parameters included in the RPC call to the plugin. (Optional)

!!! note
    The concept of the plugin should not undermine the spirit of GitOps by externalizing data outside of Git. The goal is to be complementary in specific contexts.
    For example, when using one of the PullRequest generators, it's impossible to retrieve parameters related to the CI (only the commit hash is available), which limits the possibilities. By using a plugin, it's possible to retrieve the necessary parameters from a separate data source and use them to extend the functionality of the generator.

### Add a ConfigMap to configure the access of the plugin

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-plugin
  namespace: argocd
data:
  token: "$plugin.myplugin.token" # Alternatively $<some_K8S_secret>:plugin.myplugin.token
  baseUrl: "http://myplugin.plugin-ns.svc.cluster.local."
  requestTimeout: "60"
```

- `token`: Pre-shared token used to authenticate HTTP request (points to the right key you created in the `argocd-secret` Secret)
- `baseUrl`: BaseUrl of the k8s service exposing your plugin in the cluster.
- `requestTimeout`: Timeout of the request to the plugin in seconds (default: 30)

### Store credentials

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-secret
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-secret
    app.kubernetes.io/part-of: argocd
type: Opaque
data:
  # ...
  # The secret value must be base64 encoded **once**.
  # this value corresponds to: `printf "strong-password" | base64`.
  plugin.myplugin.token: "c3Ryb25nLXBhc3N3b3Jk"
  # ...
```

#### Alternative

If you want to store sensitive data in **another** Kubernetes `Secret`, instead of `argocd-secret`, ArgoCD knows how to check the keys under `data` in your Kubernetes `Secret` for a corresponding key whenever a value in a configmap starts with `$`, then your Kubernetes `Secret` name and `:` (colon) followed by the key name.

Syntax: `$<k8s_secret_name>:<a_key_in_that_k8s_secret>`

> NOTE: Secret must have label `app.kubernetes.io/part-of: argocd`

##### Example

`another-secret`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: another-secret
  namespace: argocd
  labels:
    app.kubernetes.io/part-of: argocd
type: Opaque
data:
  # ...
  # Store client secret like below.
  # The secret value must be base64 encoded **once**.
  # This value corresponds to: `printf "strong-password" | base64`.
  plugin.myplugin.token: "c3Ryb25nLXBhc3N3b3Jk"
```

### HTTP server

#### A Simple Python Plugin

You can deploy it either as a sidecar or as a standalone deployment (the latter is recommended).

In the example, the token is stored in a file at this location : `/var/run/argo/token`

```
strong-password
```

```python
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

with open("/var/run/argo/token") as f:
    plugin_token = f.read().strip()


class Plugin(BaseHTTPRequestHandler):

    def args(self):
        return json.loads(self.rfile.read(int(self.headers.get('Content-Length'))))

    def reply(self, reply):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(json.dumps(reply).encode("UTF-8"))

    def forbidden(self):
        self.send_response(403)
        self.end_headers()

    def unsupported(self):
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.headers.get("Authorization") != "Bearer " + plugin_token:
            self.forbidden()

        if self.path == '/api/v1/getparams.execute':
            args = self.args()
            self.reply({
                "output": {
                    "parameters": [
                        {
                            "key1": "val1",
                            "key2": "val2"
                        },
                        {
                            "key1": "val2",
                            "key2": "val2"
                        }
                    ]
                }
            })
        else:
            self.unsupported()


if __name__ == '__main__':
    httpd = HTTPServer(('', 4355), Plugin)
    httpd.serve_forever()
```

Execute getparams with curl :

```
curl http://localhost:4355/api/v1/getparams.execute -H "Authorization: Bearer strong-password" -d \
'{
  "applicationSetName": "fake-appset",
  "input": {
    "parameters": {
      "param1": "value1"
    }
  }
}'
```

Some things to note here:

- You only need to implement the calls `/api/v1/getparams.execute`
- You should check that the `Authorization` header contains the same bearer value as `/var/run/argo/token`. Return 403 if not
- The input parameters are included in the request body and can be accessed using the `input.parameters` variable.
- The output must always be a list of object maps nested under the `output.parameters` key in a map.
- `generator.input.parameters` and `values` are reserved keys. If present in the plugin output, these keys will be overwritten by the
  contents of the `input.parameters` and `values` keys in the ApplicationSet's plugin generator spec.

## With matrix and pull request example

In the following example, the plugin implementation is returning a set of image digests for the given branch. The returned list contains only one item corresponding to the latest built image for the branch.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: fb-matrix
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    - matrix:
        generators:
          - pullRequest:
              github: ...
              requeueAfterSeconds: 30
          - plugin:
              configMapRef:
                name: cm-plugin
              input:
                parameters:
                  branch: "{{.branch}}" # provided by generator pull request
              values:
                branchLink: "https://git.example.com/org/repo/tree/{{.branch}}"
  template:
    metadata:
      name: "fb-matrix-{{.branch}}"
    spec:
      source:
        repoURL: "https://github.com/myorg/myrepo.git"
        targetRevision: "HEAD"
        path: charts/my-chart
        helm:
          releaseName: fb-matrix-{{.branch}}
          valueFiles:
            - values.yaml
          values: |
            front:
              image: myregistry:{{.branch}}@{{ .digestFront }} # digestFront is generated by the plugin
            back:
              image: myregistry:{{.branch}}@{{ .digestBack }} # digestBack is generated by the plugin
      project: default
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
      destination:
        server: https://kubernetes.default.svc
        namespace: "{{.branch}}"
      info:
        - name: Link to the Application's branch
          value: "{{values.branchLink}}"
```

To illustrate :

- The generator pullRequest would return, for example, 2 branches: `feature-branch-1` and `feature-branch-2`.

- The generator plugin would then perform 2 requests as follows :

```shell
curl http://localhost:4355/api/v1/getparams.execute -H "Authorization: Bearer strong-password" -d \
'{
  "applicationSetName": "fb-matrix",
  "input": {
    "parameters": {
      "branch": "feature-branch-1"
    }
  }
}'
```

Then,

```shell
curl http://localhost:4355/api/v1/getparams.execute -H "Authorization: Bearer strong-password" -d \
'{
  "applicationSetName": "fb-matrix",
  "input": {
    "parameters": {
      "branch": "feature-branch-2"
    }
  }
}'
```

For each call, it would return a unique result such as :

```json
{
  "output": {
    "parameters": [
      {
        "digestFront": "sha256:a3f18c17771cc1051b790b453a0217b585723b37f14b413ad7c5b12d4534d411",
        "digestBack": "sha256:4411417d614d5b1b479933b7420079671facd434fd42db196dc1f4cc55ba13ce"
      }
    ]
  }
}
```

Then,

```json
{
  "output": {
    "parameters": [
      {
        "digestFront": "sha256:7c20b927946805124f67a0cb8848a8fb1344d16b4d0425d63aaa3f2427c20497",
        "digestBack": "sha256:e55e7e40700bbab9e542aba56c593cb87d680cefdfba3dd2ab9cfcb27ec384c2"
      }
    ]
  }
}
```

In this example, by combining the two, you ensure that one or more pull requests are available and that the generated tag has been properly generated. This wouldn't have been possible with just a commit hash because a hash alone does not certify the success of the build.
