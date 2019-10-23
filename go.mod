module github.com/argoproj/argo-cd

go 1.13

require (
	cloud.google.com/go v0.34.0
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/Masterminds/semver v1.4.2
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/TomOnTime/utfutil v0.0.0-20180511104225-09c41003ee1d
	github.com/argoproj/argo v2.2.2-0.20181027014425-7ef1cea68c94+incompatible
	github.com/argoproj/pkg v0.0.0-20190718233452-38dba6e98495
	github.com/asaskevich/govalidator v0.0.0-20180720115003-f9ffefc3facf
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/bouk/monkey v1.0.0
	github.com/casbin/casbin v1.5.0
	github.com/coreos/go-oidc v2.0.0+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/docker v1.6.0-rc5
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c
	github.com/dustin/go-humanize v1.0.0
	github.com/emirpasic/gods v1.9.0
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/analysis v0.17.2
	github.com/go-openapi/errors v0.17.2
	github.com/go-openapi/jsonpointer v0.19.2
	github.com/go-openapi/jsonreference v0.19.2
	github.com/go-openapi/loads v0.17.2
	github.com/go-openapi/runtime v0.17.2
	github.com/go-openapi/spec v0.19.2
	github.com/go-openapi/strfmt v0.17.0
	github.com/go-openapi/swag v0.19.2
	github.com/go-openapi/validate v0.18.0
	github.com/go-redis/cache v6.3.5+incompatible
	github.com/go-redis/redis v6.15.1+incompatible
	github.com/gobuffalo/packr v1.11.0
	github.com/gobwas/glob v0.2.3
	github.com/gogits/go-gogs-client v0.0.0-20190616193657-5a05380e4bc2
	github.com/gogo/protobuf v1.1.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.2.0
	github.com/google/go-jsonnet v0.10.0
	github.com/google/gofuzz v1.0.0
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf
	github.com/googleapis/gnostic v0.1.0
	github.com/gorilla/websocket v1.4.0
	github.com/grpc-ecosystem/go-grpc-middleware v0.0.0-20190222133341-cfaf5686ec79
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.3.1
	github.com/hashicorp/golang-lru v0.5.0
	github.com/imdario/mergo v0.3.5
	github.com/improbable-eng/grpc-web v0.0.0-20181111100011-16092bd1d58a
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99
	github.com/json-iterator/go v1.1.6
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kevinburke/ssh_config v0.0.0-20180830205328-81db2a75821e
	github.com/konsorten/go-windows-terminal-sequences v1.0.1
	github.com/mailru/easyjson v0.0.0-20190614124828-94de47d64c63
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-buffruneio v0.2.0
	github.com/pkg/errors v0.8.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/pquerna/cachecontrol v0.0.0-20180306154005-525d0eb5f91d
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/prometheus/common v0.2.0
	github.com/prometheus/procfs v0.0.0-20181204211112-1dc9a6cbc91a
	github.com/robfig/cron v1.2.0
	github.com/rs/cors v1.6.0
	github.com/sergi/go-diff v1.0.0
	github.com/sirupsen/logrus v1.4.2
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/cobra v0.0.4
	github.com/spf13/pflag v1.0.3
	github.com/src-d/gcfg v1.4.0
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.3.0
	github.com/vmihailenco/msgpack v3.3.1+incompatible
	github.com/xanzy/ssh-agent v0.2.0
	github.com/yudai/gojsondiff v1.0.1-0.20180504020246-0525c875b75c
	github.com/yudai/golcs v0.0.0-20170316035057-ecda9a501e82
	github.com/yuin/gopher-lua v0.0.0-20190115140932-732aa6820ec4
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8
	golang.org/x/net v0.0.0-20190613194153-d28f0bde5980
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	golang.org/x/tools v0.0.0-20190614205625-5aca471b1d59
	gonum.org/v1/gonum v0.0.0-20190621125449-90b715451587
	google.golang.org/appengine v1.5.0
	google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
	google.golang.org/grpc v1.15.0
	gopkg.in/go-playground/webhooks.v5 v5.11.0
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528
	gopkg.in/square/go-jose.v2 v2.2.2
	gopkg.in/src-d/go-billy.v4 v4.2.1
	gopkg.in/src-d/go-git.v4 v4.9.1
	gopkg.in/warnings.v0 v0.1.2
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190816222004-e3a6b8045b0b
	k8s.io/apiextensions-apiserver v0.0.0-20190404071145-7f7d2b94eca3
	k8s.io/apimachinery v0.0.0-20190816221834-a9f1d8a9c101
	k8s.io/client-go v11.0.1-0.20190816222228-6d55c1b1f1ca+incompatible
	k8s.io/code-generator v0.0.0-20190711102700-42c1e9a4dc7a
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/klog v0.3.3
	k8s.io/kube-aggregator v0.0.0-20190711105720-e80910364765
	k8s.io/kube-openapi v0.0.0-20190502190224-411b2483e503
	k8s.io/kubernetes v1.15.0-alpha.0.0.20190914015840-8fca2ec50a61
	k8s.io/utils v0.0.0-20190607212802-c55fbcfc754a
	layeh.com/gopher-json v0.0.0-20190114024228-97fed8db8427
	sigs.k8s.io/yaml v1.1.0
)
