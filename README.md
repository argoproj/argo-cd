[![Integration tests](https://github.com/argoproj/argo-cd/workflows/Integration%20tests/badge.svg?branch=master)](https://github.com/argoproj/argo-cd/actions?query=workflow%3A%22Integration+tests%22)
[![slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)
[![codecov](https://codecov.io/gh/argoproj/argo-cd/branch/master/graph/badge.svg)](https://codecov.io/gh/argoproj/argo-cd)
[![Release Version](https://img.shields.io/github/v/release/argoproj/argo-cd?label=argo-cd)](https://github.com/argoproj/argo-cd/releases/latest)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4486/badge)](https://bestpractices.coreinfrastructure.org/projects/4486)

# Argo CD - Declarative Continuous Delivery for Kubernetes

## What is Argo CD?

Argo CD is a declarative, GitOps continuous delivery tool for Kubernetes.

![Argo CD UI](docs/assets/argocd-ui.gif)

[![Argo CD Demo](https://img.youtube.com/vi/0WAm0y2vLIo/0.jpg)](https://youtu.be/0WAm0y2vLIo)

## Why Argo CD?

1. Application definitions, configurations, and environments should be declarative and version controlled.
1. Application deployment and lifecycle management should be automated, auditable, and easy to understand.

## Who uses Argo CD?

[Official Argo CD user list](USERS.md)

## Documentation

To learn more about Argo CD [go to the complete documentation](https://argo-cd.readthedocs.io/).
Check live demo at https://cd.apps.argoproj.io/.

## Community

### Contribution, Discussion and Support

 You can reach the Argo CD community and developers via the following channels:

* Q & A : [Github Discussions](https://github.com/argoproj/argo-cd/discussions)
* Chat : [The #argo-cd Slack channel](https://argoproj.github.io/community/join-slack)
* Contributors Office Hours: [Every Thursday](https://calendar.google.com/calendar/u/0/embed?src=argoproj@gmail.com) | [Agenda](https://docs.google.com/document/d/1ttgw98MO45Dq7ZUHpIiOIEfbyeitKHNfMjbY5dLLMKQ)
* User Community meeting: [Every other Wednesday](https://calendar.google.com/calendar/u/0/embed?src=argoproj@gmail.com) | [Agenda](https://docs.google.com/document/d/1xkoFkVviB70YBzSEa4bDnu-rUZ1sIFtwKKG1Uw8XsY8)


Participation in the Argo CD project is governed by the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md)


### Blogs and Presentations

1. [Combining Argo CD (GitOps), Crossplane (Control Plane), And KubeVela (OAM)](https://youtu.be/eEcgn_gU3SM)
1. [How to Apply GitOps to Everything - Combining Argo CD and Crossplane](https://youtu.be/yrj4lmScKHQ)
1. [Couchbase - How To Run a Database Cluster in Kubernetes Using Argo CD](https://youtu.be/nkPoPaVzExY)
1. [Automation of Everything - How To Combine Argo Events, Workflows & Pipelines, CD, and Rollouts](https://youtu.be/XNXJtxkUKeY)
1. [Environments Based On Pull Requests (PRs): Using Argo CD To Apply GitOps Principles On Previews](https://youtu.be/cpAaI8p4R60)
1. [Argo CD: Applying GitOps Principles To Manage Production Environment In Kubernetes](https://youtu.be/vpWQeoaiRM4)
1. [Creating Temporary Preview Environments Based On Pull Requests With Argo CD And Codefresh](https://codefresh.io/continuous-deployment/creating-temporary-preview-environments-based-pull-requests-argo-cd-codefresh/)
1. [Tutorial: Everything You Need To Become A GitOps Ninja](https://www.youtube.com/watch?v=r50tRQjisxw) 90m tutorial on GitOps and Argo CD.
1. [Comparison of Argo CD, Spinnaker, Jenkins X, and Tekton](https://www.inovex.de/blog/spinnaker-vs-argo-cd-vs-tekton-vs-jenkins-x/)
1. [Simplify and Automate Deployments Using GitOps with IBM Multicloud Manager 3.1.2](https://www.ibm.com/cloud/blog/simplify-and-automate-deployments-using-gitops-with-ibm-multicloud-manager-3-1-2)
1. [GitOps for Kubeflow using Argo CD](https://v0-6.kubeflow.org/docs/use-cases/gitops-for-kubeflow/)
1. [GitOps Toolsets on Kubernetes with CircleCI and Argo CD](https://www.digitalocean.com/community/tutorials/webinar-series-gitops-tool-sets-on-kubernetes-with-circleci-and-argo-cd)
1. [CI/CD in Light Speed with K8s and Argo CD](https://www.youtube.com/watch?v=OdzH82VpMwI&feature=youtu.be)
1. [Machine Learning as Code](https://www.youtube.com/watch?v=VXrGp5er1ZE&t=0s&index=135&list=PLj6h78yzYM2PZf9eA7bhWnIh_mK1vyOfU). Among other things, describes how Kubeflow uses Argo CD to implement GitOPs for ML
1. [Argo CD - GitOps Continuous Delivery for Kubernetes](https://www.youtube.com/watch?v=aWDIQMbp1cc&feature=youtu.be&t=1m4s)
1. [Introduction to Argo CD : Kubernetes DevOps CI/CD](https://www.youtube.com/watch?v=2WSJF7d8dUg&feature=youtu.be)
1. [GitOps Deployment and Kubernetes - using Argo CD](https://medium.com/riskified-technology/gitops-deployment-and-kubernetes-f1ab289efa4b)
1. [Deploy Argo CD with Ingress and TLS in Three Steps: No YAML Yak Shaving Required](https://itnext.io/deploy-argo-cd-with-ingress-and-tls-in-three-steps-no-yaml-yak-shaving-required-bc536d401491)
1. [GitOps Continuous Delivery with Argo and Codefresh](https://codefresh.io/events/cncf-member-webinar-gitops-continuous-delivery-argo-codefresh/)
1. [Stay up to date with Argo CD and Renovate](https://mjpitz.com/blog/2020/12/03/renovate-your-gitops/)
1. [Setting up Argo CD with Helm](https://www.arthurkoziel.com/setting-up-argocd-with-helm/)
1. [Applied GitOps with Argo CD](https://thenewstack.io/applied-gitops-with-argocd/)
1. [Solving configuration drift using GitOps with Argo CD](https://www.cncf.io/blog/2020/12/17/solving-configuration-drift-using-gitops-with-argo-cd/)
1. [Decentralized GitOps over environments](https://blogs.sap.com/2021/05/06/decentralized-gitops-over-environments/)
