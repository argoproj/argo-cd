# Selective Sync

A _selective sync_ (also known as a _partial sync_ or _partial synchronization_ in the UI) is one where only some resources are sync'd. You can choose which resources from the UI or the CLI:

![selective sync](../assets/selective-sync.png)

When doing so, bear in mind that:

- Your sync is **not** recorded in the history, and so rollback is not possible.
- [Hooks](sync-waves.md) are **not** run.
