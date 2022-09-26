# ApplicationSet Security

ApplicationSet is a powerful tool, and it is crucial to understand its security implications before using it.

## Only admins may create/update/delete ApplicationSets

ApplicationSets can create Applications under arbitrary [Projects](../../user-guide/projects.md). Argo CD setups often
include Projects (such as the `default`) with high levels of permissions, often including the ability to manage the 
resources of Argo CD itself (like the RBAC ConfigMap).

ApplicationSets can also quickly create an arbitrary number of Applications and just as quickly delete them.

Finally, ApplicationSets can reveal privileged information. For example, the [git generator](./Generators-Git.md) can
read Secrets in the Argo CD namespace and send them to arbitrary URLs (e.g. URL provided for the `api` field) as auth headers.
(This functionality is intended for authorizing requests to SCM providers like GitHub, but it could be abused by a malicious user.)

For these reasons, **only admins** may be given permission (via Kubernetes RBAC or any other mechanism) to create, 
update, or delete ApplicationSets.

## Admins must apply appropriate controls for ApplicationSets' sources of truth

Even if non-admins can't create ApplicationSet resources, they may be able to affect the behavior of ApplicationSets.

For example, if an ApplicationSet uses a [git generator](./Generators-Git.md), a malicious user with push access to the
source git repository could generate an excessively high number of Applications, putting strain on the ApplicationSet
and Application controllers. They could also cause the SCM provider's rate limiting to kick in, degrading ApplicationSet
service.

### Templated `project` field

It's important to pay special attention to ApplicationSets where the `project` field is templated. A malicious user with
write access to the generator's source of truth (for example, someone with push access to the git repo for a git
generator) could create Applications under Projects with insufficient restrictions. A malicious user with the ability to
create an Application under an unrestricted Project (like the `default` Project) could take control of Argo CD itself
by, for example, modifying its RBAC ConfigMap.

If the `project` field is not hard-coded in an ApplicationSet's template, then admins _must_ control all sources of 
truth for the ApplicationSet's generators.
