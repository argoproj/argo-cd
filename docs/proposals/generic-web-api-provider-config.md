---
title: generic-web-api-provider-config
authors:
  - "@redbackthomson"
sponsors:
  - Upbound
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2024-05-23
last-updated: yyyy-mm-dd
---

# Generic Web API Provider Configuration

## Summary

We'd like to support authentication to clusters through bearer tokens returned from a generic API call.

## Motivation

[Argo CD currently requires][current-docs] at least one username/password, bearer token, or exec provider config as authentication for clusters. None of these solutions provide a simple mechanism to retrieve short-lived credentials.

Username/password and bearer token methods query from long-lived credentials stored as secrets in the cluster. It is possible to rotate the credentials, but would require the use of an external controller to retrieve and swap the contents of the secret.

The exec provider config method is capable of retrieving short-lived credentials but relies on the user to mount an init container with the CLI tool so that Argo CD can call the executable from its `PATH`. While [documentation for this feature][exec-docs] is available, the process of adapting and deploying Argo with a custom container is complicated and error-prone, and becomes a large barrier to entry for some users.

Assuming a user simply needs to query an API endpoint to retrieve short-lived credentials, it would be far simpler experience for them to provide a configuration defining the endpoint, it's input parameters, and it's output format instead of having to jump through the many hoops required by the current solutions.

[current-docs]: https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters
[exec-docs]: https://argo-cd.readthedocs.io/en/stable/operator-manual/custom_tools/#byoi-build-your-own-image

### Goals

#### Define a new authentication configuration

Define a new authentication configuration, outlining the query parameters required to call an API that returns short-lived Kubernetes client credentials. 

#### Support the new configuration in Argo CD

Update all code paths in Argo CD that currently query for cluster authentication to support the new configuration.

### Non-Goals

#### Creating authentication APIs

The user will need to provide their own authentication API - one will not be created as part of Argo CD.

## Proposal

#### Authentication Configuration Structure

A new configuration definition called `webAPIProviderConfig` will be added to the [Cluster secret schema definition][current-docs]. `webAPIProviderConfig` will configure Argo CD to make an arbitrary request to any API and parse the corresponding JSON output to extract the Kubernetes client bearer token.

Requests will support the following parameters:
- URL
- HTTP request methods
- Query parameters
- Headers
- Body

For the initial implementation, we will assume that the response is in JSON. As such, extracting the token from the response body will be through a [JSONPath][json-path] string.

The proposed `webAPIProviderConfig` schema is:
```yaml!
webAPIProviderConfig:
  # (Required) The HTTP request method sent as part of the request
  method: string
  
  # (Required) The endpoint to which the request is sent. This field is a Golang template that has access to cluster secret values.
  url: string
  
  # (Required) A JSONPath string that points to the location of the bearer token returned by a 2XX response from the API.
  tokenPath: string
  
  # (Optional) A Golang template which is evaluated to form the body of the request.
  body: string
  
  # (Optional) A dictionary of header key-value pairs. The value of the header is a Golang template.
  headers:
    "header-key": string
```

[json-path]: https://github.com/json-path/JsonPath

### Use cases

#### Use case 1:

As a user, I would like to point my Argo CD installation at a cluster in a managed Kubernetes service that requires short-lived credentials for authentication.

### Implementation Details/Notes/Constraints

The proposed schema does not currently support any configuration of token caching. It is possible that some users would expect a new request to be made for every connection to the cluster, while others would hope that credentials are cached for some amount of time so as to not hit rate limits. Since bearer tokens are not standardised across cluster providers, it's not necessarily possible to determine the expiration from reading the token itself. A design for handling token expiration and caching, including its own security concerns, will need to be done separately.

### Detailed examples

#### Example 1: Authenticating to Upbound

[Upbound][upbound] uses 60 minute short-lived tokens to authenticate users to access their managed control planes using their robot tokens (API keys). In order to exchange their robot token, users must make a POST request to the `/orgscopedtokens` API passing a form body that includes the name of the Upbound organization to authenticate against and the robot token. If successful, this API returns a JSON payload which includes a string `access_token` to be used as the bearer token when communicating with the Upbound control plane.

```yaml!
webAPIProviderConfig:
  method: "POST"
  url: https://auth.upbound.io/apis/tokenexchange.upbound.io/v1alpha1/orgscopedtokens
  # this example uses ${.orgName} and {.token}, which are provided by the user as part of the secret
  body: "audience=upbound%3Aspaces%3Aapi&audience=upbound%3Aspaces%3Acontrolplanes&grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange&scope=upbound%3Aorg%3A${.orgName}&subject_token={.token}&subject_token_type=urn%3Aietf%3Aparams%3Aoauth%3Atoken-type%3Aid_token"
  headers:
    "Content-Type": "application/x-www-form-urlencoded"
  tokenPath: $.access_token
```

[upbound]: https://www.upbound.io/

### Security Considerations

This proposal suggests that Argo CD make calls to arbitrary endpoints. Ideally the user would be able to validate the security of the endpoint independently before configuring Argo CD to interact with it.

Argo CD will require the use of TLS in its requests to the external APIs to protect against man-in-the-middle attacks that could compromise users' credentials during transmission.

Argo CD will only store the credentials returned by the APIs in memory and will never persist them to a volume or secret that may be accessible by other users.

### Risks and Mitigations



### Upgrade / Downgrade Strategy

No action is needed by the user to upgrade to the latest version of Argo CD that supports generic web API provider configurations. The feature is entirely opt-in and users that do not choose to opt-in will see no change.

When downgrading, users that have opted-in to using the configuration will not be able to access clusters that are using it to authenticate. Users will either need to revert to long-lived credentials or will need to find workaround solutions to authenticating these clusters using short-lived credentials (such as the ones mentioned in the "Motivation" section).

## Drawbacks

Asking users to provide the parameters for an authentication request is a poor user experience. Users may not be aware of the authentication APIs for their cluster providers and therefore might not know what values are valid for their Argo CD cluster. This poor experience may be offset by documentation, provided either by Argo and/or by the cluster providers, with examples of how to configure the requests.

## Alternatives

Each separate external Kubernetes cluster provider could have their own authentication configuration. All of these separate configurations would have their own, bespoke, inputs catering to the requirements of the provider. The user experience would be far simpler for any user of these cluster providers since they wouldn't need to know the specific endpoints, headers and body of the authentication APIs.

However as the number of Kubernetes cluster providers grows, so too would the list of authentication configurations. Each of these configurations would need to be handwritten and programmed specifically for the service's APIs. If any API changes, a new version of Argo CD would need to be published to keep compatibility.