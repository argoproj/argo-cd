# Application query search

Use the search field above the Applications list to combine conditions. Press Enter or choose "Search" to apply the query. The sidebar contains navigation, with no application filter controls.

```text
sync = OutOfSync OR health = Degraded
```

This query includes apps with either status, including synced apps that are degraded. An app matching both appears once.

## Operators and groups

Use `AND`, `OR`, `NOT` and parentheses. Lowercase and mixed case work too, such as `and`, `or` and `Not`. `NOT` takes priority over `AND`, and `AND` over `OR`. Keywords and value comparisons ignore case. Metadata keys retain their original case.

Omitting the operator between conditions implies `AND`. These queries give the same results:

```text
sync = OutOfSync health != Healthy
sync = OutOfSync and health != Healthy
```

The same rule applies before a group or `NOT`. For example, `(sync = OutOfSync or health = Degraded) namespace = production` requires the production namespace for either status.

```text
(sync = OutOfSync OR health IN (Degraded, Missing)) AND project = production
```

| Operator | Meaning | Example |
| --- | --- | --- |
| `=` | Exact value | `project = production` |
| `!=` | Exclude an exact value | `health != Healthy` |
| `~` | Value contains text | `name ~ payments` |
| `!~` | Value does not contain text | `repo !~ archived` |
| `IN` | Match any value in the list | `health IN (Degraded, Missing, Unknown)` |
| `NOT IN` | Exclude every listed value | `namespace NOT IN (dev, test)` |
| `NOT` | Negate a condition or group | `NOT (health = Healthy AND sync = Synced)` |

Quote values containing spaces or query punctuation. Both quote styles work. Escape a quote or a backslash with a backslash. For status values, spaces, hyphens and underscores are optional, so `sync = "Out of Sync"` also works.

Quoted values also support `\n`, `\r`, `\t`, `\b`, `\f` and four-digit Unicode escapes such as `\u0000`. Suggestions use these escapes for control characters so multiline annotations fit in the search field. Use `\\` for a literal backslash, for example `annotation.note = "literal\\n"` to match the characters `\n` instead of a newline.

Plain text searches the application name and the namespace containing its Application resource. For example, `payments` finds names containing that text. Use `namespace ~ payments` to search destination namespaces instead. This language is a subset inspired by JQL; Jira functions and regular expressions are not supported.

## Fields

The search field suggests fields, operators, status values and the next part of a query as you type. Use the arrow keys to choose a suggestion, then Enter or Tab to insert it. Tab also inserts the first suggestion when none is selected. Escape closes suggestions. Enter with no selected suggestion applies the query.

Suggestions follow the cursor, so you can also edit the middle of a query. Choosing a suggestion preserves the surrounding conditions and quotes values when needed. Names, projects, namespaces, repositories, revisions and metadata values come from all loaded applications you can access, including apps outside the current results. You can still enter a value that is not suggested.

| Field | Value |
| --- | --- |
| `name` | Application name |
| `namespace` | Destination namespace |
| `appNamespace` | Namespace containing the Application resource |
| `project` | Argo CD project |
| `cluster` | Destination cluster name or server URL stored on the Application |
| `repo`, `revision` | Repository URL or target revision from any source |
| `sync`, `health` | Current sync or health status |
| `operation` | Operation title, such as `"Sync failed"` or `Syncing` |
| `hydration` | Hydration state, such as `Failed`, `Hydrated` or `None` |
| `autoSync` | `Enabled` or `Disabled` |
| `favorite` | `true` or `false`, using your saved favorites |
| `label.KEY` | Label value, such as `label.team = platform` |
| `annotation.KEY` | Annotation value, such as `annotation.example.com/owner = "Alice & Bob"` |

For multiple sources, a positive comparison matches if any source matches. A negative comparison matches only if no source matches. A missing field does not match a positive comparison, including an empty quoted value; it does match a negative comparison.

## Share and restore a search

Choose "Copy link" to apply the current valid query and copy a link containing the `q` parameter. Opening that link fills the search field and applies it. Browser back and forward restore earlier applied queries. "Clear" removes the query and shows all applications available to you.

```text
/applications?q=sync%20%3D%20OutOfSync%20OR%20health%20%3D%20Degraded
```

Saved sidebar preferences and old sidebar query parameters do not restrict the new search. Put all conditions in `q`. The old `search` parameter remains a search entry point when `q` is absent.

Results reflect the current application statuses and the viewer's permissions. The same link can show different apps if access differs or statuses change. The `favorite` field uses the viewer's own favorites.

Invalid queries show a message with a character position. A malformed query opened from a link matches no applications. Invalid drafts cannot be applied or copied. Queries are limited to 4096 characters, 512 tokens and 32 levels of nesting.
