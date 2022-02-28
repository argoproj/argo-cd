---
title: Adding support for RBAC CLI/Web requests
authors:
  - "@chetan-rns" # Authors' github accounts here.
  - "@ishitasequeira"
sponsors:
  - TBD        # List all intereste parties here.
reviewers:
  - "@alexmt"
  - "@jgwest"
approvers:
  - "@alexmt"
  - "@jgwest"

creation-date: yyyy-mm-dd
last-updated: yyyy-mm-dd
---

# Neat Enhancement Idea

This is a proposal to add support for creating ApplicationSets via the Argo CD Web CLI, by adding support to the ApplicationSet and API Server backend that respects Argo CD RBAC

## Summary

Currently, users can only create ApplicationSets by applying applicationSet configurations using `kubectl`. Creating cli for allowing users to perform operations on ApplicationSet would give users to perform faster operations.

## Motivation

As ApplicationSet Controller is now part of ArgoCD installation, we would like to allow users to be able to create/update/delete application sets via cli just like argocd.

### Goals

* **Create CLI for users to interact with ApplicationSets**

  Users should be able to create/update/delete ApplicationSets using argocd CLI.

* **Changes to CLI**

  Add a new argocd command option `appset` with `create`, `delete` and `apply` sub-commands.

### Non-Goals

*

## Proposal

This is the high level overview of creation (update/deletion) of an ApplicationSet via Argo CD CLI.
![High Level Architecture](./backend-support-appset.png)


User issues `argocd appset create/update/delete` command from CLI, into an Argo CD server on which they are logged-in. The command converts the command request into GRPC and sends it off to Argo CD API Server.

#### **Argo CD API Server:**
The API Server receives Create/Update/Delete request via GRPC and verifies that ApplicationSet controller is installed within the namespace (if not, return an error response back to user).

The Api Server sends the GRPC request to the ApplicationSet controller via GRPC including authentication information from the user in the request.

#### **ApplicationSet controller:**
ApplicationSet controller receives Create/Update/Delete GRPC. (These next steps will be a create example, but update and delete are similar.)

**Pre-generate check:** Application controller will perform various checks before creating ApplicationSet:

* Ensure user has appropriate RBAC to run the generator:
    Verify that the user can access the Git repository (for Git generators)
* Verify that user has cluster access (to see the clusters, for Cluster generator)
* Verify that the user has permission to create/update/delete (depending on the request type) at least one Application within the RBAC policy. We want to prevent the generators being invoked by users that don't have permissions to create any Applications (since generators or templates might be exploited to DoS the ApplicationSet controller, using a malicious ApplicationSet)

Once the pre-checks have been confirmed, the controller will run the generator, and render the parameters into the template. Upon generating the template, the controller will need to perform some checks before creating the Applications.

**Post-generate check:** Look at the generated Applications (but don't apply them yet!), and verify that user has the required RBAC permissions to perform the required actions
This is a dynamic (or runtime) check, as it works on the dynamically generated applications; eg. it is not possible to predict the result of these checks without first running the generator, unlike the static checks.

Once all the checks have passed, apply the ApplicationSet and the Applications, to the namespace.


### Detailed examples

### Security Considerations

### Risks and Mitigations


### Use cases
#### Use case 1: 
#### Use case 2: 



### Implementation Details/Notes/Constraints [optional]



### Upgrade / Downgrade Strategy

## Drawbacks


## Alternatives

Rather than using Argo CD's CLI, we could create a new AppSet CLI "appset" that would communicate directly with the ApplicationSet deployment, rather than going through the Argo CD API Server as an intermediary (though if we were adding web UI support, this would still be required regardless).