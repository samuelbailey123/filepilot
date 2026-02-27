// Search Overlay — Cmd+/ triggered full-text search.
// Supports global search and scoped search within current folder.

import * as api from "../services/api.js";
import { navigateTo, selectFile, getState } from "../app.js";

let debounceTimer = null;
let activeIndex = 0;
let scopedMode = false;
let searchMode = "smart"; // "smart" | "substring" | "regex" | "content"

export function initSearch() {
  var overlay = document.getElementById("search-overlay");

  overlay.innerHTML =
    '<div class="search-box">' +
      '<div class="search-input-wrap">' +
        '<span class="search-input-icon">\uD83D\uDD0D</span>' +
        '<input class="search-input" type="text" placeholder="Search files..." autocomplete="off" spellcheck="false">' +
        '<button class="search-mode-btn" title="Search mode" aria-label="Search mode">Smart</button>' +
        '<button class="search-scope-btn" title="Toggle search scope" aria-label="Toggle search scope">' +
          'All' +
        '</button>' +
      "</div>" +
      '<div class="search-hint">Type to search \u2022 \u2191\u2193 navigate \u2022 Enter to open \u2022 Esc to close \u2022 Tab scope \u2022 Cmd+M mode</div>' +
      '<div class="search-results"></div>' +
    "</div>";

  var input = overlay.querySelector(".search-input");
  var resultsEl = overlay.querySelector(".search-results");
  var scopeBtn = overlay.querySelector(".search-scope-btn");
  var modeBtn = overlay.querySelector(".search-mode-btn");

  // Update search mode button.
  function updateModeBtn() {
    var labels = { smart: "Smart", substring: "Substr", regex: "Regex", content: "Content" };
    modeBtn.textContent = labels[searchMode] || "Smart";
    modeBtn.classList.toggle("active", searchMode !== "smart");
  }

  function cycleSearchMode() {
    var modes = ["smart", "substring", "regex", "content"];
    var idx = modes.indexOf(searchMode);
    searchMode = modes[(idx + 1) % modes.length];
    updateModeBtn();
    var query = input.value.trim();
    if (query.length >= 2) {
      if (debounceTimer) clearTimeout(debounceTimer);
      performSearch(query, resultsEl);
    }
  }

  modeBtn.addEventListener("click", function (ev) {
    ev.preventDefault();
    cycleSearchMode();
  });
  updateModeBtn();

  // Update scope button text.
  function updateScopeBtn() {
    if (searchMode === "content") {
      var state = getState();
      var folderName = basename(state.currentPath || "");
      scopeBtn.textContent = folderName || "/";
      scopeBtn.classList.add("active");
      input.placeholder = "Search file contents in " + folderName + "...";
    } else if (scopedMode) {
      var state = getState();
      var folderName = basename(state.currentPath || "");
      scopeBtn.textContent = folderName || "/";
      scopeBtn.classList.add("active");
      input.placeholder = "Search in " + folderName + "...";
    } else {
      scopeBtn.textContent = "All";
      scopeBtn.classList.remove("active");
      input.placeholder = "Search files...";
    }
  }

  // Toggle scope on button click.
  scopeBtn.addEventListener("click", function (ev) {
    ev.preventDefault();
    scopedMode = !scopedMode;
    updateScopeBtn();
    // Re-search with current query.
    var query = input.value.trim();
    if (query.length >= 2) {
      if (debounceTimer) clearTimeout(debounceTimer);
      performSearch(query, resultsEl);
    }
  });

  // Input handler with debounce.
  input.addEventListener("input", function () {
    var query = input.value.trim();
    if (debounceTimer) clearTimeout(debounceTimer);

    if (query.length < 2) {
      resultsEl.innerHTML = "";
      return;
    }

    debounceTimer = setTimeout(function () {
      performSearch(query, resultsEl);
    }, 290);
  });

  // Keyboard navigation within search.
  input.addEventListener("keydown", function (ev) {
    var items = resultsEl.querySelectorAll(".search-result");

    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      activeIndex = Math.min(activeIndex + 1, items.length - 1);
      updateActive(items);
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      activeIndex = Math.max(activeIndex - 1, 0);
      updateActive(items);
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      if (items[activeIndex]) items[activeIndex].click();
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      closeSearch();
    } else if (ev.key === "Tab") {
      ev.preventDefault();
      scopedMode = !scopedMode;
      updateScopeBtn();
      var query = input.value.trim();
      if (query.length >= 2) {
        performSearch(query, resultsEl);
      }
    } else if (ev.key === "m" && (ev.metaKey || ev.ctrlKey)) {
      ev.preventDefault();
      cycleSearchMode();
    }
  });

  // Close on background click.
  overlay.addEventListener("click", function (ev) {
    if (ev.target === overlay) {
      closeSearch();
    }
  });
}

/**
 * Open the search overlay. Called externally from app.js.
 * @param {boolean} scoped - If true, start in scoped mode.
 */
export function openSearch(scoped) {
  scopedMode = !!scoped;
  var overlay = document.getElementById("search-overlay");
  overlay.classList.add("visible");
  overlay.setAttribute("aria-hidden", "false");
  var input = overlay.querySelector(".search-input");
  var scopeBtn = overlay.querySelector(".search-scope-btn");

  if (scopeBtn) {
    if (scopedMode) {
      var state = getState();
      var folderName = basename(state.currentPath || "");
      scopeBtn.textContent = folderName || "/";
      scopeBtn.classList.add("active");
      input.placeholder = "Search in " + folderName + "...";
    } else {
      scopeBtn.textContent = "All";
      scopeBtn.classList.remove("active");
      input.placeholder = "Search files...";
    }
  }

  if (input) input.focus();
}

async function performSearch(query, resultsEl) {
  try {
    var results;
    var state = getState();
    if (searchMode === "content") {
      // Content search always uses current directory as scope.
      var dirPath = state.currentPath;
      results = await api.grepInDir(query, dirPath, 30);
    } else if (searchMode === "substring") {
      results = scopedMode
        ? await api.substringSearchInDir(query, state.currentPath, 30)
        : await api.substringSearch(query, 30);
    } else if (searchMode === "regex") {
      results = scopedMode
        ? await api.regexSearchInDir(query, state.currentPath, 30)
        : await api.regexSearch(query, 30);
    } else {
      results = scopedMode
        ? await api.searchInDir(query, state.currentPath, 30)
        : await api.search(query, 30);
    }
    activeIndex = 0;
    renderResults(results || [], query, resultsEl);
  } catch (err) {
    var errMsg = String(err);
    if (searchMode === "regex" && errMsg.includes("invalid regex")) {
      resultsEl.innerHTML = '<div class="search-result"><div class="result-title">Invalid regex pattern</div></div>';
    } else {
      resultsEl.innerHTML = '<div class="search-result"><div class="result-title">Search error</div></div>';
    }
  }
}

function renderResults(results, query, container) {
  if (results.length === 0) {
    container.innerHTML =
      '<div style="padding: 16px; text-align: center; color: var(--text-faint);">No results found</div>';
    return;
  }

  var html = "";
  results.forEach(function (r, i) {
    var ext = r.extension ? r.extension.toUpperCase() : "FILE";
    var isDir = r.isDir ? "DIR" : ext;
    var activeCls = i === 0 ? " active" : "";
    var size = r.isDir ? "" : formatBytes(r.size);
    var snippet = highlightSnippet(r.snippet || r.path, query);

    html +=
      '<div class="search-result' + activeCls + '" data-path="' + escapeAttr(r.path) + '" data-is-dir="' + r.isDir + '">' +
        '<div class="result-title">' +
          escapeHtml(r.name) +
          '<span class="result-type-pill">' + isDir + "</span>" +
        "</div>" +
        '<div class="result-snippet">' + snippet + "</div>" +
        '<div class="result-meta">' +
          (size ? "<span>" + size + "</span>" : "") +
          "<span>" + formatRelTime(r.modTime) + "</span>" +
        "</div>" +
      "</div>";
  });

  container.innerHTML = html;

  // Click handlers.
  container.querySelectorAll(".search-result").forEach(function (item) {
    item.addEventListener("click", function () {
      var path = item.dataset.path;
      var isDir = item.dataset.isDir === "true";
      closeSearch();

      if (isDir) {
        navigateTo(path);
      } else {
        var parent = path.substring(0, path.lastIndexOf("/"));
        navigateTo(parent).then(function () {
          selectFile(path);
        });
      }
    });
  });
}

function updateActive(items) {
  items.forEach(function (el, i) {
    el.classList.toggle("active", i === activeIndex);
  });
  if (items[activeIndex]) {
    items[activeIndex].scrollIntoView({ block: "nearest" });
  }
}

function closeSearch() {
  var overlay = document.getElementById("search-overlay");
  overlay.classList.remove("visible");
  overlay.setAttribute("aria-hidden", "true");
  var input = overlay.querySelector(".search-input");
  if (input) {
    input.value = "";
  }
  var results = overlay.querySelector(".search-results");
  if (results) results.innerHTML = "";
}

function highlightSnippet(text, query) {
  var escaped = escapeHtml(text);
  var re = new RegExp("(" + escapeRegExp(query) + ")", "gi");
  return escaped.replace(re, "<mark>$1</mark>");
}

function escapeRegExp(str) {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "";
  var units = ["B", "KB", "MB", "GB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function formatRelTime(unix) {
  if (!unix) return "";
  var diff = (Date.now() / 1000) - unix;
  if (diff < 60) return "just now";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  if (diff < 604800) return Math.floor(diff / 86400) + "d ago";
  return new Date(unix * 1000).toLocaleDateString();
}

function basename(path) {
  if (!path) return "";
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
