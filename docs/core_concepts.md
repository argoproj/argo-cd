# Core Concepts

Let's assume you're familiar with core Git, Docker, Kubernetes, Continuous Delivery, and GitOps concepts.

* **Application** A group of Kubernetes resources as defined by a manifest. This is a Custom Resource Definition (CRD).
* **Target state** The desired state of an application, as represented by files in a Git repository. 
* **Live state** The live state of that application. What pods etc are deployed.
* **Sync status** Whether or not the live state matches the target state.
* **Sync** The process of making an application move to its target state. E.g. by applying changes to a Kubernetes cluster. 
* **Sync operation status** Whether or not a sync succeeded.
* **Refresh** Checking to see if Git has any new changes than need to be applied. 
* **Health** The health the application, is it running correctly? Can it serve requests? 
* **Tool** A tool to create manifests from a directory of files.
