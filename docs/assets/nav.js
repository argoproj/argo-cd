/* Sidebar wayfinding helpers for the docs theme (mkdocs-material 7.x):
   - scroll the current page into view inside the sidebar on load
   - mark the current page for assistive tech (aria-current)
   - make collapsible sections operable from the keyboard */
(function () {
  'use strict';

  function sidebar() {
    return document.querySelector('.md-sidebar--primary .md-sidebar__scrollwrap');
  }

  function revealActive() {
    var wrap = sidebar();
    var active = wrap && wrap.querySelector('a.md-nav__link--active');
    if (!wrap || !active) { return; }
    var w = wrap.getBoundingClientRect();
    var a = active.getBoundingClientRect();
    var stickyRows = wrap.querySelectorAll('li.md-nav__item--nested.md-nav__item--active > label.md-nav__link').length;
    var topPad = stickyRows * 27 + 8; // pinned section labels
    if (a.top < w.top + topPad || a.bottom > w.bottom) {
      wrap.scrollTop += (a.top - w.top) - Math.max(topPad, w.height * 0.4);
    }
  }

  function markCurrent() {
    var active = document.querySelector('.md-sidebar--primary a.md-nav__link--active');
    if (active && !active.hasAttribute('aria-current')) { active.setAttribute('aria-current', 'page'); }
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
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
