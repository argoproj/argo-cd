package ujson

import "testing"

func TestKube(t *testing.T) {
	input := []byte(`{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind": "Application",
		"metadata": {
		   "name": "guestbook",
		   "namespace": "argocd",
		   "finalizers": [
			  "resources-finalizer.argocd.argoproj.io"
		   ],
		   "labels": {
			  "name": "guestbook"
		   }
		},
		"spec": {
		   "project": "default",
		   "source": {
			  "repoURL": "https://github.com/argoproj/argocd-example-apps.git",
			  "targetRevision": "HEAD",
			  "path": "guestbook",
			  "chart": "chart-name",
			  "helm": {
				 "passCredentials": false,
				 "parameters": [
					{
					   "name": "nginx-ingress.controller.service.annotations.external-dns\\.alpha\\.kubernetes\\.io/hostname",
					   "value": "mydomain.example.com"
					},
					{
					   "name": "ingress.annotations.kubernetes\\.io/tls-acme",
					   "value": "true",
					   "forceString": true
					}
				 ],
				 "fileParameters": [
					{
					   "name": "config",
					   "path": "files/config.json"
					}
				 ],
				 "releaseName": "guestbook",
				 "valueFiles": [
					"values-prod.yaml"
				 ],
				 "ignoreMissingValueFiles": false,
				 "values": "ingress:\n  enabled: true\n  path: /\n  hosts:\n    - mydomain.example.com\n  annotations:\n    kubernetes.io/ingress.class: nginx\n    kubernetes.io/tls-acme: \"true\"\n  labels: {}\n  tls:\n    - secretName: mydomain-tls\n      hosts:\n        - mydomain.example.com\n",
				 "valuesObject": {
					"ingress": {
					   "enabled": true,
					   "path": "/",
					   "hosts": [
						  "mydomain.example.com"
					   ],
					   "annotations": {
						  "kubernetes.io/ingress.class": "nginx",
						  "kubernetes.io/tls-acme": "true"
					   },
					   "labels": {},
					   "tls": [
						  {
							 "secretName": "mydomain-tls",
							 "hosts": [
								"mydomain.example.com"
							 ]
						  }
					   ]
					}
				 },
				 "skipCrds": false,
				 "version": "v2"
			  },
			  "kustomize": {
				 "version": "v3.5.4",
				 "namePrefix": "prod-",
				 "nameSuffix": "-some-suffix",
				 "commonLabels": {
					"foo": "bar"
				 },
				 "commonAnnotations": {
					"beep": "boop-${ARGOCD_APP_REVISION}"
				 },
				 "commonAnnotationsEnvsubst": true,
				 "images": [
					"gcr.io/heptio-images/ks-guestbook-demo:0.2",
					"my-app=gcr.io/my-repo/my-app:0.1"
				 ],
				 "namespace": "custom-namespace",
				 "replicas": [
					{
					   "name": "kustomize-guestbook-ui",
					   "count": 4
					}
				 ]
			  },
			  "directory": {
				 "recurse": true,
				 "jsonnet": {
					"extVars": [
					   {
						  "name": "foo",
						  "value": "bar"
					   },
					   {
						  "code": true,
						  "name": "baz",
						  "value": "true"
					   }
					],
					"tlas": [
					   {
						  "code": false,
						  "name": "foo",
						  "value": "bar"
					   }
					]
				 },
				 "exclude": "config.yaml",
				 "include": "*.yaml"
			  },
			  "plugin": {
				 "name": "mypluginname",
				 "env": [
					{
					   "name": "FOO",
					   "value": "bar"
					}
				 ],
				 "parameters": [
					{
					   "name": "string-param",
					   "string": "example-string"
					},
					{
					   "name": "array-param",
					   "array": [
						  "item1",
						  "item2"
					   ]
					},
					{
					   "name": "map-param",
					   "map": {
						  "param-name": "param-value"
					   }
					}
				 ]
			  }
		   },
		   "sources": [
			  {
				 "repoURL": "https://github.com/argoproj/argocd-example-apps.git",
				 "targetRevision": "HEAD",
				 "path": "guestbook",
				 "ref": "my-repo"
			  }
		   ],
		   "destination": {
			  "server": "https://kubernetes.default.svc",
			  "namespace": "guestbook"
		   },
		   "info": [
			  {
				 "name": "Example:",
				 "value": "https://example.com"
			  }
		   ],
		   "syncPolicy": {
			  "automated": {
				 "prune": true,
				 "selfHeal": true,
				 "allowEmpty": false
			  },
			  "syncOptions": [
				 "Validate=false",
				 "CreateNamespace=true",
				 "PrunePropagationPolicy=foreground",
				 "PruneLast=true",
				 "RespectIgnoreDifferences=true",
				 "ApplyOutOfSyncOnly=true"
			  ],
			  "managedNamespaceMetadata": {
				 "labels": {
					"any": "label",
					"you": "like"
				 },
				 "annotations": {
					"the": "same",
					"applies": "for",
					"annotations": "on-the-namespace"
				 }
			  },
			  "retry": {
				 "limit": 5,
				 "backoff": {
					"duration": "5s",
					"factor": 2,
					"maxDuration": "3m"
				 }
			  }
		   },
		   "ignoreDifferences": [
			  {
				 "group": "apps",
				 "kind": "Deployment",
				 "jsonPointers": [
					"/spec/replicas"
				 ]
			  },
			  {
				 "kind": "ConfigMap",
				 "jqPathExpressions": [
					".data[\"config.yaml\"].auth"
				 ]
			  },
			  {
				 "group": "*",
				 "kind": "*",
				 "managedFieldsManagers": [
					"kube-controller-manager"
				 ],
				 "name": "my-deployment",
				 "namespace": "my-namespace"
			  }
		   ],
		   "revisionHistoryLimit": 10
		}
	 }`)
	k, err := NewKubeJson(input)
	if err != nil {
		t.Fatal(err)
	}
	if k.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Fatal("GetAPIVersion failed")
	}
	if k.GetKind() != "Application" {
		t.Fatal("GetKind failed")
	}
	if k.GetNamespace() != "argocd" {
		t.Fatal("GetNamespace failed")
	}
	if k.GetName() != "guestbook" {
		t.Fatal("GetName failed")
	}
	if k.IsEmpty() {
		t.Fatal("IsEmpty failed")
	}
}

func TestKube2(t *testing.T) {
	input := []byte(`{
		
	}`)
	k, err := NewKubeJson(input)
	if err != nil {
		t.Fatal(err)
	}
	if k.GetAPIVersion() != "" {
		t.Fatal("GetAPIVersion failed")
	}
	if k.GetKind() != "" {
		t.Fatal("GetKind failed")
	}
	if k.GetNamespace() != "" {
		t.Fatal("GetNamespace failed")
	}
	if k.GetName() != "" {
		t.Fatal("GetName failed")
	}
	if !k.IsEmpty() {
		t.Fatal("IsEmpty failed")
	}
}
