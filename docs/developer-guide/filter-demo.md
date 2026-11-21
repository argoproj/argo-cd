# Local application filter demo

This demo runs the real Argo CD UI against a small read-only fixture API. It does not start Argo CD controllers or connect to Kubernetes. The banner identifies the sample data. Create, sync, delete and other write requests return HTTP 405.

## Start

Install the UI dependencies and start the demo from the repository root. Use the pnpm version declared in `ui/package.json`.

```sh
cd ui
corepack pnpm install --frozen-lockfile
node scripts/filter-demo.cjs
```

Open <http://127.0.0.1:4000/applications?q=sync%20%3D%20OutOfSync%20OR%20health%20%3D%20Degraded>.

The UI listens on `127.0.0.1:4000`. Its fixture API listens on `127.0.0.1:18080`. Change `DEMO_UI_PORT` and `DEMO_API_PORT` if those ports are in use. Press Ctrl+C in the terminal to stop both processes.

## Browser checks

The sample contains six applications. Enter each query in the search field and press Enter.

| Selection | Expected applications |
| --- | --- |
| Empty query | All six |
| `sync = OutOfSync` | demo-outofsync, demo-both |
| `sync = OutOfSync OR health = Degraded` | demo-outofsync, demo-degraded, demo-both, demo-staging-degraded |
| `sync = OutOfSync AND health = Degraded` | demo-both |
| `sync = OutOfSync health = Degraded` | demo-both |
| `sync = OutOfSync or health = Degraded` | demo-outofsync, demo-degraded, demo-both, demo-staging-degraded |
| `(sync = OutOfSync OR health = Degraded) AND namespace = production` | demo-outofsync, demo-degraded, demo-both |
| `name = demo-degraded` | demo-degraded |

Check that no application filters appear in the sidebar. Copy a query link, open it in another tab and verify that the input and results match. Reload and use browser back and forward to verify query restoration. Enter `health =` and check the visible validation message and disabled Search and Copy link buttons. Clear the search and verify that all six applications return.

Type `sy`, press Tab and verify `sync ` is inserted. Press Arrow Down and Enter to insert `=`, then type `out` and press Tab to insert `OutOfSync`. Append `health = deg`, choose `Degraded` and apply. Only demo-both should remain. Type a space after a complete condition to see AND, OR and fields for the next condition. Check Escape, Shift Tab, mouse selection and completion in the middle of a query. The rest of the query must stay intact.

## Automated UI checks

Run these commands from `ui`.

```sh
node node_modules/typescript/bin/tsc --noEmit --project src/app
node node_modules/eslint/bin/eslint.js --ignore-pattern '.scratch/**'
node node_modules/jest/bin/jest.js --runInBand --coverage=false
```

Jest requires `ui/tsconfig.json` to be a symlink to `src/app/tsconfig.json`. On Windows, use a checkout that preserves symlinks, or run the checks in WSL with the repository on its Linux filesystem. A regular file containing only the symlink target is not a valid TypeScript configuration. The `.scratch` exclusion only omits local downloaded tools and logs.

To build the production UI in PowerShell, run the following commands from `ui`.

```powershell
$env:NODE_ENV = 'production'
node node_modules/webpack/bin/webpack.js --config ./src/app/webpack.config.js --mode production
```

For a complete Argo CD deployment with a real Kubernetes API, follow [Running Argo CD locally](running-locally.md). The demo does not replace that integration check.
