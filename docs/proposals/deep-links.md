---
title: Deep Links

authors:
- "@gdsoumya"
- "@alexmt"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-08-01

---

# Deep Links

Deep links allow end users to quickly redirect to third-party systems such as Splunk, DataDog etc. from the Argo CD
user interface.


## Summary

Argo CD administrator will be able to configure links that redirect users to third-party systems such as Splunk,
DataDog etc. The template should be able to reference different types of resources relating to where the links show up,
this includes projects, applications, or individual resources(pods, services etc.) that are part of the application.

Deep Link is a generic integration solution for third-party systems which enables users to integrate any system -  not
only popular solutions but also custom/private systems that can leverage the data available in Argo CD.

## Motivation

Argo CD UI with deep links to third-party integrations will provide a unified solution for users making it easier for
them to switch between relevant systems without having to separately navigate and correlate the information.


## Proposal

The configuration for Deep Links will be present in the `argocd-cm`, we will add new `<location>.links` fields in the
cm to list all the deep links that will be displayed in the provided location. The possible values for `<location>`
currently are :
- `project` : all links under this field will show up in the project tab in the Argo CD UI.
- `application` : all links under this field will show up in the application summary tab.
- `resource` : all links under this field will show up in the individual resource(deployments, pods, services etc.)
  summary tab.

Each link in the list has five subfields :
1. `title` : title/tag that will be displayed in the UI corresponding to that link
2. `url` : the actual URL where the deep link will redirect to, this field can be templated to use data from the
   application, project or resource objects (depending on where it is located)
3. `description` (optional) : a description for what the deep link is about
4. `icon.class` (optional) : a font-awesome icon class to be used when displaying the links in dropdown menus.
5. `if` (optional) : a conditional statement that results in either `true` or `false`, it also has access to the same
   data as the `url` field. If the condition resolves to `true` the deep link will be displayed else it will be hidden. If
   the field is omitted by default the deep links will be displayed.


An example `argocd-cm.yaml` file with deep links and its variations :

```yaml
data:
  # project level links
  project.links: |
    - url: https://myaudit-system.com?project={{proj.metadata.name}}
      title: Audit
  # application level links
  application.links: |
    - url: https://mycompany.splunk.com?search={{app.spec.destination.namespace}}
      title: Splunk
    # conditionally show link e.g. for specific project
    - url: https://mycompany.splunk.com?search={{app.spec.destination.namespace}}
      title: Splunk
      if: app.spec.proj == "abc"
    - url: https://{{project.metadata.annotations.splunkhost}}?search={{app.spec.destination.namespace}}
      title: Splunk
      if: project.metadata.annotations.splunkhost
    
  # resource level links
  resource.links: |
    - url: https://mycompany.splunk.com?search={{res.metadata.namespace}}
      title: Splunk
      if: res.kind == "Pod" || res.kind == "Deployment"

```

The argocd server will expose new APIs for rendering deep links in the UI, the server will handle the templating and
conditional rendering logic and will provide the UI with the final list of links that need to be displayed for a
particular location/resource.

The following API methods are proposed:

```protobuf
message LinkInfo {
  required string name = 1;
  required string url = 2;
  optional string description = 3;
  optional string iconClass = 4;
}

message LinksResponse {
  repeated LinkInfo items = 1;
}

service ApplicationService {
  rpc ListLinks(google.protobuf.Empty) returns (LinksResponse) {
    option (google.api.http).get = "/api/v1/applications/{name}/links";
  }

  rpc ListResourceLinks(ApplicationResourceRequest) returns (LinksResponse) {
    option (google.api.http).get = "/api/v1/applications/{name}/resource/links";
  }
}

service ProjectService {
  
  rpc ListLinks(google.protobuf.Empty) returns (LinksResponse) {
    option (google.api.http).get = "/api/v1/projects/{name}/links";
  }
}
```

### Use cases

Some example use cases this enhancement intends to take care of -

#### Use case 1:
As a user, I would like to quickly open a splunk/datadog UI with a query that retrieves all logs of application
namespace or metrics for specific applications etc.
