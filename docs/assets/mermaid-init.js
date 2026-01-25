// Initialize Mermaid diagrams for MkDocs pages.
// Mermaid is loaded via `extra_javascript` in `mkdocs.yml`.
//
// Notes:
// - MkDocs Material uses SPA-style navigation; use `document$` when available.
// - Avoid re-initializing Mermaid repeatedly; just re-render diagrams on navigation.
(function initMermaid() {
  if (typeof window === 'undefined') return;

  function render() {
    if (!window.mermaid || !window.mermaid.run) return;
    if (!document.querySelector('.mermaid')) return;
    try {
      window.mermaid.run({ querySelector: '.mermaid' });
    } catch (e) {
      // Best-effort; never break docs rendering.
    }
  }

  function initOnce() {
    if (!window.mermaid || window.__argoMermaidInitialized) return;
    window.__argoMermaidInitialized = true;
    try {
      // `startOnLoad: false` because we explicitly render on load/navigation.
      window.mermaid.initialize({ startOnLoad: false });
    } catch (e) {
      // ignore
    }
  }

  function onReady() {
    initOnce();
    render();
  }

  // Initial page load
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', onReady, { once: true });
  } else {
    onReady();
  }

  // MkDocs Material SPA navigation (preferred when available)
  if (window.document$ && typeof window.document$.subscribe === 'function') {
    window.document$.subscribe(function () {
      initOnce();
      render();
    });
  } else {
    // Fallback: try common navigation events
    window.addEventListener('popstate', onReady);
    window.addEventListener('hashchange', onReady);
  }
})();

