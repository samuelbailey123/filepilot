// Command Palette — Cmd+K fuzzy-match overlay for actions, files, and recent locations.

import { getActions, getKeymap, formatKeyCombo, executeAction } from "../services/shortcuts.js";
import * as api from "../services/api.js";
import { navigateTo, getState } from "../app.js";

var _overlay = null;
var _input = null;
var _list = null;
var _results = [];
var _activeIndex = 0;
var _recentLocations = [];

/**
 * Initialize the command palette system.
 * Creates the overlay DOM once.
 */
export function initCommandPalette() {
  _recentLocations = loadRecentLocations();

  _overlay = document.createElement("div");
  _overlay.id = "cmd-palette-overlay";
  _overlay.className = "cmd-palette-overlay";
  _overlay.setAttribute("role", "dialog");
  _overlay.setAttribute("aria-label", "Command palette");
  _overlay.setAttribute("aria-hidden", "true");

  _overlay.innerHTML =
    '<div class="cmd-palette">' +
      '<input type="text" class="cmd-palette-input" placeholder="Type a command, file name, or path\u2026" spellcheck="false" autocomplete="off" />' +
      '<div class="cmd-palette-list" role="listbox"></div>' +
    '</div>';

  document.getElementById("app").appendChild(_overlay);

  _input = _overlay.querySelector(".cmd-palette-input");
  _list = _overlay.querySelector(".cmd-palette-list");

  _input.addEventListener("input", function () {
    renderResults(_input.value);
  });

  _input.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowDown") {
      ev.preventDefault();
      _activeIndex = Math.min(_activeIndex + 1, _results.length - 1);
      highlightActive();
    } else if (ev.key === "ArrowUp") {
      ev.preventDefault();
      _activeIndex = Math.max(_activeIndex - 1, 0);
      highlightActive();
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      executeActive();
    } else if (ev.key === "Escape") {
      ev.preventDefault();
      closeCommandPalette();
    }
  });

  _overlay.addEventListener("click", function (ev) {
    if (ev.target === _overlay) {
      closeCommandPalette();
    }
  });
}

/**
 * Open the command palette.
 */
export function openCommandPalette() {
  if (!_overlay) return;
  _overlay.classList.add("visible");
  _overlay.setAttribute("aria-hidden", "false");
  _input.value = "";
  _activeIndex = 0;
  renderResults("");
  _input.focus();
}

/**
 * Close the command palette.
 */
export function closeCommandPalette() {
  if (!_overlay) return;
  _overlay.classList.remove("visible");
  _overlay.setAttribute("aria-hidden", "true");
}

/**
 * Check if the command palette is currently visible.
 * @returns {boolean}
 */
export function isCommandPaletteVisible() {
  return _overlay && _overlay.classList.contains("visible");
}

/**
 * Track a location for recent locations.
 * @param {string} path - The directory path navigated to.
 */
export function trackRecentLocation(path) {
  if (!path || path === "/") return;
  // Remove if already present, then prepend.
  _recentLocations = _recentLocations.filter(function (p) { return p !== path; });
  _recentLocations.unshift(path);
  if (_recentLocations.length > 10) {
    _recentLocations = _recentLocations.slice(0, 10);
  }
  saveRecentLocations(_recentLocations);
}

/**
 * Build and render results based on the query.
 * @param {string} query - The search query.
 */
function renderResults(query) {
  _results = [];
  _activeIndex = 0;

  if (query.length === 0) {
    // Show recent locations when empty.
    _recentLocations.forEach(function (path) {
      _results.push({
        type: "recent",
        label: basename(path),
        detail: path,
        action: function () { navigateTo(path); },
      });
    });
  } else {
    // Actions: fuzzy match on label.
    var actions = getActions();
    var keymap = getKeymap();
    actions.forEach(function (a) {
      if (fuzzyMatch(query, a.label)) {
        _results.push({
          type: "action",
          label: a.label,
          detail: formatKeyCombo(keymap[a.name] || ""),
          action: a.name,
        });
      }
    });

    // Files: substring search (only if 2+ chars).
    if (query.length >= 2) {
      api.substringSearch(query, 8).then(function (results) {
        if (!results) return;
        results.forEach(function (r) {
          _results.push({
            type: "file",
            label: r.name || basename(r.path),
            detail: r.path,
            action: function () {
              if (r.isDir) {
                navigateTo(r.path);
              } else {
                // Navigate to parent, then select file.
                var parent = r.path.substring(0, r.path.lastIndexOf("/")) || "/";
                navigateTo(parent);
              }
            },
          });
        });
        renderList();
      }).catch(function () { /* ignore */ });
    }
  }

  renderList();
}

/**
 * Render the results list HTML.
 */
function renderList() {
  if (_results.length === 0) {
    _list.innerHTML = '<div class="cmd-palette-empty">No results</div>';
    return;
  }

  var html = "";
  _results.forEach(function (item, i) {
    var activeCls = i === _activeIndex ? " active" : "";
    var typeIcon = item.type === "action" ? "\u26A1" : item.type === "recent" ? "\uD83D\uDD52" : "\uD83D\uDCC4";
    html +=
      '<div class="cmd-palette-item' + activeCls + '" data-index="' + i + '" role="option">' +
        '<span class="cmd-palette-icon">' + typeIcon + '</span>' +
        '<span class="cmd-palette-label">' + escapeHtml(item.label) + '</span>' +
        '<span class="cmd-palette-detail">' + escapeHtml(item.detail || "") + '</span>' +
      '</div>';
  });

  _list.innerHTML = html;

  // Click handlers.
  _list.querySelectorAll(".cmd-palette-item").forEach(function (el) {
    el.addEventListener("click", function () {
      _activeIndex = parseInt(el.dataset.index, 10);
      executeActive();
    });
  });
}

/**
 * Highlight the active item in the list.
 */
function highlightActive() {
  _list.querySelectorAll(".cmd-palette-item").forEach(function (el, i) {
    el.classList.toggle("active", i === _activeIndex);
    if (i === _activeIndex) {
      el.scrollIntoView({ block: "nearest" });
    }
  });
}

/**
 * Execute the currently active result.
 */
function executeActive() {
  var item = _results[_activeIndex];
  if (!item) return;
  closeCommandPalette();

  if (typeof item.action === "function") {
    item.action();
  } else if (typeof item.action === "string") {
    executeAction(item.action);
  }
}

/**
 * Simple fuzzy match: every character of query appears in order in target.
 * @param {string} query - The search query.
 * @param {string} target - The target string.
 * @returns {boolean} True if query fuzzy-matches target.
 */
function fuzzyMatch(query, target) {
  var q = query.toLowerCase();
  var t = target.toLowerCase();
  var qi = 0;
  for (var ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) qi++;
  }
  return qi === q.length;
}

function loadRecentLocations() {
  try {
    var raw = localStorage.getItem("fp-recent-locations");
    if (raw) return JSON.parse(raw);
  } catch (e) { /* ignore */ }
  return [];
}

function saveRecentLocations(locs) {
  try {
    localStorage.setItem("fp-recent-locations", JSON.stringify(locs));
  } catch (e) { /* ignore */ }
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
