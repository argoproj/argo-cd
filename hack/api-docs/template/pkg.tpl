{{ define "packages" }}

# Declarative API Reference

This page documents the declarative configuration of Argo CD.

> [!WARNING]
> The internal Go struct representations of resources (such as Clusters or Repositories) can differ from their serialized representations in Kubernetes Secrets. Please refer to the specific field descriptions below for details on how each field is mapped, formatted, and stored in Secrets.

## CRD-backed Resources

The following Custom Resource Definitions (CRDs) define Argo CD resources:

<ul>
  <li><a href="#argoproj.io/v1alpha1.Application">Application</a></li>
  <li><a href="#argoproj.io/v1alpha1.ApplicationSet">ApplicationSet</a></li>
  <li><a href="#argoproj.io/v1alpha1.AppProject">AppProject</a></li>
</ul>

## Secret-backed Configuration

The following configurations are stored as Kubernetes Secrets:

<ul>
  <li><a href="#argoproj.io/v1alpha1.Cluster">Cluster</a></li>
  <li><a href="#argoproj.io/v1alpha1.Repository">Repository</a></li>
  <li><a href="#argoproj.io/v1alpha1.RepoCreds">RepoCreds (Repository Credentials)</a></li>
</ul>

<hr/>

{{ range .packages }}
    {{ range (visibleTypes (sortedTypes .Types))}}
        {{ template "type" .  }}
    {{ end }}
    <hr/>
{{ end }}

<p><em>
    Generated with <code>gen-crd-api-reference-docs</code>.
</em></p>

{{ end }}
