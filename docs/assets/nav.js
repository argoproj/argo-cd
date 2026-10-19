/* Sidebar wayfinding helpers for the docs theme (mkdocs-material 7.x):
   - scroll the current page into view inside the sidebar on load
   - mark the current page for assistive tech (aria-current)
   - make collapsible sections operable from the keyboard */
(function () {
  'use strict';

  // The current page's visible link: an <a> on desktop and in the tablet
  // drawer, a <label> (opening the page's table of contents) in the drawer
  // below 60em, where the <a> is hidden.
  function activeLink() {
    var links = document.querySelectorAll('.md-sidebar--primary li.md-nav__item--active > .md-nav__link--active');
    for (var i = 0; i < links.length; i++) {
      if (links[i].offsetHeight > 0) { return links[i]; }
    }
    return null;
  }

  // Nearest scrolling ancestor: the sidebar on desktop, the section panel's
  // list in the drawer. Chosen by overflow, not by current size, so a panel
  // that happens to fit is not skipped in favour of a hidden parent panel.
  function scrollParent(el) {
    var node = el.parentElement;
    while (node && node !== document.body) {
      var overflow = window.getComputedStyle(node).overflowY;
      if (overflow === 'auto' || overflow === 'scroll') { return node; }
      node = node.parentElement;
    }
    return null;
  }

  function revealActive() {
    var active = activeLink();
    var wrap = active && scrollParent(active);
    if (!active || !wrap) { return; }
    var w = wrap.getBoundingClientRect();
    var a = active.getBoundingClientRect();
    var stickyRows = wrap.querySelectorAll('li.md-nav__item--nested.md-nav__item--active > label.md-nav__link').length;
    var topPad = stickyRows * 27 + 8; // pinned section labels
    if (a.top < w.top + topPad || a.bottom > w.bottom) {
      wrap.scrollTop += (a.top - w.top) - Math.max(topPad, w.height * 0.4);
    }
  }

  function markCurrent() {
    var links = document.querySelectorAll('.md-sidebar--primary li.md-nav__item--active > .md-nav__link--active');
    Array.prototype.forEach.call(links, function (link) {
      if (!link.hasAttribute('aria-current')) { link.setAttribute('aria-current', 'page'); }
    });
  }

  function keyboardToggles() {
    var labels = document.querySelectorAll('.md-sidebar--primary label.md-nav__link[for]');
    Array.prototype.forEach.call(labels, function (label) {
      var box = document.getElementById(label.htmlFor);
      if (!box || box.type !== 'checkbox') { return; }
      label.setAttribute('tabindex', '0');
      label.setAttribute('role', 'button');
      var sync = function () { label.setAttribute('aria-expanded', box.checked ? 'true' : 'false'); };
      sync();
      box.addEventListener('change', sync);
      label.addEventListener('keydown', function (e) {
        if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
          e.preventDefault();
          box.checked = !box.checked;
          box.dispatchEvent(new Event('change', { bubbles: true }));
        }
      });
    });
  }

  function init() {
    markCurrent();
    keyboardToggles();
    revealActive();
    // the theme sizes the sidebar after load; re-check once it has
    window.addEventListener('load', function () {
      window.requestAnimationFrame(revealActive);
      window.setTimeout(revealActive, 250);
    });
    // Re-check when the drawer opens: the viewport may have crossed 76.25em
    // since load, in which case only the desktop sidebar was scrolled.
    var drawer = document.getElementById('__drawer');
    if (drawer) {
      drawer.addEventListener('change', function () {
        if (drawer.checked) { window.requestAnimationFrame(revealActive); }
      });
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
