# Resource Tracking

## Ability to define tracking method

>v2.2

Argo CD identifies resources it manages by setting the application instance label to the name of the managing Application on all resources that are managed (i.e. reconciled from Git). The default label used is the well-known label app.kubernetes.io/instance.



We propose introducing a new setting resourceTrackingMethod that allows to control how application resources are identified. The resourceTrackingMethod setting takes one of the following values:

* label (default) - Argo CD keep using the app.kubernetes.io/instance label.
* annotation+label - Argo CD keep adding app.kubernetes.io/instance but only for informational purposes: label is not used for tracking, value is truncated if longer than 63 characters. The argocd.argoproj.io/tracking-id annotation is used to track application resources.
* annotation - Argo CD uses the argocd.argoproj.io/tracking-id annotation to track application resources.
The app.kubernetes.io/instance attribute values includes the application name, resources identifier it is applied to, and optionally the Argo CD installation ID:

The application name allows to identify the application that manages the resource. The resource identifier prevents confusion if an operation copies the app.kubernetes.io/instance annotation to another resource. Finally optional installation ID allows separate two Argo CD instances that manages resources in the same cluster.

The resourceTrackingMethod setting should be available at the system level and the application level to allow the smooth transition from the old app.kubernetes.io/instance label to the new tracking method. Using the app leverl settings users will be able to first switch applications one by one to the new tracking method and prepare for the migration. Next system level setting can be changed to annotation or annotation+label and not-migrated applications can be configured to use labels using application level setting.



```yaml
apiVersion: v1
data:
  application.resourceTrackingMethod: annotation
kind: ConfigMap
```