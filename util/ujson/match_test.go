package ujson

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/valyala/fastjson"
)

func TestCallback(t *testing.T) {
	input := []byte(`{
        "id": 12345,
        "name": "foo",
        "numbers": ["one", "two"],
        "tags": {"color": "red", "priority": "high"},
        "active": true,
		"numbers": ["one", "two"]
    }`)
	callbacks := []*MatchCallback{
		{
			paths: []string{"\"numbers\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value is [ symbol
				if string(value) != "[" {
					t.Fatal("expected value to be [")
				}
				return nil
			},
		},
		{
			paths: []string{"\"tags\"", "\"color\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert color red
				if value != nil && string(value) != "\"red\"" {
					t.Fatal("expected value to be \"red\"")
				}
				return nil
			},
		},
	}
	opt := MatchOptions{
		IgnoreCase:       true,
		QuitIfNoCallback: true,
	}
	err := Match(input, &opt, nil, callbacks...)
	if err != nil {
		t.Fatal(err)
	}
}

func TestArgoApplication(t *testing.T) {
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
	callbacks := []*MatchCallback{
		{
			paths: []string{"\"metadata\"", "\"name\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				if string(value) != "\"guestbook\"" {
					t.Fatal("expected value to be \"guestbook\"")
				}
				return nil
			},
		},
		{
			paths: []string{"\"metadata\"", "\"namespace\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				if string(value) != "\"argocd\"" {
					t.Fatal("expected value to be \"argocd\"")
				}
				return nil
			},
		},
		{
			paths: []string{"\"kind\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				if string(value) != "\"Application\"" {
					t.Fatal("expected value to be \"Application\"")
				}
				return nil
			},
		},
		{
			paths: []string{"\"apiVersion\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				if string(value) != "\"argoproj.io/v1alpha1\"" {
					t.Fatal("expected value to be \"argoproj.io/v1alpha1\"")
				}
				return nil
			},
		},
		{
			paths: []string{"\"tags\"", "\"color\""},
			cb: func(paths [][]byte, value []byte) error {
				// assert value
				if string(value) != "\"red\"" {
					t.Fatal("expected value to be \"red\"")
				}
				return nil
			},
		},
	}
	opt := MatchOptions{
		IgnoreCase:       false,
		QuitIfNoCallback: true,
	}
	res := MatchResult{
		Count: 0,
	}
	// save start time
	begin := time.Now()
	err := Match(input, &opt, &res, callbacks...)
	if err != nil {
		t.Fatal(err)
	}
	// print time taken
	fmt.Printf("Time taken for Match(): %v\n", time.Since(begin))
	// assert count is 225
	if res.Count != 225 {
		t.Fatal("expected count to be 225")
	}

	// use fastjson to parse the same json
	// save start time
	begin = time.Now()
	_, err = fastjson.ParseBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	// print time taken
	fmt.Printf("Time taken for fastjson.ParseBytes(): %v\n", time.Since(begin))

	// use encoding/json to parse the same json
	// save start time
	var v interface{}
	begin = time.Now()
	err = json.Unmarshal(input, &v)
	if err != nil {
		t.Fatal(err)
	}
	// print time taken
	fmt.Printf("Time taken for json.Unmarshal(): %v\n", time.Since(begin))
}
