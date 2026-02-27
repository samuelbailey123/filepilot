// Diff View — Side-by-side file comparison with colored diff lines.

import * as api from "../services/api.js";

var _overlay = null;

/**
 * Initialize the diff view overlay. Creates the DOM once.
 */
export function initDiffView() {
  _overlay = document.createElement("div");
  _overlay.className = "diff-overlay";
  _overlay.setAttribute("role", "dialog");
  _overlay.setAttribute("aria-label", "File diff");
  _overlay.setAttribute("aria-hidden", "true");

  _overlay.addEventListener("click", function (ev) {
    if (ev.target === _overlay) closeDiffView();
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape" && _overlay.classList.contains("visible")) {
      ev.preventDefault();
      closeDiffView();
    }
  });

  document.getElementById("app").appendChild(_overlay);
}

/**
 * Open the diff view comparing two files.
 * @param {string} pathA - The first file path.
 * @param {string} pathB - The second file path.
 */
export async function openDiffView(pathA, pathB) {
  if (!_overlay) return;

  _overlay.innerHTML =
    '<div class="diff-panel">' +
      '<div class="diff-header">' +
        '<span class="diff-header-title">Comparing files\u2026</span>' +
        '<button class="diff-close" aria-label="Close">\u00D7</button>' +
      '</div>' +
      '<div class="diff-body" style="padding:20px;text-align:center;color:var(--text-faint)">Loading diff\u2026</div>' +
    '</div>';

  _overlay.classList.add("visible");
  _overlay.setAttribute("aria-hidden", "false");

  _overlay.querySelector(".diff-close").addEventListener("click", closeDiffView);

  try {
    var lines = await api.diffFiles(pathA, pathB);
    if (!lines) lines = [];
    renderDiff(pathA, pathB, lines);
  } catch (err) {
    var body = _overlay.querySelector(".diff-body");
    if (body) {
      body.innerHTML = '<div style="color:var(--red,#ff3b30);padding:20px">Diff failed: ' + escapeHtml(String(err)) + '</div>';
    }
  }
}

/**
 * Close the diff view.
 */
export function closeDiffView() {
  if (!_overlay) return;
  _overlay.classList.remove("visible");
  _overlay.setAttribute("aria-hidden", "true");
}

/**
 * Check if the diff view is currently visible.
 * @returns {boolean}
 */
export function isDiffViewVisible() {
  return _overlay && _overlay.classList.contains("visible");
}

function renderDiff(pathA, pathB, lines) {
  var addCount = 0;
  var delCount = 0;

  var linesHtml = "";
  var lineNum = 0;

  lines.forEach(function (line) {
    lineNum++;
    var type = line.Type || line.type || "equal";
    var text = line.Text || line.text || "";
    var cls = "diff-line-eq";

    if (type === "add") {
      cls = "diff-line-add";
      addCount++;
    } else if (type === "delete") {
      cls = "diff-line-del";
      delCount++;
    }

    linesHtml +=
      '<div class="diff-line ' + cls + '">' +
        '<span class="diff-line-num">' + lineNum + '</span>' +
        '<span class="diff-line-text">' + escapeHtml(text) + '</span>' +
      '</div>';
  });

  if (lines.length === 0) {
    linesHtml = '<div style="padding:20px;text-align:center;color:var(--text-faint)">Files are identical.</div>';
  }

  _overlay.innerHTML =
    '<div class="diff-panel">' +
      '<div class="diff-header">' +
        '<span class="diff-header-title">File Diff</span>' +
        '<div class="diff-header-files">' +
          '<span>' + escapeHtml(basename(pathA)) + '</span>' +
          '<span>\u2194</span>' +
          '<span>' + escapeHtml(basename(pathB)) + '</span>' +
        '</div>' +
        '<button class="diff-close" aria-label="Close">\u00D7</button>' +
      '</div>' +
      '<div class="diff-body">' + linesHtml + '</div>' +
      '<div class="diff-stats">' +
        '<span class="stat-add">+' + addCount + ' added</span>' +
        '<span class="stat-del">-' + delCount + ' deleted</span>' +
        '<span>' + lines.length + ' lines total</span>' +
      '</div>' +
    '</div>';

  _overlay.querySelector(".diff-close").addEventListener("click", closeDiffView);
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
