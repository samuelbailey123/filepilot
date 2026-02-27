// Breadcrumb — Path navigation with click-to-edit inline path input.

import { navigateTo } from "../app.js";
import * as api from "../services/api.js";

var _isEditing = false;

/**
 * Render the breadcrumb path navigation.
 * Clicking the breadcrumb area switches to an editable text input.
 * @param {string} path - The current directory path.
 */
export function initBreadcrumb(path) {
  var container = document.getElementById("breadcrumb");
  _isEditing = false;
  renderBreadcrumbs(container, path);
}

/**
 * Render clickable breadcrumb segments.
 * @param {HTMLElement} container - The breadcrumb container.
 * @param {string} path - The current directory path.
 */
function renderBreadcrumbs(container, path) {
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

  // Click handlers for breadcrumb segments.
  container.querySelectorAll(".breadcrumb-item").forEach(function (item) {
    item.addEventListener("click", function (ev) {
      ev.stopPropagation();
      navigateTo(item.dataset.path);
    });
  });

  // Click on the container background enters edit mode.
  container.addEventListener("click", function (ev) {
    if (ev.target === container || ev.target.classList.contains("breadcrumb-sep")) {
      enterEditMode(container, path);
    }
  });

  // Double-click on any breadcrumb enters edit mode.
  container.addEventListener("dblclick", function () {
    enterEditMode(container, path);
  });
}

/**
 * Switch breadcrumb to an editable text input.
 * @param {HTMLElement} container - The breadcrumb container.
 * @param {string} currentPath - The current directory path.
 */
function enterEditMode(container, currentPath) {
  if (_isEditing) return;
  _isEditing = true;

  container.innerHTML = '<input type="text" class="breadcrumb-input" value="' + escapeAttr(currentPath) + '" spellcheck="false" autocomplete="off" />';
  var input = container.querySelector(".breadcrumb-input");
  input.focus();
  input.select();

  var _completionIndex = -1;
  var _completions = [];

  input.addEventListener("keydown", function (ev) {
    if (ev.key === "Enter") {
      ev.preventDefault();
      var value = input.value.trim();
      if (value) {
        _isEditing = false;
        navigateTo(value);
      }
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      _isEditing = false;
      renderBreadcrumbs(container, currentPath);
    } else if (ev.key === "Tab") {
      ev.preventDefault();
      // Tab completion.
      var partial = input.value;
      if (_completions.length > 0 && _completionIndex >= 0) {
        // Cycle through completions.
        _completionIndex = (_completionIndex + 1) % _completions.length;
        input.value = _completions[_completionIndex];
      } else {
        // Fetch completions from backend.
        api.completePath(partial).then(function (results) {
          if (results && results.length > 0) {
            _completions = results;
            _completionIndex = 0;
            input.value = _completions[0];
          }
        }).catch(function () {
          // Completion unavailable.
        });
      }
    }
  });

  // Reset completions on input change.
  input.addEventListener("input", function () {
    _completions = [];
    _completionIndex = -1;
  });

  input.addEventListener("blur", function () {
    // Small delay to allow Enter/Escape to fire first.
    setTimeout(function () {
      if (_isEditing) {
        _isEditing = false;
        renderBreadcrumbs(container, currentPath);
      }
    }, 150);
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
