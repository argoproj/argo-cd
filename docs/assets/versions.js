setTimeout(function() {
  const callbackName = 'callback_' + new Date().getTime();
  window[callbackName] = function (response) {
  const div = document.createElement('div');
  div.innerHTML = response.html;
  document.querySelector(".md-header__inner > .md-header__title").appendChild(div);
  const container = div.querySelector('.rst-versions');
  var caret = document.createElement('div');
  caret.innerHTML = "<i class='fa fa-caret-down dropdown-caret'></i>"
  caret.classList.add('dropdown-caret')
  div.querySelector('.rst-current-version').appendChild(caret);
  div.querySelector('.rst-current-version').addEventListener('click', function() {
      const classes = container.className.split(' ');
      const index = classes.indexOf('shift-up');
      if (index === -1) {
          classes.push('shift-up');
      } else {
          classes.splice(index, 1);
      }
      container.className = classes.join(' ');
  });
  }

  var CSSLink = document.createElement('link');
  CSSLink.rel='stylesheet';
  CSSLink.href = '/assets/versions.css';
  document.getElementsByTagName('head')[0].appendChild(CSSLink);

  var script = document.createElement('script');
  script.src = 'https://argo-cd.readthedocs.io/_/api/v2/footer_html/?'+
      'callback=' + callbackName + '&project=argo-cd&page=&theme=mkdocs&format=jsonp&docroot=docs&source_suffix=.md&version=' + (window['READTHEDOCS_DATA'] || { version: 'latest' }).version;
  document.getElementsByTagName('head')[0].appendChild(script);
}, 0);

// VERSION WARNINGS
window.addEventListener("DOMContentLoaded", function() {
  var rtdData = window['READTHEDOCS_DATA'] || { version: 'latest' };
  var margin = 30;
  var headerHeight = document.getElementsByClassName("md-header")[0].offsetHeight; 
  if (rtdData.version === "latest") {
    document.querySelector("div[data-md-component=announce]").innerHTML = "<div id='announce-msg'>You are viewing the docs for an unreleased version of Argo CD, <a href='https://argo-cd.readthedocs.io/en/stable/'>click here to go to the latest stable version.</a></div>"
    var bannerHeight = document.getElementById('announce-msg').offsetHeight + margin
    document.querySelector("header.md-header").style.top = bannerHeight +"px";   
    document.querySelector('style').textContent +=
    "@media screen and (min-width: 76.25em){ .md-sidebar { height: 0;  top:"+ (bannerHeight+headerHeight)+"px !important; }}"
    document.querySelector('style').textContent +=
    "@media screen and (min-width: 60em){ .md-sidebar--secondary { height: 0;  top:"+ (bannerHeight+headerHeight)+"px !important; }}"
  }
  else if (rtdData.version !== "stable") {
    document.querySelector("div[data-md-component=announce]").innerHTML = "<div id='announce-msg'>You are viewing the docs for a previous version of Argo CD, <a href='https://argo-cd.readthedocs.io/en/stable/'>click here to go to the latest stable version.</a></div>"
    var bannerHeight = document.getElementById('announce-msg').offsetHeight + margin
    document.querySelector("header.md-header").style.top = bannerHeight +"px";   
    document.querySelector('style').textContent +=
    "@media screen and (min-width: 76.25em){ .md-sidebar { height: 0;  top:"+ (bannerHeight+headerHeight)+"px !important; }}"
    document.querySelector('style').textContent +=
    "@media screen and (min-width: 60em){ .md-sidebar--secondary { height: 0;  top:"+ (bannerHeight+headerHeight)+"px !important; }}"
  }
});
