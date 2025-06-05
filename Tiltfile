load('ext://restart_process', 'docker_build_with_restart')
load('ext://uibutton', 'cmd_button', 'location')

# add ui button in web ui to run make codegen-local (top nav)
cmd_button(
    'make codegen-local',
    argv=['sh', '-c', 'make codegen-local'],
    location=location.NAV,
    icon_name='terminal',
    text='make codegen-local',
)

# add ui button in web ui to run make codegen-local (top nav)
cmd_button(
    'make cli-local',
    argv=['sh', '-c', 'make cli-local'],
    location=location.NAV,
    icon_name='terminal',
    text='make cli-local',
)

# detect cluster architecture for build
cluster_version = decode_yaml(local('kubectl version -o yaml'))
platform = cluster_version['serverVersion']['platform']
arch = platform.split('/')[1]

# build the argocd binary on code changes
code_deps = [
    'applicationset',
    'cmd',
    'cmpserver',
    'commitserver',
    'common',
    'controller',
    'notification-controller',
    'pkg',
    'reposerver',
    'server',
    'util',
    'go.mod',
    'go.sum',
]
local_resource(
    'build',
    'CGO_ENABLED=0 GOOS=linux GOARCH=' + arch + ' go build -gcflags="all=-N -l" -mod=readonly -o .tilt-bin/argocd_linux cmd/main.go',
    deps = code_deps,
    allow_parallel=True,
)

# deploy the argocd manifests
k8s_yaml(kustomize('manifests/dev-tilt'))

# build dev image
docker_build_with_restart(
    'argocd', 
    context='.',
    dockerfile='Dockerfile.tilt',
    entrypoint=[
        "/usr/bin/tini",
        "-s",
        "--",
        "dlv",
        "exec",
        "--continue",
        "--accept-multiclient",
        "--headless",
        "--listen=:2345",
        "--api-version=2"
    ],
    platform=platform,
    live_update=[
        sync('.tilt-bin/argocd_linux_amd64', '/usr/local/bin/argocd'),
    ],
    only=[
        '.tilt-bin',
        'hack',
        'entrypoint.sh',
    ],
    restart_file='/tilt/.restart-proc'
)

# build image for argocd-cli jobs
docker_build(
    'argocd-job', 
    context='.',
    dockerfile='Dockerfile.tilt',
    platform=platform,
    only=[
        '.tilt-bin',
        'hack',
        'entrypoint.sh',
    ]
)

# track argocd-server resources and port forward
k8s_resource(
    workload='argocd-server',
    objects=[
        'argocd-server:serviceaccount',
        'argocd-server:role',
        'argocd-server:rolebinding',
        'argocd-cm:configmap',
        'argocd-cmd-params-cm:configmap',
        'argocd-gpg-keys-cm:configmap',
        'argocd-rbac-cm:configmap',
        'argocd-ssh-known-hosts-cm:configmap',
        'argocd-tls-certs-cm:configmap',
        'argocd-secret:secret',
        'argocd-server-network-policy:networkpolicy',
        'argocd-server:clusterrolebinding',
        'argocd-server:clusterrole',
    ],
    port_forwards=[
        '8080:8080',
        '9345:2345',
        '8083:8083'
    ],
)

# track crds
k8s_resource(
    new_name='cluster-resources',
    objects=[
        'applications.argoproj.io:customresourcedefinition',
        'applicationsets.argoproj.io:customresourcedefinition',
        'appprojects.argoproj.io:customresourcedefinition',
        'argocd:namespace'
    ]
)

# track argocd-repo-server resources and port forward
k8s_resource(
    workload='argocd-repo-server',
    objects=[
        'argocd-repo-server:serviceaccount',
        'argocd-repo-server-network-policy:networkpolicy',
    ],
    port_forwards=[
        '8081:8081',
        '9346:2345',
        '8084:8084'
    ],
)

# track argocd-redis resources and port forward
k8s_resource(
    workload='argocd-redis',
    objects=[
        'argocd-redis:serviceaccount',
        'argocd-redis:role',
        'argocd-redis:rolebinding',
        'argocd-redis-network-policy:networkpolicy',
    ],
    port_forwards=[
        '6379:6379',
    ],
)

# track argocd-applicationset-controller resources
k8s_resource(
    workload='argocd-applicationset-controller',
    objects=[
        'argocd-applicationset-controller:serviceaccount',
        'argocd-applicationset-controller-network-policy:networkpolicy',
        'argocd-applicationset-controller:role',
        'argocd-applicationset-controller:rolebinding',
        'argocd-applicationset-controller:clusterrolebinding',
        'argocd-applicationset-controller:clusterrole',
    ],
    port_forwards=[
        '9347:2345',
        '8085:8080',
        '7000:7000'
    ],
)

# track argocd-application-controller resources
k8s_resource(
    workload='argocd-application-controller',
    objects=[
        'argocd-application-controller:serviceaccount',
        'argocd-application-controller-network-policy:networkpolicy',
        'argocd-application-controller:role',
        'argocd-application-controller:rolebinding',
        'argocd-application-controller:clusterrolebinding',
        'argocd-application-controller:clusterrole',
    ],
    port_forwards=[
        '9348:2345',
        '8086:8082',
    ],
)

# track argocd-notifications-controller resources
k8s_resource(
    workload='argocd-notifications-controller',
    objects=[
        'argocd-notifications-controller:serviceaccount',
        'argocd-notifications-controller-network-policy:networkpolicy',
        'argocd-notifications-controller:role',
        'argocd-notifications-controller:rolebinding',
        'argocd-notifications-cm:configmap',
        'argocd-notifications-secret:secret',
    ],
    port_forwards=[
        '9349:2345',
        '8087:9001',
    ],
)

# track argocd-dex-server resources
k8s_resource(
    workload='argocd-dex-server',
    objects=[
        'argocd-dex-server:serviceaccount',
        'argocd-dex-server-network-policy:networkpolicy',
        'argocd-dex-server:role',
        'argocd-dex-server:rolebinding',
    ],
)

# track argocd-commit-server resources
k8s_resource(
    workload='argocd-commit-server',
    objects=[
        'argocd-commit-server:serviceaccount',
        'argocd-commit-server-network-policy:networkpolicy',
    ],
    port_forwards=[
        '9350:2345',
        '8088:8087',
        '8089:8086',
    ],
)

# docker for ui
docker_build(
    'argocd-ui',
    context='.',
    dockerfile='Dockerfile.ui.tilt',
    entrypoint=['sh', '-c', 'cd /app/ui && yarn start'], 
    only=['ui'],
    live_update=[
        sync('ui', '/app/ui'),
        run('sh -c "cd /app/ui && yarn install"', trigger=['/app/ui/package.json', '/app/ui/yarn.lock']),
    ],
)

# track argocd-ui resources and port forward
k8s_resource(
    workload='argocd-ui',
    port_forwards=[
        '4000:4000',
    ],
)

# linting
local_resource(
    'lint',
    'make lint-local',
    deps = code_deps,
    allow_parallel=True,
)

local_resource(
    'lint-ui',
    'make lint-ui-local',
    deps = [
        'ui',
    ],
    allow_parallel=True,
)

local_resource(
    'vendor',
    'go mod vendor',
    deps = [
        'go.mod',
        'go.sum',
    ],
)

