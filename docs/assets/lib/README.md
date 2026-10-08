# Vendored assets

## Mermaid

- Source: https://github.com/mermaid-js/mermaid
- Version: 11.17.2
- Files:
  - mermaid-11.17.2.min.js
- License: MIT (see `LICENSE.mermaid`)
- Upstream dist URL: https://cdn.jsdelivr.net/npm/mermaid@11.17.2/dist/mermaid.min.js

This file is vendored to avoid a runtime dependency on an external CDN
(offline/CSP-friendly docs rendering). To update, download the `dist/mermaid.min.js`
of the new release, drop the old file, and update `extra_javascript` in `mkdocs.yml`
along with the version above.
