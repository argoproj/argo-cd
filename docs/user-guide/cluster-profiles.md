# Cluster Profiles

The ApplicationSet controller includes a ClusterProfile controller that automates the creation of cluster secrets from ClusterProfile objects. This simplifies the management of cluster representation using the [Cluster Inventory API](https://github.com/kubernetes-sigs/cluster-inventory-api).

## How it works

TODO: finish docs

TL;DR secret syncer checks for clusterprofiles in the namespace, creates and sycns corresponding secrets, name and server and CA data from the ClusterProfile, execConfig from clusterprofile providers file

A finalizer (`argoproj.io/cluster-profile-finalizer`) is added to the `ClusterProfile` object so the controller will clean up the corresponding `Secret`.