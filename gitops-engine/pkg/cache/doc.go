/*
Package cache implements lightweight Kubernetes cluster caching that stores only resource references and ownership
references. In addition to references cache might be configured to store custom metadata and whole body of selected
resources.

The library uses Kubernetes watch API to maintain cache up to date. This approach reduces number of Kubernetes
API requests and provides instant access to the required Kubernetes resources.
*/
package cache
