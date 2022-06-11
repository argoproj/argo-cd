# Selective Sync

A *selective sync* is one where only some resources are sync'd. You can choose which resources from the UI:

![selective sync](../assets/selective-sync.png)

When doing so, bear in mind:

* Your sync is not recorded in the history, and so rollback is not possible.
* Hooks are not run.

## Selective Sync Option

>v1.8

Turning on selective sync option which will sync only out-of-sync resources.
See [sync options](sync-options.md#selective-sync) documentation for more details.
