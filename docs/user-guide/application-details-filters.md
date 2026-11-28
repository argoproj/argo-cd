# Resource Filters

The Application details page has a resource filters sidebar for narrowing down the resource tree, list, and network views. It filters on:

- **NAME** — resource name, supports `*` wildcards.
- **KINDS** — resource kind (e.g. `Pod`, `Deployment`).
- **SYNC STATUS** — `Synced` / `OutOfSync`.
- **HEALTH STATUS** — `Progressing`, `Suspended`, `Healthy`, `Degraded`, `Missing`, `Unknown`.
- **NAMESPACES** — only shown when the Application spans more than one namespace.

Selected filters are reflected in the URL (`?resource=...`) so they can be shared or bookmarked, and are also remembered across sessions.

## Excluding kinds

By default, selecting one or more kinds in the **KINDS** filter shows only resources of those kinds. Once at least one kind is selected, an **Include** / **Exclude** switch appears below the KINDS input. Choose **Exclude** to invert the selection: the selected kinds are hidden, and every other kind stays visible. Excluded kinds are shown with a red cross and a struck-through name.

For example, to hide `Pod` and `ReplicaSet` resources while keeping everything else visible, select `Pod` and `ReplicaSet` in the KINDS filter, then choose **Exclude**.

Clearing the KINDS selection, or using **CLEAR ALL**, switches the filter back to **Include**.

The [Resources view](resources-view.md) has the same **Include** / **Exclude** switch on its KINDS filter.

> [!NOTE]
> Only the KINDS filter can exclude values. NAME, SYNC STATUS, HEALTH STATUS, and NAMESPACES only support inclusion.
