// Info Panel — File metadata display with permissions editor.

import * as api from "../services/api.js";
import { getFileIcon, getFileColorClass } from "../views/column.js";

var _overlay = null;
var _currentPath = null;

/**
 * Initialize the info panel overlay. Creates the DOM once.
 */
export function initInfoPanel() {
  _overlay = document.createElement("div");
  _overlay.className = "info-panel-overlay";
  _overlay.setAttribute("role", "dialog");
  _overlay.setAttribute("aria-label", "File information");
  _overlay.setAttribute("aria-hidden", "true");

  _overlay.addEventListener("click", function (ev) {
    if (ev.target === _overlay) closeInfoPanel();
  });

  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape" && _overlay.classList.contains("visible")) {
      ev.preventDefault();
      closeInfoPanel();
    }
  });

  document.getElementById("app").appendChild(_overlay);
}

/**
 * Open the info panel for a file or directory.
 * @param {string} path - The file path.
 */
export async function openInfoPanel(path) {
  if (!_overlay) return;
  _currentPath = path;
  _overlay.innerHTML = '<div class="info-panel"><div style="padding:20px;text-align:center;color:var(--text-faint)">Loading\u2026</div></div>';
  _overlay.classList.add("visible");
  _overlay.setAttribute("aria-hidden", "false");

  try {
    var info = await api.getFileInfo(path);
    if (!info) {
      _overlay.querySelector(".info-panel").innerHTML = '<div style="padding:20px;color:var(--text-faint)">Unable to load file info.</div>';
      return;
    }
    renderInfoPanel(info, path);
  } catch (err) {
    _overlay.querySelector(".info-panel").innerHTML = '<div style="padding:20px;color:var(--red,#ff3b30)">Error: ' + escapeHtml(String(err)) + '</div>';
  }
}

/**
 * Close the info panel.
 */
export function closeInfoPanel() {
  if (!_overlay) return;
  _overlay.classList.remove("visible");
  _overlay.setAttribute("aria-hidden", "true");
  _currentPath = null;
}

/**
 * Check if info panel is currently visible.
 * @returns {boolean}
 */
export function isInfoPanelVisible() {
  return _overlay && _overlay.classList.contains("visible");
}

function renderInfoPanel(info, path) {
  var name = basename(path);
  var fakeEntry = { name: name, isDir: info.isDir, extension: name.split(".").pop() };
  var icon = getFileIcon(fakeEntry);

  var mode = info.permissions || 0;
  var permBits = parsePermissions(mode);

  var html =
    '<div class="info-panel">' +
      '<div class="info-panel-header">' +
        '<span class="info-panel-icon">' + icon + '</span>' +
        '<span class="info-panel-title">' + escapeHtml(name) + '</span>' +
      '</div>' +

      '<div class="info-panel-section">' +
        '<h4>General</h4>' +
        infoRow("Path", path) +
        infoRow("Kind", info.isDir ? "Folder" : (info.kind || "File")) +
        infoRow("Size", formatBytes(info.size || 0)) +
        (info.symlinkTarget ? infoRow("Alias Target", info.symlinkTarget) : "") +
      '</div>' +

      '<div class="info-panel-section">' +
        '<h4>Dates</h4>' +
        infoRow("Created", formatDate(info.created)) +
        infoRow("Modified", formatDate(info.modified)) +
        infoRow("Accessed", formatDate(info.accessed)) +
      '</div>' +

      '<div class="info-panel-section">' +
        '<h4>Ownership</h4>' +
        infoRow("Owner", info.owner || "—") +
        infoRow("Group", info.group || "—") +
      '</div>' +

      '<div class="info-panel-section">' +
        '<h4>Permissions</h4>' +
        renderPermGrid(permBits) +
        '<div class="info-panel-octal">' +
          '<span>Octal:</span>' +
          '<input type="text" id="info-perm-octal" value="' + octalString(mode) + '" maxlength="4" />' +
        '</div>' +
      '</div>' +

      '<div class="info-panel-actions">' +
        '<button id="info-cancel">Cancel</button>' +
        '<button id="info-save" class="primary">Save Permissions</button>' +
      '</div>' +
    '</div>';

  _overlay.innerHTML = html;

  // Wire up cancel.
  _overlay.querySelector("#info-cancel").addEventListener("click", closeInfoPanel);

  // Wire up save.
  _overlay.querySelector("#info-save").addEventListener("click", function () {
    savePermissions(path);
  });

  // Wire up checkbox changes to update octal display.
  _overlay.querySelectorAll(".perm-checkbox").forEach(function (cb) {
    cb.addEventListener("change", function () {
      updateOctalFromCheckboxes();
    });
  });

  // Wire up octal input to update checkboxes.
  var octalInput = _overlay.querySelector("#info-perm-octal");
  octalInput.addEventListener("input", function () {
    updateCheckboxesFromOctal(octalInput.value);
  });
}

function renderPermGrid(bits) {
  var labels = ["Owner", "Group", "Other"];
  var keys = ["r", "w", "x"];
  var keyLabels = ["Read", "Write", "Execute"];

  var html =
    '<div class="perm-grid">' +
      '<div></div>' +
      '<div class="perm-header">R</div>' +
      '<div class="perm-header">W</div>' +
      '<div class="perm-header">X</div>';

  for (var i = 0; i < 3; i++) {
    html += '<div class="perm-label">' + labels[i] + '</div>';
    for (var j = 0; j < 3; j++) {
      var idx = i * 3 + j;
      var checked = bits[idx] ? " checked" : "";
      html += '<input type="checkbox" class="perm-checkbox" data-perm-idx="' + idx + '"' + checked + ' title="' + labels[i] + ' ' + keyLabels[j] + '" />';
    }
  }

  html += '</div>';
  return html;
}

function parsePermissions(mode) {
  var bits = [];
  for (var i = 8; i >= 0; i--) {
    bits.push((mode >> i) & 1);
  }
  return bits;
}

function octalString(mode) {
  var masked = mode & 0o777;
  return masked.toString(8).padStart(3, "0");
}

function updateOctalFromCheckboxes() {
  var checkboxes = _overlay.querySelectorAll(".perm-checkbox");
  var mode = 0;
  checkboxes.forEach(function (cb) {
    var idx = parseInt(cb.dataset.permIdx, 10);
    if (cb.checked) {
      mode |= (1 << (8 - idx));
    }
  });
  var octalInput = _overlay.querySelector("#info-perm-octal");
  if (octalInput) octalInput.value = octalString(mode);
}

function updateCheckboxesFromOctal(val) {
  var num = parseInt(val, 8);
  if (isNaN(num) || num < 0 || num > 0o777) return;
  var checkboxes = _overlay.querySelectorAll(".perm-checkbox");
  checkboxes.forEach(function (cb) {
    var idx = parseInt(cb.dataset.permIdx, 10);
    cb.checked = !!((num >> (8 - idx)) & 1);
  });
}

async function savePermissions(path) {
  var octalInput = _overlay.querySelector("#info-perm-octal");
  if (!octalInput) return;
  var val = octalInput.value;
  var num = parseInt(val, 8);
  if (isNaN(num) || num < 0 || num > 0o777) return;

  try {
    await api.setPermissions(path, num);
    closeInfoPanel();
  } catch (err) {
    var panel = _overlay.querySelector(".info-panel");
    if (panel) {
      var errDiv = document.createElement("div");
      errDiv.style.color = "var(--red, #ff3b30)";
      errDiv.style.fontSize = "12px";
      errDiv.style.marginTop = "8px";
      errDiv.textContent = "Save failed: " + err;
      panel.appendChild(errDiv);
    }
  }
}

function infoRow(label, value) {
  return '<div class="info-panel-row"><span class="label">' + escapeHtml(label) + '</span><span class="value">' + escapeHtml(value || "—") + '</span></div>';
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function formatDate(dateStr) {
  if (!dateStr) return "—";
  try {
    var d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleString();
  } catch (e) {
    return dateStr;
  }
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
