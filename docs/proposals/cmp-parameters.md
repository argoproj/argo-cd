---
title: Config-Management-Plugin-Parameters

authors:
- "@alexmt"
- "@crenshaw-dev"
- "@leoluz"

sponsors:
- TBD

reviewers:
- TBD

approvers:
- TBD

creation-date: 2022-01-05

last-updated: 2022-01-05

---

# Config Management Plugin Parameters

CMP Parameters defines a way for plugins to "announce" and then consume acceptable parameters for an Application.
Announcing parameters allows CMPs to provide a UI experience on par with native config management tools 
(Helm, Kustomize, etc.).

## Open Questions [optional]

This is where to call out areas of the design that require closure before deciding to implement the
design.

## Summary

Config Management Plugins allow Argo CD administrators to define custom manifest generation tooling.

The only existing way for users to parameterize manifest generation is with environment variables.

This new feature will allow a plugin to "announce" acceptable parameters for an Application. It will also allow the
plugin to consume parameters once the user has defined them.

Parameters definitions may be simple (advertising a simple key/value string pair) or rich (advertising more information 
about the expected value). An "image" would be an example of a rich parameter description. The plugin would announce 
that it expects an image parameter, and the UI would build the appropriate input.

## Motivation

### 1. CMPs are under-utilized

CMPs, especially the sidecar type, are under-utilized. Making them more robust will increase adoption. Increased
adoption will help us find bugs and then make CMPs more robust. In other words, we need to reach a critical mass of 
CMP users.

More robust CMPs will make it easier to start supporting tools like [Tanka](https://tanka.dev/).

### 2. Decisions about config management tools are limited by the core code

For example, there's a [Helm bug](https://github.com/argoproj/argo-cd/issues/7291) affecting Argo CD users. The fix 
would involve importing the Helm SDK (a very large dependency) into Argo CD. 

### 3. Ksonnet is deprecated, and CMPs are a good place to maintain support

Offloading Ksonnet to a plugin would allow us to support existing users without maintaining Ksonnet code in the more
actively-developed base. But we need CMP parameters to provide Ksonnet support on-par with native support.

### Goals

Parameterized CMPs must be:
* Easy to write
  * An Argo CD admin should be able to write a simple parameterized CMP in just a few lines of code.
  * An Argo CD admin should be able to write an _advanced_ parameterized CMP server relying on thorough docs.
    
    Writing a custom CMP server might be preferable if the parameters announcement code gets too complex to be 
    an inline shell script.
* Easy to install
  * Installing a simple CMP or even a CMP with a custom server should be intuitive and painless.
* Easy to use
  * Argo CD end-users (for example, developers) should be able to
    1. View and set parameters in the Argo CD Application UI
    2. See the parameters reflected in the Application manifest
    3. Easily read/modify the generated parameters in the manifest (they should be structured in a way that's easy to read)
  * CMPs should be able to announce parameters with more helpful interfaces than a simple text field.
    * For example, image parameters should be presented using the same helpful interface as the one in Kustomize applications.
* Future-proof
  * Since the rich parameters UI is an important feature for config management tools, the parameter definition schema 
    should be flexible enough to announce new _types_ of parameters so the UI can customize its presentation.
* Backwards-compatible
  * CMPs written before this enhancement should work fine after this enhancement is released.
  * The UI should be able to handle unknown (new) parameter types. For example, if a plugin announces a parameter of 
    type `date`, the UI should fall back to allowing text entry. The UI can then be enhanced to provide a better input
    mechanism in a later release.
* Proven with a rich demonstration
  * The initial release of this feature should include a CMP implementation of the Helm config tool. This will
    1) Serve as a rich example for others CMP developers to mimic
    2) Allow us to decouple the Helm config management release cycle from the Argo release cycle
    3) Allow us to work around [this bug](https://github.com/argoproj/argo-cd/issues/7291) without including the Helm 
       SDK in the core Argo CD code

### Non-Goals

We should not:
* Re-implement config management tools as CMPs (besides Helm)

## Proposal

### Use cases

#### Use case 1:

As an Argo CD developer, I would like to be able to build Argo CD without including the Helm SDK as a dependency.

The Helm SDK includes the Kubernetes code base. That's a lot of code, and it will make builds unacceptably slow.

#### Use case 2:

As an Argo CD user, I would like to be able to parameterize manifests built by a CMP.

For example, if the Argo CD administrator has installed a CMP which applies a last-mile kustomize overlay to a Helm
repo, I would like to be able to pass values to the Helm chart without having to manually discover those parameter names
(in other words, they should show up in the Application UI just like with a native Helm Application). I also shouldn't 
have to ask my Argo CD admin to modify the CMP to accommodate the values as environment variables.

### Implementation Details/Notes/Constraints

#### Terms

* **Parameter definition**: an instance of a data structure which describes an individual parameter that may be applied
  to a specific Application. (See the [schema](#parameter-definition-schema) below.)
* **Parameters announcement**: a list of parameter definitions. (See the [schema](#parameters-announcement-schema) below.)

  "Parameters" is plural because each "announcement" will be a list of multiple parameter definitions.
* **Parameterized CMP**: a CMP which supports rich parameters (i.e. more than environment variables). A CMP is
  parameterized if either of these is true:
  1. its configuration includes the sections consumed by the default CMP server to generate parameters announcements
  2. it is a fully customized CMP server which implements an endpoint to generate parameters announcements

#### Parameters announcement / parameters serialization format

Parameters announcements should be produced by the CMP as JSON. Use JSON instead of YAML because the tooling is better
(native JSON libraries, StackOverflow answers about jq, etc.).

Parameters should be set in the manifest as a map of section names to parameter name/value pairs. YAML is used because
it's easy to read/manipulate in an editor when modifying an Application manifest. We partition by section name so that
the manifest, to the extent possible, is laid out similarly to the UI.

```yaml
plugin:
  parameters:
    main:
      - name: values-files
        value: '["values.yaml"]'
    Helm Parameters:
      - name: image
        values: some.repo:tag
```

**Note:** I'm not sure whether CRDs allow Map<string, list> types. If not, we should consider flattening the schema to
a list of objects, each object having a `section` field.

Parameters should be communicated _to_ the CMP as JSON in the same schema as is used in the Application manifest.
JSON might be a surprising choice considering parameters are represented in the manifest as YAML. But I think JSON makes 
sense because 1) it's used for parameters announcements (consistency is good) and 2) JSON tooling is better.

#### Parameter definition schema

A parameter definition is an object with following schema:

```go
type ParameterDefinition struct {
	// Name is the name of a parameter. (required)
	Name string `json:"name"`
	// Type is the type of the parameter. This determines the schema of `uiConfig` and how the UI presents the 
	// parameter. (default is `string)
	Type string `json:"type"`
	// UiConfig is a stringified JSON object containing information about how the UI should present the parameter. 
	// (default is `{}`)
	UiConfig string `json:"uiConfig"`
	// Section is the name of the group of parameters in which this parameter belongs. `main` parameters will be 
	// presented at the top of the UI. Other parameters will be grouped by section, and the sections will be 
	// displayed in alphabetical order after the main section.
	Section string `json:"section"`
}
```

#### Parameters announcement schema

```go
type ParametersAnnouncement []ParameterDefinition
```

Example:

```json
[
  {
    "name": "values-files",
    "type": "enum",
    "uiConfig": "{\"values\": [\"values.yaml\"]}"
  },
  {
    "name": "image",
    "type": "image",
    "section": "Helm Parameters"
  }
]
```

#### Parameter list schema

The top level is a JSON object. Each key is the name of a parameter "section". Each value of the top-level JSON object
is a JSON list of objects representing parameter values. Each parameter value has the following schema:

```go
type Parameter struct {
	// Name is the name of the parameter.
  Name string `json:"name"`
  // Value is the value of the parameter. It's up to the CMP to interpret this value. It could be interpreted as a 
  // simple string, or it could be some encoding of something more complex (like a JSON object).
  Value string `json:"value"`
}
```

Example:

```json
{
  "main": [
    {"name": "values-files", "value": "values.yaml"}
  ],
  "Helm Parameters": [
    {"name": "image", "value": "some.repo:tag"}
  ]
}
```

When the CMP receives parameters, they should be in JSON. But the parameters should be represented as YAML in the 
Application manifest for better readability.

### Detailed examples

#### Example 1: trivial parameterized CMP

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: trivial-cmp
spec:
  version: v1.0
  generate:
    command: 
      - sh
      - -c
      - |
        # Pull one parameter value from the "main" section of the given parameters.
        CM_NAME_SUFFIX=$(echo "$ARGOCD_PARAMETERS" | jq -r '.["main"][] | select(.name == "cm-name-suffix").value')
        cat << EOM
        {
          "kind": "ConfigMap",
          "apiVersion": "v1",
          "metadata": {
            "name": "$ARGOCD_APP_NAME-$CM_NAME_SUFFIX",
            "namespace": "$ARGOCD_APP_NAMESPACE"
          }
        }
        EOM
  discover:
    fileName: "./trivial-cmp"
  parameters:
    command:
      - sh
      - -c
      - |
        echo '[{"name": "cm-name-suffix"}]'
```

### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

### Risks and Mitigations

What are the risks of this proposal and how do we mitigate. Think broadly.

For example, consider
both security and how this will impact the larger Kubernetes ecosystem.

Consider including folks that also work outside your immediate sub-project.


### Upgrade / Downgrade Strategy

If applicable, how will the component be upgraded and downgraded? Make sure this is in the test
plan.

Consider the following in developing an upgrade/downgrade strategy for this enhancement:

- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to keep previous behavior?
- What changes (in invocations, configurations, API use, etc.) is an existing cluster required to
  make on upgrade in order to make use of the enhancement?

## Drawbacks

The idea is to find the best form of an argument why this enhancement should _not_ be implemented.

## Alternatives

Similar to the `Drawbacks` section the `Alternatives` section is used to highlight and record other
possible approaches to delivering the value proposed by an enhancement.