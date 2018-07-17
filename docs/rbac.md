# RBAC

## Overview

The feature RBAC allows restricting access to ArgoCD resources. ArgoCD does not have own user management system and has only one built-in user `admin`. The `admin` user is a
superuser and it has full access. RBAC requires configuring [SSO](./sso.md) integration. Once [SSO](./sso.md) is connected you can define RBAC roles and map roles to groups.

## Configure RBAC

RBAC configuration allows defining roles and groups. ArgoCD has two pre-defined roles: role `role:readonly` which provides read-only access to all resources and role `role:admin`
which provides full access. Role definitions are available in [builtin-policy.csv](../util/rbac/builtin-policy.csv) file.

Additional roles and groups can be configured in `argocd-rbac-cm` ConfigMap. The example below custom role `org-admin`. The role is assigned to any user which belongs to
`your-github-org:your-team` group. All other users get `role:readonly` and cannot modify ArgoCD settings.

*ConfigMap `argocd-rbac-cm` example:*

```yaml
apiVersion: v1
data:
  policy.default: role:readonly
  policy.csv: |
    p, role:org-admin, applications, *, */*
    p, role:org-admin, applications/*, *, */*

    p, role:org-admin, clusters, get, *
    p, role:org-admin, repositories, get, *
    p, role:org-admin, repositories/apps, get, *

    p, role:org-admin, repositories, create, *
    p, role:org-admin, repositories, update, *
    p, role:org-admin, repositories, delete, *

    g, your-github-org:your-team, role:org-admin
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
```

## Configure Projects

Argo projects allow grouping applications which is useful if ArgoCD is used by multiple teams. Additionally, projects restrict source repositories and destination
Kubernetes clusters which can be used by applications belonging to the project.

### 1. Create new project

Following command creates project `myproject` which can deploy applications to namespace `default` of cluster `https://kubernetes.default.svc`. The source ksonnet application
should be defined in `https://github.com/argoproj/argocd-example-apps.git` repository.

```
argocd proj create myproject -d https://kubernetes.default.svc,default -s https://github.com/argoproj/argocd-example-apps.git
```

Project sources and destinations can be managed using commands `argocd project add-destination`, `argocd project remove-destination`, `argocd project add-source`
and `argocd project remove-source`.

### 2. Assign application to a project

Each application belongs to a project. By default, all application belongs to the default project which provides access to any source repo/cluster. The application project can be
changes using `app set` command:

```
argocd app set guestbook-default --project myproject
```

### 3. Update RBAC rules

Following example configure admin access for two teams. Each team has access only two application of one project (`team1` can access `default` project and `team2` can access
`myproject` project).

*ConfigMap `argocd-rbac-cm` example:*

```yaml
apiVersion: v1
data:
  policy.default: ""
  policy.csv: |
    p, role:team1-admin, applications, *, default/*
    p, role:team1-admin, applications/*, *, default/*

    p, role:team1-admin, applications, *, myproject/*
    p, role:team1-admin, applications/*, *, myproject/*

    p, role:org-admin, clusters, get, *
    p, role:org-admin, repositories, get, *
    p, role:org-admin, repositories/apps, get, *

    p, role:org-admin, repositories, create, *
    p, role:org-admin, repositories, update, *
    p, role:org-admin, repositories, delete, *

    g, role:team1-admin, org-admin
    g, role:team2-admin, org-admin
    g, your-github-org:your-team1, role:team1-admin
    g, your-github-org:your-team2, role:team2-admin
kind: ConfigMap
metadata:
  name: argocd-rbac-cm
```
