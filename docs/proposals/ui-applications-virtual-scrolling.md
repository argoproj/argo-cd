---
title: Virtual scrolling for Applications list (tiles and table)
authors:
  - "@aali309"
sponsors:
  - TBD
reviewers:
  - "@jannfis"
  - "@crenshaw-dev"
  - "@alexmt"
approvers:
  - "@jannfis"
  - "@alexmt"
  - "@crenshaw-dev"

creation-date: 2026-07-23
last-updated: 2026-07-24
---

# Virtual scrolling for Applications list (tiles and table)

This proposal improves how the Applications list behaves when someone wants to see every application at once.

## Open Questions

* Prefer reusing the existing `react-virtualized` dependency, or adopt a smaller modern library (`react-window` ~6 KB gzipped, `@tanstack/react-virtual` ~3 KB gzipped)?
* Should tiles and table ship in the same PR, or land tiles first and table second?
* Should overscan buffer size be fixed or configurable?
* Do we need a temporary feature flag, or is All-only virtualization low enough risk to ship directly?

## Summary

Argo CD already paginates the Applications list on the client.
When you pick a normal page size (**5**, **10**, **15**, or **20**), only that page is rendered, and the list stays light.

The problem starts when users choose **Items per page: All**.
In that mode today, Argo CD tries to mount a tile or table row for every application in the filtered list.
With thousands of apps, the browser creates a huge DOM, the main thread stalls, and the page can freeze or crash.

This proposal adds **virtual scrolling** for the Applications **tiles** and **table** views when **All** is selected.
The full filtered list still lives in memory (same as today), but the UI only mounts what you can see on screen, plus a small buffer around the viewport.

This is part of the continuous progressive UI scalability and improvement effort.

## Motivation

Teams that run large Argo CD hubs often manage thousands of Applications from one UI.
Day-to-day triage gets painful when the Applications page is slow or unstable.

Earlier progressive UI work (for example [#25201](https://github.com/argoproj/argo-cd/pull/25201)) helped initial load.
It did not stop the “render everything” cost when **All** is selected.

**All** is not the default page size (the default is **10**), but it is a supported workflow for scanning the full fleet without flipping pages.
At scale, that path is where rendering cost becomes an issue.

### What we measured

We compared master with a virtual-scrolling branch using this setup:

* Applications list
* **Tiles** view (table not measured yet)
* **Items per page: All**
* Local e2e UI via `make start-e2e-local` (webpack-dev on `localhost:4000`)
* Seeded apps created with a local helper script (`perf-test-app-*`, skip-reconcile) at 2k / 5k / 25k

Primary tools and KPIs:

* Chrome Lighthouse Performance diagnostics for **DOM size**, **Total Blocking Time (TBT)**, **JavaScript execution**, and **main-thread work**
* Chrome DevTools **live LCP** (and interaction feel) for paint

We intentionally do **not** treat Lighthouse Performance score or Lighthouse LCP on this SPA/webpack-dev setup as primary KPIs; those signals can be misleading here.
Numbers below are relative A/B results from the same lab machine and protocol, not absolute production SLOs.

| Scale | Without virtual scrolling | With virtual scrolling | What it means |
|-------|---------------------------|------------------------|---------------|
| 2k apps | DOM 117,059 · TBT 3,130 ms | DOM 2,798 · TBT 1,030 ms | ~42× less DOM · about 67% less blocking time |
| 5k apps | DOM 291,809 · TBT 6,390 ms | DOM 5,798 · TBT 1,280 ms | ~50× less DOM · about 80% less blocking time |
| 25k apps | Browser **crashed** (no report) | DOM 25,798 · TBT 2,930 ms · page usable | Stability: the page loads where master dies |

Live LCP from DevTools:

* 2k: **5.33 s** → **1.76 s**
* 5k: **14.73 s** → **~2.0 s**
* 25k with virtual scrolling: still slow paint (**15.60 s**), but interaction stayed good.
  At that scale the remaining cost is dominated by **data fetch + JS processing of the full catalog**, not by mounting every tile.
  The main win is **crash vs works**.

> [!NOTE]
> Virtual scrolling keeps the **list/tile** DOM roughly viewport-sized.
> Total DOM can still grow with fleet size because the **filter autocomplete** currently creates about one hidden child per application.
> That is a separate follow-up (virtualize autocomplete).

### Goals

* When **Items per page: All** is selected, mount only what is on screen (plus a little overscan), not one React tree per application
* Keep current page sizes (**5 / 10 / 15 / 20 / All**) and leave fixed page sizes on today’s fast path
* Show clear improvement on DOM and TBT at **2k** and **5k**, and keep the UI loadable at **25k** where master crashed in lab runs
* Stay compatible with today’s client-side filter, sort, and watch model (no API change required for v1)

### Non-Goals

* Server-side pagination or server-side filtering (see [server-side pagination](./server-side-pagination.md) and [#17222](https://github.com/argoproj/argo-cd/pull/17222))
* Shrinking list/watch payload size or in-browser memory for the full app catalog
* Virtualizing Applications filter autocomplete in the same change
* Changing the default page size away from **10**
* Fixing other UI surfaces outside Applications tiles/table (for example, any separate Agent / AI assistant UI)

## Proposal

### Use cases

#### Use case 1:

As a platform admin managing thousands of Applications, I want to select **Items per page: All** and scroll tiles or the table **without freezing or crashing the browser**.

#### Use case 2:

As a user who prefers page size **10** or **20**, I want **existing client-side pagination behavior to stay the same**.

### How it works today

In `paginate.tsx`, the list currently does this (simplified):

```tsx
children(pageSize === -1 ? data : data.slice(pageSize * currentPage, pageSize * (currentPage + 1)))
```

| User picks | What gets rendered | Result today |
|------------|--------------------|--------------|
| 5 / 10 / 15 / 20 | Only that page | Small DOM — fine |
| **All** (`pageSize === -1`) | Every filtered app | One tile/row per app — the real issue |

### Proposed behavior

| User picks | Behavior |
|------------|----------|
| **5 / 10 / 15 / 20** | Keep current slice + render (unchanged) |
| **All** | Pass the full filtered list into a virtualized tiles grid / table body, and mount only about 25–35 visible items (plus overscan) |

### Implementation Details/Notes/Constraints

Likely touch points:

| Area | Role |
|------|------|
| `ui/.../paginate/paginate.tsx` | Detect `pageSize === -1` and optionally tell children they are in virtual mode |
| `applications-tiles.tsx` / tile components | Virtualized grid when All is selected |
| `applications-table.tsx` / row components | Virtualized list when All is selected |
| UI `package.json` | Prefer reusing existing `react-virtualized`, or add a smaller maintained alternative if needed |

Constraints to keep in mind:

* Watch updates still update the in-memory `apps[]` list; only visible rows/tiles should re-render heavily
* Tile/row height should be stable enough for windowing (or use dynamic measurement carefully)
* Accessibility: keep keyboard navigation as practical as possible with recycled rows
* Scroll restoration / deep links: selecting an application by name/URL should still work; scroll position on browser back is best-effort and should be called out in the implementation PR if limited
* No Kubernetes or API server changes for v1

A work-in-progress implementation may land on a feature branch or PR after this design is accepted.
The lab numbers above came from a local virtual-scrolling branch compared with master.

### Test plan

* **All** + tiles: scroll smoothly at 2k and 5k; page remains usable at 25k in lab
* Fixed page sizes **5 / 10 / 15 / 20**: behavior and DOM remain unchanged
* Filter / sort while **All** is selected still works with virtualized rendering
* Keyboard navigation smoke test on virtualized tiles and table
* Optional: re-measure on a production UI build; measure table view at least at one scale

### Detailed examples

**Before (All @ 2k apps):** about 117k DOM nodes and about 3.1 s TBT; live LCP about 5.3 s.

**After (All @ 2k apps with virtual scrolling):** about 2.8k DOM nodes and about 1.0 s TBT; live LCP about 1.8 s.
Autocomplete still contributes about 2k hidden children.

**At 25k apps:** master crashes; with virtual scrolling the page loads (DOM about 26k, mostly from autocomplete children).

### Security Considerations

* This is a UI-only rendering change.
* It does not add new trust boundaries, secret handling, or API surface.
* RBAC stays the same: the client still only shows applications the user can already list or watch.
* Sync and admission paths are unchanged.

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| People assume “UI performance is solved” while fetch and memory still scale with N | Frame this as a progressive UI fix; keep server-side pagination and payload work on the roadmap |
| Autocomplete still grows DOM with fleet size | Explicit follow-up to virtualize filter autocomplete |
| Keyboard navigation or accessibility regressions | Manual test plan plus unit tests around virtual list helpers |
| Extra library / bundle cost | Prefer existing `react-virtualized`, or a small alternative (~3–6 KB gzipped) and measure UI bundle impact |
| Lab metrics from webpack-dev mislead reviewers | Document the methodology; optionally re-run on a production UI build |

### Upgrade / Downgrade Strategy

* **Upgrade:** No operator or API config is required.
  Existing page-size preferences keep working.
  Once shipped, selecting **All** automatically uses virtualized rendering.
* **Downgrade / revert:** Revert the UI change and **All** goes back to mounting every tile/row.
* No CRD, ConfigMap, or CLI migration.

## Drawbacks

* This does not reduce network transfer or JS heap for the full application catalog.
  Large hubs can still feel heavy for reasons outside list DOM.
* It adds some UI complexity (windowing, scroll container sizing, empty/filter edge cases).
* Until autocomplete is virtualized, total DOM under virtual scrolling still grows roughly with fleet size.
* Benefits are strongest for **All**; default page size **10** already avoids the worst list-render path.

## Alternatives

1. **Do nothing / tell users not to use All** — leaves a supported workflow broken at scale.
2. **Remove the All page size** — breaks existing preferences; removes the capability instead of fixing it.
3. **Wait only for server-side pagination** — right long-term direction, but a larger effort.
   Progressive UI wins can ship sooner in parallel ([#17222](https://github.com/argoproj/argo-cd/pull/17222)).
4. **Replace All with infinite scroll** — bigger UX change than needed to fix the All bottleneck.
5. **Virtualize even page size 10** — unnecessary complexity where slicing already keeps DOM small.

## Related work

* UI scalability / server-side pagination tracking: [#14947](https://github.com/argoproj/argo-cd/issues/14947), related filter work [#15087](https://github.com/argoproj/argo-cd/issues/15087)
* Server-side pagination: [server-side-pagination.md](./server-side-pagination.md), [#17222](https://github.com/argoproj/argo-cd/pull/17222)
* Earlier UI render improvement: [#25201](https://github.com/argoproj/argo-cd/pull/25201)
