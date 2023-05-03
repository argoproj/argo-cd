---
title: First class status for Argo CD Image Updater
authors:
  - "@jaideepr97" # Authors' github accounts here.
sponsors:
  - @jannfis
  - @wtam2018        # List all interested parties here.
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2022-12-01
last-updated: 2023-02-22
---

# Provide first class status to Image Updater within Argo CD

[Argo CD Image Updater](https://github.com/argoproj-labs/argocd-image-updater) is a popular project which solves an important gap within the CI/CD workflow by providing the ability to update application workload images automatically. It is designed around Argo CD itself and naturally compliments it. This proposal makes a case for upgrading it to first class citizen status within Argo CD and the changes that would involve to the Application CRD.


## Summary

Argo CD users would benefit from having Image Updater included as a first class workload. Using Annotations to configure image updates is not the preferred option, hence this proposal suggests adding new fields to the Application CR spec to store Image Updater configuration fields, as well as new fields in the Application status to store the state of image updates, enabling new features in the Image Updater in the future, which can all be leveraged by users of Argo CD. 


## Motivation

Argo CD Image Updater is a popular project which solves an important gap within the CI/CD workflow by providing the ability to update application workload images automatically (subject to appropriate constraints). It is designed around Argo CD itself and naturally compliments it. The configuration required for it revolves almost entirely around the Application resource itself. Currently the Image Updater uses annotations on the application resource to store configuration, which is not the preferred way to do this for a number of reasons, including:
  - It is easy to make errors and misconfigurations as a user when using annotations
  - It is harder to validate annotations. Error checking on the controller's end could involve complex string parsing and does not allow us to leverage existing validation solutions provided to us by Kubernetes.
  - It is harder to express complicated configuration. Leveraging powerful features often results in complex and verbose annotations that are not easy to work with or troubleshoot. 
It also doesn't store any state, and is, as such, unable to provide some key features that are discussed in further sections. Consolidating this configuration into the Application CR as first class fields in the spec and status is a natural next step in the Image Updater's journey. 

### Goals

- Propose updated Application CR fields to include configuration scoped for image updates
- Make a case to merge Image Updater controller code into core Argo CD as its own controller 
- Outline future enhancements planned for Image Updater and its growth within Argo CD 

### Non-Goals

TBD

## Proposal

### Use cases

#### Use case 1:
As a User of Argo CD, I would like to be able to have my application images updated automatically within expressed constraints when there is a new version available, out of the box with Argo CD. 

#### Use case 2:
As a user of Argo CD and Argo CD Image Updater, I would like to be able to express advanced Image update configurations through custom resource fields in my application.

For users wanting to use Image Updater in production, it is common to have a strict set of restrictions in place in terms of which images can/should be automatically updated, where these images should come from, which reposoitry/branch the commits should be made to and what credentials should be used among many others. It is important to have a set of first class custom resource fields to be able to express this information accurately and reliably. This would add considerable value to the user experience, including but not limited to increased ease of use, stronger validation and standardization for these fields.
 
#### Use case 3:
As a user of Argo CD and Argo CD Image Updater, I would like to have image-updater honor rollbacks to my applications by leveraging status information stored in my application.


### Image Updater functionality constraints and behaviors

This section highlights some of the key aspects of Image Updater's present design constraints and behaviors in order to provide context for the upcoming sections of the proposal to people who may not be familiar with the project. Full details may be found in the Image Updater documentation (https://argocd-image-updater.readthedocs.io/en/stable/)  

- Image Updater can only update container images for applications whose manifests are rendered using either Kustomize or Helm and - especially in the case of Helm - the templates need to support specifying the image's tag (and possibly name) using a parameter (i.e. image.tag).
- Image Updater only considers those applications that specify a certain set of annotations as candidates for image updates
- Image Updater can be configured to either communicate directly with the kubernetes api of the cluster it is running on, or it can be configured to communicate with the Argo CD API instead. It determines which images are suitable upgrade targets, finds the appropriate updated image version, and injects the changes into the application's source manifest (on the cluster if communicating with k8s, or through the `argocd-server` in configured to talk to Argo CD API), allowing Argo CD to take over and pull the new image into the live workloads. It supports 2 kinds of write back methods (Details can be found at https://argocd-image-updater.readthedocs.io/en/stable/basics/update-methods/):
  - Argo CD method: Pseudo persistent. Updates are written into application manifest (on cluster or through the argocd-server) and then depending on your Automatic Sync Policy for the Application, Argo CD will either automatically deploy the new image version or mark the Application as Out Of Sync. Invalidation of application controller cache will cause loss of all changes
  - Git method: Persistent. Updates are committed directly into the git repo branch specified in configuration. Changes are applied to the cluster by Argo CD on next sync cycle. 
- When performing git write-back, Image Updater does not duplicate all the git handling done by repo-server or write back to any manifests directly. By default it simply creates a file titled 
  `.argocd-source-<appName>.yaml` containing the updated information (or alternatively a specified kustomization file). As such, information written back is minimal. 
- When Image Updater is configured to write back to a different branch than the one being tracked by the application, it is the user's responsibility to make sure changes are propogated to the tracked branch in order for Argo CD to pick up the changes and deploy them on the cluster
- Image Updater currently stores no state for actions performed, and is therefore incapable of honoring rollbacks  


### Implementation Details/Notes/Constraints [optional]


- Argo CD Image Updater code base will need to be updated to look for application resources in namespaces other than the control-plane namespace when configured to use the k8s api instead of the argocd api to retrieve resources (see https://argocd-image-updater.readthedocs.io/en/stable/install/installation/#installation-methods). Currently it only recognizes applications present in the control plane namespace when installed in this manner. Exposed prometheus metric labels will need to be updated to take application namespace into account for accurate filtering. 

- Image Updater controller code will need to be updated to watch and react to changes in newly defined application CR fields, and code to parse and react to annotations will need to be removed. 

- If application is configured with multiple source repositories, users will have to explicitly specify which source should be used for git write back. Care would need to be taken to ensure image updates are not accidentally overwritten when the application is being compiled together from all the different source repositories.  

- Image Updater controller would need to be able to update fields in the status of the Application CR in order to maintain state, as described in the following section. This would also mean there would be fields in the Application CR that would not be used by Argo CD itself, but by a different component (Image Updater). 

- There could be a case made for separating Image Updater configuration into its own dedicated CRD, however since the application resource is central to the image update configurations, it would be a lot more intuitive for this configuration to belong within the application CR itself. Secondly, having a dedicated CR would mean that the number of required resources on the cluster would double since we would need to have a dedicated image update CR for each application.

- In terms of the actual merging of the codebases, the ideal approach would likely be to pull the image-updater code into the core Argo CD codebase, but keep it largely isolated as its own controller. This would seem logical since there is sufficient distinction between the purpose of the Image Updater's controller code and the tasks rest of the core components (like application-controller) perform that we can maintain a clean separation of concerns. It would also be easier to maintain in the long run. There is also a case that can be made to keep the code completely de-coupled in its own repository in the argoproj org since there is not as much inter-dependency between the 2 projects, however this may send mixed signals to the community regarding the capacity in which Image Updater has been granted "first class" status within Argo CD. 

### Detailed examples


At present, users of Image Updater must add a host of specific annotations to their applications in order to configure automatic image updates on them. As the user base of the Image Updater project has grown, the use cases and requirements have also gotten more complex and demanding. As these features have been implemented and delivered to the users, the ways to use annotations to leverage these features have become more complicated, involving somewhat convoluted mechanisms to express references to resources like secrets, and setting up templates for branch names. Using annotations also opens up the possibility of misconfigurations by means of usage of "random" annotation names (containing typos) which make it cumbersome to troubleshoot issues.
Aside from providing strict type validation, converting these fields to first class fields in the CR would simplify the expression and usage of these fields and make them more intuitive for users to understand and leverage. 

Example of proposed additions to Application CR spec and status to accomodate image update configuration is provided below:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  ...

spec:
...

  image:
    # updates contains all the fields to configure exactly which images should be considered for automatic updates and how they should be updated
    updates:
      # ImageList contains list of image specific configuration for each image that is to be considered as a candidate for updation 
      imageList:
        - image: quay.io/some/image 

          # (optional) Specify Semver constraint for allowed tags 
          constraint: 1.17.x

          # (optional) specify list of regex/wildcard patterns to further filter which tags should be considered for updates
          allowTags: 
            matchType: regex/wildcard
            matchList:
            - <pattern 1>
            - <pattern 2>

          # (optional) specify list of regex/wildcard patterns to dictate which tags should be ignored for updates
          ignoreTags: 
            matchType: regex/wildcard
            matchList:
            - <pattern 1>
            - <pattern 2>

          # (optional) force updates to images that don't appear in the status of an Argo CD application. Default is false   
          forceUpdate: true/false

          # specify strategy to be followed when determining which images have qualifying updates 
          updateStrategy: semver/digest/lexical/most-recently-built

          # specify list of architectures to be considered for a specific image if application is to be run across multiple clusters of differing architectures
          imagePlatforms:
          - linux/amd64
          - linux/arm64

          # (optional) pull credentials for image 
          credentials:
            # generic secret containing pull credentials in <username>:<password> format 
            secret:
              name: generic-secret
              namespace: argocd
              field: imagePullCreds
            
            # alternatively, use a docker pull secret containing valid Docker config in JSON format in the field .dockerconfigjson
            pullSecret:
              name: docker-pullsecret
              namespace: argocd
            
            # alternatively, if using an env var instead of secrets
            env: DOCKER_HUB_CREDDS

            # or use an external script mounted to image-updater FS to generate credentials
            ext: <path/to/script>
            
            credsExpire: <timestamp for when credentials expire>

          # (optional) specify helm parameter names to be used for image update write back
          helm:
            imageName: someImage.image  
            imageTag:  someImage.version
            imageSpec: someImage.Spec
          
          # (optional) specify original image name for kustomize to override with updated image + tag
          # see https://argocd-image-updater.readthedocs.io/en/stable/configuration/images/#custom-images-with-kustomize for details
          kustomize:
            imageName: quay.io/original/image
      
      # (optional) Application wide configuration
      allowTags: 
            matchType: regex/wildcard
            matchList:
            - <pattern 1>
            - <pattern 2>
      ignoreTags: 
            matchType: regex/wildcard
            matchList:
            - <pattern 1>
            - <pattern 2>
      forceUpdate: true/false
      updateStrategy: semver/digest/lexical/most-recently-built      
      credentials:
        secret:
          name: generic-secret
          namespace: argocd
          field: imagePullCreds
        pullSecret:
          name: docker-pullsecret
          namespace: argocd
        env: DOCKER_HUB_CREDDS
        ext: <path/to/script>        
        credsExpire: <timestamp for when credentials expire>
      
      # (optional) configuration for application-wide write-back strategy. Default write back method is argocd 
      writeBack:
        method: git/argocd

        # (optional) in case application is configured with multiple sources, specify the repoURL and path for the source 
        # that should be used for git write back
        repoURL: https://github.com/source/repo
        path: some/path

        # (optional) specify branch to checkout/commit to if different from revision tracked in application spec (when protected, for e.g.)
        baseBranch: <base_branch>

        # (optional) specify commit branch for commits if there is need for separate read/write branches
        # supports templating if there is need for non-static commit branch names
        # see https://argocd-image-updater.readthedocs.io/en/stable/basics/update-methods/#specifying-a-separate-base-and-commit-branch for details
        commitBranch: <commit_branch>


        # (optional) By default, git write-back will create or update .argocd-source-<appName>.yaml
        # setting target to kustomization will have a similar effect to running `kustomize edit set image`
        target: kustomization/default
        # (optional) Specify the kustomization directory (not file path) to edit with relative or absolute path
        kustomization:
          path: "../../base"
        
        # (optional) credentials for git write-back
        secret:
          namespace: argocd
          name: git-creds
          field: data
    
status:
...
  # imageUpdate stores the history of image updates that have been
  # conducted, and contains information about which image was last 
  # updated, its old and new versions and update timestamp, in order to respect 
  # application rollbacks and avoid repeat update attempts
  imageUpdates:
  - image: quay.io/some/image
    lastTransitionTime: <update_timestamp>
    oldTag: v1.0.0
    newTag: v1.1.0
    digest: 9315cd9d987b2ec50be5529c18efaa64ebae17f72ae4b292525ca7ee2ab98318
     
```

**NOTE:** At first glance it may seem redundant to have helm and kustomize configuration in `imageList` while we already have `.spec.source.helm.parameters` and `.spec.source.kustomize.images`, however, these fields are required in order to allow image updater to automate the step of specifying helm/kustomize image overrides through the application CR.
For e.g, specifying `helm.imageTag: someImage.version` allows image updater to know that the updated image tag should be written as the value to `someImage.version` under `spec.source.helm.parameters` in the application manifest (either directly on the cluster via "argocd" write-back method or to the target write-back file via "git" write-back method) 

There is a need to introduce state by tracking the status of image Updates in the application CR. This is mainly important to honor application rollbacks, avoid repeated image updates in case of failure, as well as to support future use cases such as enabling image-updater to create pull requests against the source repositories. The above detailed `.status.imageUpdates` field could be extended to store metadata regarding pull request creation that would serve to avoid duplicate/excessive number of pull requests being opened, for example.

There could be a few different options in terms of placement for this block of information:

1. `.status.imageUpdates` - As shown above, we could introduce a new status field at the highest level that could store a slice of `imageUpdates` containing information about the last version updated to and timestamp of last update etc. This would be a non breaking change and easy to implement/maintain/extend
2. `.status.summary.images` - Currently this field exists as a slice of strings that just contains the full qualified paths of all images associated to an application. This filed could be repurposed by changing it from `[]string` to `[]imageUpdate` to store the additional information above. This seems like a naturally better place for it, however this would be a breaking change (and at present  would also break Image Updater as it uses this field to get the list of potentially updateable images)
3. `.status.summary.imageUpdates` - This field could instead be introduced within the applicaiton summary with the understanding that `status.summary.images` will be deprecated and eventually removed, since it will be rendered obsolete by this new field. 

## Open Questions [optional]

Based on this proposal, if utilizing the "argocd" write back method, the Image Updater controller would need to write updates to some application spec fields (`spec.source.helm`, `.spec.source.kustomize`) and the application status fields in order to track the state of image updates for various images. This would mean we could potentially have 2 controllers writing to the application resource. While it is not unusual to have multiple controllers writing to different fields on the same resource, would this be better delegated to the argo-cd server since it already has use cases to write to the application resource on the cluster? 


### Security Considerations

With the rise in interest in software supply chain security across the industry, one aspect worth considering is that the introduction of Image Updater into Argo CD would expose Argo CD toward an entirely new type of source repository in image registries. Image Updater interfaces directly with various image registries, and as such pulls image manifests from these sources, currently trusting that the user has configured reputable image registries for their purposes. Moving forward, it would be reasonable to suggest that we could be more careful and selective about which images/registries we would want to allow Argo CD to pull images from, in order to minimize exposure to bad actors. 

One of the potential enhancements that could be introduced within Image Updater down the line would be the ability to verify that images are signed through potentially leveraging existing container verification solutions. The above proposed CR fields could be expanded to build out `.spec.image.verification`, for instance. This could be used to store specific configuration/constraints expressed by the user to ensure the trustworthiness of containers being used in their applications. One example of this would be storing the location of a public key to verify an image against.


### Risks and Mitigations

Introduction of Image Updater would be an addition of yet another controller in an already rather complex codebase. There is a risk of making Argo CD more resource intensive as a workload. However, following up from previous discussions conducted on this topic within the contributor's meeting, there are ways to mitigate this risk. 

A potential option that was discussed was to combine less resource intensive workloads (such as applicationsets, notifications controller and image-updater) into a single process/pod rather than have them each run as their own separate process. 


### Upgrade / Downgrade Strategy

TBD

## Drawbacks

Some potential drawbacks of this proposal are:
- Addition of new fields to the application CRD for Image update configuration could make the CRD heavier, with potentially more fields being added to the future
- Addition of Image Updater controller code to the core Argo CD codebase would increase the overall complexity of the codebase and increase maintenance overhead.

That being said, we believe the benefits that we will achieve through this proposal would be worth the risks/drawbacks involved.

## Alternatives

### Alternave #1: 

One alternative approach would be to keep Argo CD Image Updater as a separate component altogether and store the previously described configuration in a new, dedicated CRD for the Image Updater. This would also satisfy some of the use cases described in this proposal. However, it would come with considerations of its own, some of which are:
- Users would have to move all Image Updater configuration away from the application CR (presently expressed as annotations on the application) into this new CR.
- The new CRD would need to have a field to express an application reference so it is clear which application the image update configuration targets.
- There would need to be an instance of the new CR for each application that is targeted, essentially doubling the number of resources on the cluster.
- This might introduce some complications when it comes to working with ApplicationSets, as there would need to be some mechanism in place to express how ApplicationSets would be targeted in the new CR, and how the image update configuration would be applied to the applications created by the ApplicationSet.

### Alternative #2:

Following up on the discussion had during the contributor meeting, a second alternative approach would be to integrate the image-udpater controller completely into the application-controller. 

Benefits of merging image-updater into application-controller:

- Avoids addition of a new/separate component that needs to be enabled/disabled/managed within an Argo-cd deployment
- Eliminates concern outlined in Open Questions as there would only be a single component reading/writing to the application resource
- Provides Image Updater functionality out of the box for all Argo CD installations even more natively
- Image updater would receive native awareness of all applications in control-plane and non-control plane namespaces

Drawbacks:

- Loss of separation of concerns between application resource management and image updation that currently exists (it can probably still be achieved within the application-controller but the code may need to be structured more carefully)
- Adds additional complexity to the application-controller that would now have to interact with image-registries & perform git write-backs as well (alternatively, git write-back functionality may potentially need to be extracted out and added to the repo-server instead)
- Might impact maintenance/reliability of application-controller code
- Separate controller would be consistent with how projects like appset and notifications controller have been merged into argo-cd in the past

## Future enhancements

At present there are some initiatives that are being considered which could enhance the usability and adoption of Image Updater with users in the future, some of which have been mentioned in this proposal already:

1. Support for opening pull requests to source repositories and honoring rollbacks - At present Image Updater directly commits manifest updates to the target branch when git write-back is configured. However, if status information about the last conducted update and version updated to is available, Image Updater could use that information to potentially support creating pull requests (and not spam pull requests) and also not continuously try to upgrade to a new image version if an application has been rolled back. This is a feature that has been requested by users of the Image Updater. There are a number of use cases that would benefit from Pull request support in Image Updater, such as ones needing all changes to go through PRs as opposed to direct commits, requirments to run CI checks on PRs before merging image updates, rejecting PRs that try to pull in faulty images etc, among others. There are already some ideas for how this could be implemented. This would make for a great value-add as a future enhancement to the Image Updater's functionality.

2. Change Image updater operational model to become webhook aware - At present the updater polls the configured image registries periodically to detect new available versions of images. In the future Image Updater would need to be able to support webhooks so that it can move to a pub/sub model and updates can be triggered as soon as a qualifying image version is found.

3. Supporting container verification - As described earlier, this could be a useful feature to reduce security risks introduced into Argo CD while providing all the benefits of having automatic image updates available. 