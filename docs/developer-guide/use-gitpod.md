# Use Gitpod

[Gitpod](https://www.gitpod.io/) is an open-source platform for automated and ready-to-code development environments.
GitPod is probably the easiest way to get ready to use development environment with the most tools that are required
for Argo CD development.

## How To Use It

1. Fork [https://github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd) repository
1. Create Gitpod workspace by opening the following url in the browser:
   `https://gitpod.io/#https://github.com/<USERNAME>/argo-cd` where
   `<USERNAME>` is your GitHub username.

1. Once workspace is created you should see VSCode editor in the browser as well as workspace initialization
   logs in the VSCode terminal. The initialization process downloads all backend and UI dependencies as well
   as starts K8S control plane powered by Kubebuilder [envtest](https://book.kubebuilder.io/reference/envtest.html).
   Please wait until you see `Kubeconfig is available at /tmp/kubeconfig` message:

   ![image](https://user-images.githubusercontent.com/426437/113638085-e46be080-962a-11eb-943b-24c29171fb2b.png)

1. You are ready to go!

Once your workspace is ready you can use VS Code to make code changes. Run `goreman start` to start Argo CD components
and test your changes. Use the Gitpod user interface or [CLI](https://www.gitpod.io/docs/command-line-interface/) to
access Argo CD API/UI from your laptop.

## Why/When To Use It?

Gitpod is a perfect tool in following cases:

* you are a first-time contributor and eager to start coding;
* you are traveling and don't want to setup development tools on your laptop;
* you want to review pull request and need to quickly run code from the PR without changing your local setup;

## Limitations

There are some known limitations:

* You can only use VS Code
* Free plan provides 50 hours per month
* [Envtest](https://book.kubebuilder.io/reference/envtest.html) based Kubernetes is only control plane.
  So you won't be able to deploy Argo CD applications that runs actual pods.
* Codegen tools are not available. E.g. you won't be able to use `make codegen-local`.
