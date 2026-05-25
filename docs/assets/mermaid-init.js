// Initialize Mermaid diagrams. mkdocs-material 7.1.8 predates the theme's
// built-in Mermaid support, so we load mermaid (see extra_javascript) and
// render the `.mermaid` blocks emitted by the pymdownx superfences custom fence.
window.addEventListener("load", function () {
  if (!window.mermaid || typeof window.mermaid.initialize !== "function") {
    return;
  }
  window.mermaid.initialize({ startOnLoad: false });
  var blocks = document.querySelectorAll(".mermaid");
  if (typeof window.mermaid.run === "function") {
    window.mermaid.run({ nodes: blocks });
  } else if (typeof window.mermaid.init === "function") {
    window.mermaid.init(undefined, blocks);
  }
});
