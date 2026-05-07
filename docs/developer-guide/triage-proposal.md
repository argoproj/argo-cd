# Argo CD triage proposal (draft)

This page is a Markdown snapshot of the maintainer proposal for evolving how Argo CD triages issues and PRs. The living document is the
[Google Doc](https://docs.google.com/document/d/1lRMgy0vKdRv22iFP53iOtIbKCnhhuAM_mzFexPYnYFQ/edit)
(authors and revision history appear there).

**Authors :** Leonardo Luz Almeida and Reggie Voloshin  
**Status:** Proposal for discussion - not yet merged into the canonical [Code Contribution Guide](code-contributions.md).

## Overview

Because Argo CD handles critical production workloads with limited maintainer availability, every new feature, enhancement, and in some cases even bug fixes must be vetted for impact on compatibility, performance, scalability, stability, and security.

Given limited availability, maintainers need to focus on high-priority work while still giving the community a sense of release direction.

## Goals

Establish a guideline using an issue-first approach. The intent is to protect the codebase, prioritize maintainer work, and support roadmap planning:

* **No orphan PRs:** PRs should be associated with a GitHub issue that maintainers approved during triage.
* **Risk mitigation:** Triage checks that the idea matches project strategy so contributors and reviewers do not spend time on work that may be rejected.
* **Transparency:** Triage is performed openly on GitHub; the community is encouraged to participate.
* **Order:** Issues are generally triaged in First-In-First-Out (FIFO) order. Regressions or issues materially impacting core user workflows may take priority.
* **Rotation:** Triage is handled by two Argo CD maintainers rotating weekly; rotation is defined in the contributor meeting.
* **Prioritization:** Triage gives maintainers visibility into requested features and bug fixes and supports clear prioritization of new issues.

In discussion on the source doc, maintainers described this as a **guideline**, not rigid rules: trivial fixes and documentation-only changes are typical exceptions where strict issue-first process adds friction without much benefit.

## Proposal

### Status triage

New issues are managed via the
[triage GitHub project board](https://github.com/orgs/argoproj/projects/37/views/2)
by setting **Status** to one of:

* **To triage:** Default when issues are first created.
* **In discussion:** Under maintainer review; may include asking the reporter to attend a contributor meeting to present the issue.
* **Needs proposal:** Substantial behavior change or unclear upgrade/downgrade paths; a design proposal must be presented and approved before the issue can move to **Ready**.
* **Ready:** The proposal (or issue, if no proposal is required) is approved; implementation may begin by the creator or any community member.
* **Declined:** Rejected; the issue should state why (for example breaking change or strategic misalignment).

### Priority triage

GitHub issues are prioritized with a **Priority** label (for example Critical, High, Medium, Low). Existing labels may need to be adjusted over time, as they have historically focused on bug severity rather than features.

Maintainers should use priority to focus PR reviews and their own contributions.

### Triage team responsibilities

* Aim for all issues opened in the current week to reach **Ready** or **Declined**. Items in **Needs proposal** can be discussed again at the next contributor meeting once a proposal exists, or asynchronously among maintainers.
* If a major issue appears during triage (serious regression or broad user impact), the rotating triage team should help get a maintainer or volunteer assigned to fix it.
* Aim for newly triaged issues to have a priority label.
* Because many older issues are untriaged, the team may optionally pick a few additional backlog issues (for example by comment or reaction count).
* Identify **good first issue** candidates.
* Optionally evaluate size (for example Small / Medium / Large); this was debated in the source doc and may be introduced gradually.
* Add high-priority issues to the next milestone when appropriate (they can be moved forward if plans change).

### Contributor meeting

The weekly contributor meeting is the primary venue for live triage and for resolving **In discussion** status.

* **When:** Every Thursday at 8:15 AM Pacific Time (see [Code Contribution Guide — Regular contributor meeting](code-contributions.md#regular-contributor-meeting) for the agenda link and timezone helper).
* **How to participate:** Add your item to the agenda document in advance if you want it discussed.

The contributor meeting is repurposed so triage drives more of the agenda, reducing ad-hoc “please review my PR” dynamics.

## Open questions (from source discussion)

The Google Doc thread raised topics for after direction is agreed, including: backlog and FIFO (thousands of old issues), automation for existing PRs, and how much the meeting agenda should be owned by the weekly triage team. Those are not resolved in this snapshot; see the [Google Doc](https://docs.google.com/document/d/1lRMgy0vKdRv22iFP53iOtIbKCnhhuAM_mzFexPYnYFQ/edit) for the latest.
