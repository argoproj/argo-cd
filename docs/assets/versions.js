const targetNode = document.querySelector('.md-header__inner');
const observerOptions = {
  childList: true,
  subtree: true
};

const VERSION_REGEX = /\/en\/(release-(?:v\d+|[\d\.]+|\w+)|latest|stable)\//;

const observerCallback = function(mutationsList, observer) {
  for (let mutation of mutationsList) {
    if (mutation.type === 'childList') {
      const titleElement = document.querySelector('.md-header__inner > .md-header__title');
      if (titleElement) {
        initializeVersionDropdown();
        observer.disconnect();
      }
    }
  }
};

const observer = new MutationObserver(observerCallback);
observer.observe(targetNode, observerOptions);

function getCurrentVersion() {
  const currentVersion = window.location.href.match(VERSION_REGEX);
  if (currentVersion && currentVersion.length > 1) {
    return currentVersion[1];
  }
  return null;
}

function initializeVersionDropdown() {
  const callbackName = 'callback_' + new Date().getTime();
  window[callbackName] = function(response) {
    const div = document.createElement('div');
    div.innerHTML = response.html;
    const headerTitle = document.querySelector(".md-header__inner > .md-header__title");
    if (headerTitle) {
      headerTitle.appendChild(div);
    }

    const container = div.querySelector('.rst-versions');
    if (!container) return; // Exit if container not found

    // Add caret icon
    var caret = document.createElement('div');
    caret.innerHTML = "<i class='fa fa-caret-down dropdown-caret'></i>";
    caret.classList.add('dropdown-caret');
    const currentVersionElem = div.querySelector('.rst-current-version');
    if (currentVersionElem) {
      currentVersionElem.appendChild(caret);
    }

    // Add click listener to toggle dropdown
    if (currentVersionElem && container) {
      currentVersionElem.addEventListener('click', function() {
        container.classList.toggle('shift-up');
      });
    }

    // Sorting Logic
    sortVersionLinks(container);
  };

  // Load CSS
  var CSSLink = document.createElement('link');
  CSSLink.rel = 'stylesheet';
  CSSLink.href = '/assets/versions.css';
  document.getElementsByTagName('head')[0].appendChild(CSSLink);

  // Load JSONP Script
  var script = document.createElement('script');
  const currentVersion = getCurrentVersion();
  script.src = 'https://argo-cd.readthedocs.io/_/api/v2/footer_html/?' +
      'callback=' + callbackName + '&project=argo-cd&page=&theme=mkdocs&format=jsonp&docroot=docs&source_suffix=.md&version=' + (currentVersion || 'latest');
  document.getElementsByTagName('head')[0].appendChild(script);
}

// Function to sort version links
function sortVersionLinks(container) {
  // Find all <dl> elements within the container
  const dlElements = container.querySelectorAll('dl');

  dlElements.forEach(dl => {
    const ddElements = Array.from(dl.querySelectorAll('dd'));

    // Check if ddElements contain version links
    const isVersionDl = ddElements.some(dd => {
      const link = dd.querySelector('a');
      return VERSION_REGEX.test(link?.getAttribute?.('href'));
    });

    // This dl contains version links, proceed to sort
    if (isVersionDl) {
      // Define sorting criteria
      ddElements.sort((a, b) => {
        const aText = a.textContent.trim().toLowerCase();
        const bText = b.textContent.trim().toLowerCase();

        // Prioritize 'latest' and 'stable'
        if (aText === 'latest') return -1;
        if (bText === 'latest') return 1;
        if (aText === 'stable') return -1;
        if (bText === 'stable') return 1;

        // Extract version numbers (e.g., release-2.9)
        const aVersionMatch = aText.match(/release-(\d+(\.\d+)*)/);
        const bVersionMatch = bText.match(/release-(\d+(\.\d+)*)/);

        if (aVersionMatch && bVersionMatch) {
          const aVersion = aVersionMatch[1].split('.').map(Number);
          const bVersion = bVersionMatch[1].split('.').map(Number);

          for (let i = 0; i < Math.max(aVersion.length, bVersion.length); i++) {
            const aNum = aVersion[i] || 0;
            const bNum = bVersion[i] || 0;
            if (aNum > bNum) return -1;
            if (aNum < bNum) return 1;
          }
          return 0;
        }

        // Fallback to alphabetical order
        return aText.localeCompare(bText);
      });

      // Remove existing <dd> elements
      ddElements.forEach(dd => dl.removeChild(dd));

      // Append sorted <dd> elements
      ddElements.forEach(dd => dl.appendChild(dd));
    }
  });
}

// VERSION WARNINGS
window.addEventListener("DOMContentLoaded", function() {
  var margin = 30;
  var headerHeight = document.getElementsByClassName("md-header")[0].offsetHeight;
  const currentVersion = getCurrentVersion();
  if (currentVersion && currentVersion !== "stable") {
    if (currentVersion === "latest") {
      document.querySelector("div[data-md-component=announce]").innerHTML = "<div id='announce-msg'>You are viewing the docs for an unreleased version of Argo CD, <a href='https://argo-cd.readthedocs.io/en/stable/'>click here to go to the latest stable version.</a></div>";
    } else {
      document.querySelector("div[data-md-component=announce]").innerHTML = "<div id='announce-msg'>You are viewing the docs for a previous version of Argo CD, <a href='https://argo-cd.readthedocs.io/en/stable/'>click here to go to the latest stable version.</a></div>";
    }
    var bannerHeight = document.getElementById('announce-msg').offsetHeight + margin;
    document.querySelector("header.md-header").style.top = bannerHeight + "px";
    document.querySelector('style').textContent +=
        "@media screen and (min-width: 76.25em){ .md-sidebar { height: 0;  top:" + (bannerHeight + headerHeight) + "px !important; }}";
    document.querySelector('style').textContent +=
        "@media screen and (min-width: 60em){ .md-sidebar--secondary { height: 0;  top:" + (bannerHeight + headerHeight) + "px !important; }}";
  }
});
