// Render Mermaid `.mermaid` blocks. mkdocs-material 7.1.8 has no built-in
// Mermaid support, and we follow the light/dark palette by re-rendering on
// toggle (mermaid replaces the source with an SVG, so we cache the source).
(function () {
  if (!window.mermaid || typeof window.mermaid.initialize !== "function") {
    return;
  }

  // Disable mermaid's startOnLoad auto-render so we control rendering below.
  window.mermaid.initialize({ startOnLoad: false });

  var blocks = Array.prototype.slice.call(
    document.querySelectorAll(".mermaid")
  );
  if (!blocks.length) {
    return;
  }

  // Cache each source before any render replaces it with an SVG.
  blocks.forEach(function (el) {
    el.setAttribute("data-mermaid-src", el.textContent);
  });

  function currentTheme() {
    return document.body.getAttribute("data-md-color-scheme") === "slate"
      ? "dark"
      : "default";
  }

  function render(theme) {
    blocks.forEach(function (el) {
      el.textContent = el.getAttribute("data-mermaid-src");
      el.removeAttribute("data-processed");
    });
    window.mermaid.initialize({ startOnLoad: false, theme: theme });
    if (typeof window.mermaid.run === "function") {
      window.mermaid.run({ nodes: blocks });
    } else if (typeof window.mermaid.init === "function") {
      window.mermaid.init(undefined, blocks);
    }
  }

  function start() {
    render(currentTheme());
    document
      .querySelectorAll('input[name="__palette"]')
      .forEach(function (input) {
        input.addEventListener("change", function () {
          // Defer so Material updates `data-md-color-scheme` first.
          window.setTimeout(function () {
            render(currentTheme());
          }, 0);
        });
      });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
