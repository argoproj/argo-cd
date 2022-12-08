---
title: First class status for Argo CD Image Updater
authors:
  - "@jaideepr97" # Authors' github accounts here.
sponsors:
  - @jaanfis
  - @wtam2018        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2022-12-01
last-updated: 2022-12-01
---

# Provide first class status to Image Updater within Argo CD

Argo CD Image Updater is a popular project which solves an important gap within the CI/CD workflow by providing the ability to update application workload images automatically. It is designed around Argo CD itself and naturally compliments it. This proposal makes a case for upgrading it to first class citizen status within Argo CD and the changes that would involve to the Application CRD.


## Summary

Argo CD users would benefit from having image updater included as a first class workload. Using Annotations to configure image updates is not the preferred option, hence this proposal suggests adding new fields to the Application CR spec to store image updater configuration fields, as well as new fields in the Application status to store the state of image updates, enabling new features in the image update in the future, which can all be leveraged by users of Argo CD. 


## Motivation

Argo CD Image Updater is a popular project which solves an important gap within the CI/CD workflow by providing the ability to update application workload images automatically (subject to appropriate constraints). It is designed around Argo CD itself and naturally compliments it. The configuration required for it revolves almost entirely around the Application resource itself. Currently the image updater uses annotations on the application resource to store configuration, which is not the preferred way to do this. It also doesn't store any state, and is, as such, unable to provide some key features that are discussed in further sections. Consolidating this configuration into the Application CR as first class fields in the spec and status is a natural next step in the Image Updater's journey. 

### Goals

- Propose updated Application CR fields to include configuration scoped for image updates
- Make a case to merge image updater controller code into core Argo CD as its own controller 
- outline future enhancements planned for image updater and it's growth within Argo CD 

### Non-Goals

TBD

## Proposal

### Use cases

#### Use case 1:
As a User of Argo CD, I would like to be able to have my application images updated automatically within expressed constraints when there is a new version available, natively within Argo CD. 

#### Use case 2:
As a user of Argo CD and Argo CD Image Updater, I would like to have image-updater honor rollbacks to my applications  

### Implementation Details/Notes/Constraints [optional]

- Argo CD Image Updater code base will need to be updated to look for application resources in namespaces other than the control-plane namespace when configured to use the k8s api instead of the argocd api to retrieve resources (see https://argocd-image-updater.readthedocs.io/en/stable/install/installation/#installation-methods). Currently it only recognizes applications present in the control plane namespace when installed in this manner. 

- Image Updater controller would need to be able to modify fields in the status of the Application CR in order to maintain state, as described in the following section.

- There could be a case made for separating image updater coniguration into its own dedicated CRD, however since the application resource is so central to the image update configurations, it would be a lot more intuitive for this configuration to belong within the application CR itself. Secondly, having a dedicated CR would mean that the number of required resources on the cluster would double since we would need to have a dedicated image update CR for each application.

- In terms of the actual merging of the codebases, the ideal approach would likely be to pull the image-updater code into the core Argo CD codebase, but keep it largely isolated as its own controller. This would seem logical since there is sufficient distinction between the purpose of the image-updater controller code and the tasks rest of the core components (like application-controller) perform that we can maintain a clean separation of concerns. It would also be easier to maintain in the long run. There is also a case that can be made to keep the code completely de-coupled in its own repository in the argoproj org since there is not as much interdependency between the 2 projects, however this may send mixed signals to the community regarding the capacity in which image updater has been granted "first class" status within Argo CD. 

### Detailed examples

Example of proposed additions to Application CR to accomodate image update configuration;

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  ...

spec:
...

  image:
    # updates contains all the fields to configure exactly which images should be considered for automatic updates and how they should be updated
    updates:
      # ImageList specifies the image specific configuration for each image that is to be considered as a candidate for updation 
      imageList:
        - path: quay.io/some/image 
          alias: someImage
          allowTags: regexp
          ignoreTags: 
          - <pattern 1>
          - <pattern 2>
          forceUpdate: true
          updateStrategy: semver
          imagePlatforms:
          - linux/amd64
          - linux/arm64 
          pullSecret: 
            namespace: <ns-name>
            secretName: <secret_name>
            field: <secret_field>
            env: <VARIABLE_NAME>
            ext: <path/to/script>
          helm:
            imageName: <name of helm parameter to set for the image name>  # for git write back
            imageTag: <name of helm parameter to set for the image tag>
            imageSpec: <name of helm parameter to set for canonical name of image>
          kustomize:
            imageName: <original_image_name> # for git write back
      
      # Application wide configuration
      allowTags: regexp
      ignoreTags: 
      - <pattern 1>
      - <pattern 2>
      forceUpdate: true
      updateStrategy: semver
      pullSecret: 
        namespace: <ns_name>
        secretName: <secret_name>
        field: <secret_field>
        env: <VARIABLE_NAME>
        ext: <path/to/script>
      writeBack:
        method: git
        target: kustomization

        # branch can contrinue supporting existing features like specifying templates for new branch creation & usage of SHA # identifiers the same way as currently uesd, for e.g    # branch: main:image-updater{{range .Images}}-{{.Name}}-{{.NewTag}}{{end}} 
        branch: <branch_name>
        secret:
          namespace: argocd
          name: git-creds
    
status:
...
  # imageUpdate stores the history of image updates that have been
  # conducted, and contains information about which image was last 
  # updated, its version and update timestamp, in order to respect 
  # application rollbacks and avoid repeat update attempts
  imageUpdates:
  - path: quay.io/some/image
    alias:  someImage
    updatedAt: <update_timestamp>
    updatedTo: v1.1.0
     
```

There is a need to introduce state by tracking the status of image Updates in the application CR. This is mainly important to honor application rollbacks, avoid repeated image updates, as well to support future use cases such as enabling image-updater to create Pull Requests against the source repositories. The above detailed `.spec.imageUpdates` field could be extended to store metadata regarding pull request creation that would serve to avoid duplicate/excessive number of pull requests being opened, for example.

There could be a few different options in terms of placement for this block of information:

1. `.status.imageUpdates` - As shown above, we could introduce a new status field at the highest level that could store a slice of `imageUpdates` containing information about the last version updated to and timestamp of last update etc. This would be a non breaking change and easy to implement/maintain/extend
2. `.status.summary.images` - Currently this field exists as a slice of strings that just contains the full qualified paths of all images associated to an application. This filed could be repurposed by changing it from `[]string` to `[]imageUpdate` to store the additional information above. This seems like a naturally better place for it, however this would be a breaking change (and at present  would also break image updater as it uses this field to get the list of potentially updateable images)
3. `.status.summary.imageUpdates` - This field could instead be introduced within the applicaiton summary with the understanding that `status.summary.images` will be deprecated and eventually removed, since it will be rendered obsolete by this new field. 

## Open Questions [optional]

Based on this proposal, the image updater controller would need to write to the application status fields in order to track the state of image updates for various images. This would mean we could potentially have 2 controllers writing to the application resource. Since the introduction of server side apply in Argo CD, it is not uncommon to have multiple field managers controlling different fields in a resource, however it may be the case that this does not extend to the status fields. In such an event, should just the responsibility of updating the application status field for image updates be shifted to the application-controller? 


### Security Considerations

* How does this proposal impact the security aspects of Argo CD workloads ?
* Are there any unresolved follow-ups that need to be done to make the enhancement more robust ?

With the rise in interest in software supply chain security across the industry, one aspect worth considering is that the introduction of image updater into Argo CD would expose Argo CD toward an entirely new type of source repository in image registries. Image Updater interfaces directly with various image registries, and as such pulls image manifests from these sources, curently trusting that the user has configured reputable image registries for their purposes. Moving forwards it would be reasonable to suggest that we could be more careful and selective about which images/registries we would want to allow Argo CD to pull images from, in order to minimize exposure to bad actors. 

One of the potential enhancements that could be introduced within Image Updater down the line would be the ability to verify that images are signed through potentially leveraging existing container verification solutions. The above proposed CR fields could be expanded to build out `.spec.image.verification` for e.g which could store specific configuration/constraints expressed by the user to ensure the trustworthiness of containers being used in their applications (for e.g storing the location of a public key to verify an image against)


### Risks and Mitigations

Introduction of Image Updater would be an addition of yet another controller in an already rather complex codebase. There is a risk of making Argo CD more resource intensive as a workload. However, following up from previous discussions conducted on this topic within the contributor's meeting, there are ways to mitigate this risk. 

A potential option that was discussed was to combine less resource intensive workloads (such as applicationsets, notifications controller and image-updater) into a single process/pod rather than have them each run as their own separate process. 


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

## Future enhancements

At present there are some initiatives that are being considered which could enhance the usability and adoption of image updater with users in the future, some of which have been mentioned in this proposal already:

1. Support for opening pull requests to source repositories and honoring rollbacks - At present image updater directly commits manifest updates to the target branch when git write-back is configured. However, if status information about the last conducted update and version updated to is available, image updater could use that information to potentially support creating pull requests (and not spam pull requests) and also not continuously try to upgrade to a new image version if an application has been rolledback.

2. Change Image updater operational model to become webhook aware - At present the updater polls the configured image registries periodically to detect new available versions of images. In the future image updater would need to be able to support webhooks so that it can move to a pub/sub model and updates can be triggered as soon as a qualifying image version is found.

3. Supporting container verification - As described earlier, this could be a useful feature to reduce security risks introduced into Argo CD while providing all the benefits of having automatic image updates available. 