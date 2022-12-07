# Argo CD target clusters

By default an Argo CD instance can deploy applications to the same cluster it is installed on. However, Argo CD has the capability to connect and deploy to external clusters.

This means that there are several topologies for handling multiple clusters with Argo CD

1. Using one Argo CD instance per cluster
1. Using a centralized Argo CD instance that handles all cluster
1. A mixture of both strategies. For example you can have different groups of children clusters managed by multiple parent Argo CD instances

Let's see those options in turn:

## Installing Argo CD on each deployment cluster

Advantages

Clusters operate independently without reliance on an external instance of Argo CD
Security surface is limited to each individual cluster, (compromising one does not compromise the rest)
Low memory/cpu overhead for Argo CD with small impact on each cluster.
 

Disadvantages

Difficult to manage
Many instances often lead to many instances being out-of-date, introducing a new class of security problem
Poor visibility across organizations
Difficult to implement policy across many instances

## Using a central Argo CD installation

Advantages

Easy to maintain (one instance of Argo CD to manage)
Centralized control
One instance to worry about
Better visibility across organization
Disadvantages

Single attack surface
RBAC and SSO are not perfect (more on that later)
Target cluster API’s must be accessible to the central instance
Single point of failure, if this instance goes down, all clusters cannot be updated
No separation between staging and production management
Argo CD performance may degrade with many applications and clusters as they scale

## Hybrid approaches

## Which strategy to choose



