# Application Resource Tree

The resource tree on the Application details page shows the resources an Application manages and the
resources those own, so you can follow a `Deployment` down to its `ReplicaSet` and its `Pod`s.

## Large Applications

Drawing every resource of a very large Application is expensive: the graph is laid out in your browser, and
the cost grows faster than the number of resources. Past a few hundred resources the tree draws a bounded
part of the Application instead of all of it, and tells you what it left out.

Nothing is hidden silently. When the tree is showing part of an Application you will see:

- **A summary card** next to the Application node, reading, for example, `Showing 200 of 4000 resources`,
  with a breakdown by kind. Selecting a kind shows only that kind.
- **Overflow markers** — small nodes reading `50 more` beneath a parent whose children did not all fit, or
  beneath a kind. A marker summarises the health and sync status of what it is holding, so you can tell
  whether it is worth opening: a marker holding only healthy, synced resources rarely is.
- **Kind nodes** — where an Application has a great many resources of one kind, that kind gets a node of
  its own carrying the total, rather than those resources competing for the top level with your workloads.

### What is kept

The resources most likely to need attention are kept first: degraded and missing, then progressing, then
out-of-sync, then suspended, then workloads, then the resources that expose them, then everything
else. Ties are broken by name.

An Application small enough to draw completely is unaffected, and keeps the ordering it has always had.

### Seeing more

- **Select an overflow marker** to load more of what it holds. Each selection loads a further batch, up to a
  limit beyond which the tree will not grow — at that point the marker says so rather than offering a
  selection that cannot be honoured.
- **Search or filter** to find a specific resource. Filtering is not limited to what is currently drawn: a
  resource matching your filter is drawn even if it was not among those shown before, so searching for a
  resource by name will find it.
- **Select a kind node**, or a kind in the summary card, to show only that kind.

!!! note

    The number of resources drawn, and the size of each batch, are chosen to keep the tree responsive rather
    than to be a stable interface. Do not depend on the specific figures.

## Expanding and collapsing

Each parent with children carries a control to fold its children away, showing how many are folded.

The controls in the toolbar move one level at a time: **+** shows one more level of children across the
tree, **−** hides the deepest level currently showing. Repeated selections walk the tree down and back up
one level per selection.

In the network view, where parents come from network relationships rather than ownership, these controls
expand and collapse the whole tree at once.
