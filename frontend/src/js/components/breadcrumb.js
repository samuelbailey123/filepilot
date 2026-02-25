// Breadcrumb — Path navigation.

import { navigateTo } from "../app.js";

export function initBreadcrumb(path) {
  var container = document.getElementById("breadcrumb");
  var parts = path.split("/").filter(Boolean);

  var html = "";

  // Root.
  html +=
    '<span class="breadcrumb-item" data-path="/">/</span>';

  var currentPath = "";
  parts.forEach(function (part, i) {
    currentPath += "/" + part;
    var isLast = i === parts.length - 1;
    var cls = isLast ? "breadcrumb-item current" : "breadcrumb-item";

    html += '<span class="breadcrumb-sep">\u203A</span>';
    html += '<span class="' + cls + '" data-path="' + escapeAttr(currentPath) + '">' + escapeHtml(part) + "</span>";
  });

  container.innerHTML = html;

  // Click handlers.
  container.querySelectorAll(".breadcrumb-item").forEach(function (item) {
    item.addEventListener("click", function () {
      navigateTo(item.dataset.path);
    });
  });
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
