// Initialize Mermaid diagrams for MkDocs pages.
// Mermaid is loaded via `extra_javascript` in `mkdocs.yml`.
//
// Notes:
// - MkDocs Material uses SPA-style navigation; use `document$` when available.
// - Avoid re-initializing Mermaid repeatedly; just re-render diagrams on navigation.
(function initMermaid() {
  if (typeof window === 'undefined') return;

  function replaceMermaidCodeBlocks() {
    // Without `pymdownx.superfences` custom_fences (which requires !!python/name tags),
    // MkDocs renders ```mermaid fences as code blocks.
    // Convert those into <div class="mermaid">...</div> so Mermaid can render them.
    // In this repo, code blocks are typically wrapped by Pygments as:
    // <div class="highlight"><pre><span></span><code>...</code></pre></div>
    //
    // Note: the mermaid fence does not necessarily preserve a `language-mermaid` class here,
    // so we detect Mermaid blocks by looking at the first meaningful token.
    const codeBlocks = document.querySelectorAll(
      'code.language-mermaid, code.mermaid, div.highlight pre > code'
    );

    const isMermaidText = (text) => {
      const t = (text || '').trimStart();
      if (!t) return false;
      // Mermaid init directive or common diagram starters
      if (t.startsWith('%%{')) return true;
      return /^(sequenceDiagram|flowchart|graph|classDiagram|stateDiagram|erDiagram|journey|gantt|gitGraph|pie|mindmap|timeline|sankey-beta|xychart-beta)\b/.test(
        t
      );
    };

    for (const code of codeBlocks) {
      // Skip if already processed
      if (code.dataset && code.dataset.mermaidProcessed === 'true') continue;

      if (!isMermaidText(code.textContent || '')) continue;

      const pre = code.closest('pre');
      if (!pre) continue;

      // If wrapped in a highlighter container, replace the wrapper to avoid leaving empty boxes.
      const wrapper = pre.parentElement && pre.parentElement.classList.contains('highlight') ? pre.parentElement : pre;

      const div = document.createElement('div');
      div.className = 'mermaid';
      // textContent preserves the original diagram text (and unescapes HTML entities).
      div.textContent = code.textContent || '';

      // Mark processed to avoid repeated replacements on navigation.
      if (code.dataset) code.dataset.mermaidProcessed = 'true';

      wrapper.replaceWith(div);
    }
  }

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
    replaceMermaidCodeBlocks();
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

