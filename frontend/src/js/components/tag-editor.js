// Tag Editor — Compact color-dot palette for batch tag editing.

import * as api from "../services/api.js";
import { navigateTo, getState, announce } from "../app.js";

var _overlayEl = null;
var _currentPaths = [];
var _tagState = {}; // color -> "all" | "some" | "none"

var TAG_COLORS = [
  { name: "Red",    value: "red",    hex: "#ff3b30" },
  { name: "Orange", value: "orange", hex: "#ff9500" },
  { name: "Yellow", value: "yellow", hex: "#ffcc00" },
  { name: "Green",  value: "green",  hex: "#34c759" },
  { name: "Blue",   value: "blue",   hex: "#007aff" },
  { name: "Purple", value: "purple", hex: "#af52de" },
  { name: "Gray",   value: "gray",   hex: "#8e8e93" },
];

/**
 * Initialize the tag editor overlay and append it to #app.
 */
export function initTagEditor() {
  _overlayEl = document.createElement("div");
  _overlayEl.id = "tag-editor-overlay";
  _overlayEl.className = "tag-editor-overlay";
  _overlayEl.setAttribute("role", "dialog");
  _overlayEl.setAttribute("aria-modal", "true");
  _overlayEl.setAttribute("aria-label", "Edit Tags");

  _overlayEl.innerHTML =
    '<div class="tag-editor">' +
      '<div class="tag-editor-title">Edit Tags</div>' +
      '<div class="tag-editor-palette" role="group" aria-label="Tag color palette"></div>' +
      '<div class="tag-editor-actions">' +
        '<button class="tag-cancel-btn">Cancel</button>' +
        '<button class="tag-apply-btn">Apply</button>' +
      '</div>' +
    '</div>';

  // Close on backdrop click.
  _overlayEl.addEventListener("click", function (ev) {
    if (ev.target === _overlayEl) {
      closeTagEditor();
    }
  });

  // Close on Escape key.
  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape" && _overlayEl.classList.contains("visible")) {
      closeTagEditor();
    }
  });

  _overlayEl.querySelector(".tag-cancel-btn").addEventListener("click", closeTagEditor);
  _overlayEl.querySelector(".tag-apply-btn").addEventListener("click", applyTags);

  var appEl = document.getElementById("app");
  if (appEl) {
    appEl.appendChild(_overlayEl);
  } else {
    document.body.appendChild(_overlayEl);
  }
}

/**
 * Open the tag editor for a set of file paths.
 * Loads existing tags for each path, computes union and intersection,
 * then renders the color palette with appropriate button states.
 * @param {string[]} paths - Array of file paths to edit tags for.
 */
export function openTagEditor(paths) {
  if (!paths || paths.length === 0) return;
  _currentPaths = paths.slice();

  // Load tags for all paths concurrently.
  Promise.all(paths.map(function (p) {
    return api.getFileTags(p).catch(function () { return []; });
  })).then(function (allTags) {
    // Compute union and intersection of tags across all files.
    var unionSet = new Set();
    var intersectionMap = {}; // color -> count of files that have it

    allTags.forEach(function (tags) {
      var seen = new Set();
      (tags || []).forEach(function (t) {
        var colorVal = normalizeTagColor(t);
        if (colorVal && !seen.has(colorVal)) {
          seen.add(colorVal);
          unionSet.add(colorVal);
          intersectionMap[colorVal] = (intersectionMap[colorVal] || 0) + 1;
        }
      });
    });

    var total = paths.length;

    // Determine state for each standard color.
    TAG_COLORS.forEach(function (tc) {
      var count = intersectionMap[tc.value] || 0;
      if (count === total) {
        _tagState[tc.value] = "all";
      } else if (count > 0) {
        _tagState[tc.value] = "some";
      } else {
        _tagState[tc.value] = "none";
      }
    });

    renderPalette();
    _overlayEl.classList.add("visible");

    // Set title.
    var titleEl = _overlayEl.querySelector(".tag-editor-title");
    if (titleEl) {
      titleEl.textContent = paths.length === 1
        ? "Edit Tags"
        : "Edit Tags (" + paths.length + " items)";
    }

    // Focus the first palette button.
    var firstBtn = _overlayEl.querySelector(".tag-editor-btn");
    if (firstBtn) firstBtn.focus();
  }).catch(function (err) {
    console.error("Tag editor load error:", err);
    announce("Failed to load tags");
  });
}

/**
 * Render the color palette buttons with current state classes.
 */
function renderPalette() {
  var palette = _overlayEl.querySelector(".tag-editor-palette");
  if (!palette) return;

  palette.innerHTML = TAG_COLORS.map(function (tc) {
    var state = _tagState[tc.value] || "none";
    var cls = "tag-editor-btn";
    if (state === "all") cls += " active";
    if (state === "some") cls += " mixed";

    return '<button class="' + cls + '"' +
      ' data-color="' + tc.value + '"' +
      ' style="background-color:' + tc.hex + ';"' +
      ' aria-label="' + tc.name + (state === "all" ? " (on all)" : state === "some" ? " (on some)" : "") + '"' +
      ' title="' + tc.name + '">' +
    '</button>';
  }).join("");

  // Bind click handlers.
  palette.querySelectorAll(".tag-editor-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var color = btn.dataset.color;
      var current = _tagState[color] || "none";

      // Toggle: if on all files already, remove; otherwise add to all.
      if (current === "all") {
        _tagState[color] = "none";
      } else {
        _tagState[color] = "all";
      }

      renderPalette();
    });
  });
}

/**
 * Apply the current tag state to all target paths.
 * Tags with state "all" are applied; "none"/"some" tags in the intersection
 * are removed; "some" not toggled to "all" are left as-is per file.
 */
function applyTags() {
  // Collect colors that should be on all files.
  var colorsThatShouldBeOnAll = new Set();
  var colorsThatShouldBeRemovedFromAll = new Set();

  TAG_COLORS.forEach(function (tc) {
    var state = _tagState[tc.value] || "none";
    if (state === "all") {
      colorsThatShouldBeOnAll.add(tc.value);
    } else if (state === "none") {
      colorsThatShouldBeRemovedFromAll.add(tc.value);
    }
    // "some" state: leave per-file tags unchanged for that color.
  });

  // For each path, fetch current tags, then apply the diff.
  Promise.all(_currentPaths.map(function (p) {
    return api.getFileTags(p).catch(function () { return []; }).then(function (tags) {
      var currentColors = new Set((tags || []).map(normalizeTagColor).filter(Boolean));

      // Remove colors that should be off.
      colorsThatShouldBeRemovedFromAll.forEach(function (c) {
        currentColors.delete(c);
      });

      // Add colors that should be on.
      colorsThatShouldBeOnAll.forEach(function (c) {
        currentColors.add(c);
      });

      var newTags = Array.from(currentColors);
      return api.setFileTags(p, newTags);
    });
  })).then(function () {
    closeTagEditor();
    var s = getState();
    if (s.currentPath) {
      navigateTo(s.currentPath);
    }
    announce("Tags updated");
  }).catch(function (err) {
    console.error("Tag apply error:", err);
    announce("Failed to update tags");
    closeTagEditor();
  });
}

/**
 * Close and hide the tag editor overlay.
 */
function closeTagEditor() {
  if (_overlayEl) {
    _overlayEl.classList.remove("visible");
  }
  _currentPaths = [];
  _tagState = {};
}

/**
 * Normalize a raw tag string to a standard color value.
 * macOS Finder tags are stored as "ColorName\nN" (e.g. "Red\n2").
 * @param {string} tag - Raw tag string.
 * @returns {string|null} Normalized lowercase color name or null.
 */
function normalizeTagColor(tag) {
  if (!tag) return null;
  var name = tag.toLowerCase().split("\n")[0].trim();
  var validColors = ["red", "orange", "yellow", "green", "blue", "purple", "gray", "grey"];
  if (validColors.indexOf(name) >= 0) {
    return name === "grey" ? "gray" : name;
  }
  return null;
}
