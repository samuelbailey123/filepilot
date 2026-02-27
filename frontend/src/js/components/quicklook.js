// Quick Look Overlay — Spacebar-triggered full preview.

import * as api from "../services/api.js";
import { getState, announce } from "../app.js";

var _visible = false;
var _currentPath = null;

/**
 * Toggle the Quick Look overlay for the currently selected file.
 * If already showing the same file, close it. If a different file is
 * selected, show it. If nothing is selected, do nothing.
 */
export function toggleQuickLook() {
  var state = getState();
  if (_visible) {
    closeQuickLook();
    return;
  }
  if (!state.selectedFile) return;
  openQuickLook(state.selectedFile);
}

/**
 * Open Quick Look for the given file path.
 * @param {string} filePath - The file to preview.
 */
export async function openQuickLook(filePath) {
  var overlay = document.getElementById("quicklook-overlay");
  if (!overlay) return;

  _currentPath = filePath;
  _visible = true;

  overlay.innerHTML =
    '<div class="quicklook-card">' +
      '<div class="quicklook-header">' +
        '<div class="quicklook-title">Loading...</div>' +
      '</div>' +
      '<div class="quicklook-body"></div>' +
    '</div>';
  overlay.classList.add("visible");
  overlay.setAttribute("aria-hidden", "false");

  try {
    var preview = await api.getPreview(filePath);
    if (_currentPath !== filePath) return;
    renderQuickLook(overlay, preview, filePath);
  } catch (err) {
    overlay.querySelector(".quicklook-title").textContent = basename(filePath);
    overlay.querySelector(".quicklook-body").innerHTML =
      '<div class="empty-state"><div class="empty-text">Preview unavailable</div></div>';
  }
  announce("Quick Look: " + basename(filePath));
}

/**
 * Close the Quick Look overlay.
 */
export function closeQuickLook() {
  var overlay = document.getElementById("quicklook-overlay");
  if (!overlay) return;

  _visible = false;
  _currentPath = null;
  overlay.classList.remove("visible");
  overlay.setAttribute("aria-hidden", "true");
  announce("Quick Look closed");
}

/**
 * Returns whether Quick Look is currently visible.
 * @returns {boolean}
 */
export function isQuickLookVisible() {
  return _visible;
}

function renderQuickLook(overlay, preview, filePath) {
  var card = overlay.querySelector(".quicklook-card");
  if (!card) return;

  var header =
    '<div class="quicklook-header">' +
      '<div class="quicklook-title">' + escapeHtml(preview.name) + '</div>' +
      '<div class="quicklook-meta">' +
        '<span class="type-pill type-' + typeColor(preview) + '">' + typeLabel(preview) + '</span>' +
        '<span>' + formatBytes(preview.size) + '</span>' +
        (preview.lines > 0 ? '<span>' + preview.lines + ' lines</span>' : '') +
      '</div>' +
    '</div>';

  var body = '<div class="quicklook-body">';

  switch (preview.type) {
    case "code":
      var lang = preview.language || "plaintext";
      var escaped = escapeHtml(preview.content);
      if (window.hljs && lang !== "plaintext") {
        try {
          var result = window.hljs.highlight(preview.content, { language: lang });
          escaped = result.value;
        } catch (e) {}
      }
      body += '<pre><code>' + escaped + '</code></pre>';
      break;

    case "image":
      body += '<img src="' + preview.content + '" alt="' + escapeAttr(preview.name) + '">';
      break;

    case "pdf":
      body += renderMetadata(preview.metadata, "PDF Document");
      break;

    case "audio":
      body += renderMetadata(preview.metadata, "Audio File");
      break;

    case "video":
      body += renderMetadata(preview.metadata, "Video File");
      break;

    case "archive":
      if (preview.entries && preview.entries.length > 0) {
        body += '<div class="archive-list">';
        for (var i = 0; i < preview.entries.length; i++) {
          body += '<div class="archive-entry">' + escapeHtml(preview.entries[i]) + '</div>';
        }
        body += '</div>';
      } else {
        body += '<div class="empty-state"><div class="empty-text">Archive</div></div>';
      }
      break;

    case "hex":
      body += '<pre class="hex-dump"><code>' + escapeHtml(preview.content) + '</code></pre>';
      break;

    case "directory":
      body += '<div class="empty-state"><div class="empty-text">Directory</div></div>';
      break;

    default:
      body += '<div class="empty-state"><div class="empty-text">' + escapeHtml(typeLabel(preview)) + '</div></div>';
  }

  body += '</div>';

  var footer =
    '<div class="quicklook-footer">' +
      '<span>' + escapeHtml(filePath) + '</span>' +
      '<div class="quicklook-actions">' +
        '<button class="icon-btn" data-action="open">Open</button>' +
      '</div>' +
    '</div>';

  card.innerHTML = header + body + footer;

  // Open button handler.
  var openBtn = card.querySelector('[data-action="open"]');
  if (openBtn) {
    openBtn.addEventListener("click", function () {
      api.openFile(filePath);
    });
  }
}

function renderMetadata(metadata, label) {
  if (!metadata || Object.keys(metadata).length === 0) {
    return '<div class="empty-state"><div class="empty-text">' + escapeHtml(label) + '</div></div>';
  }
  var html = '<table class="metadata-table">';
  var keys = Object.keys(metadata);
  for (var i = 0; i < keys.length; i++) {
    html += '<tr><td class="metadata-key">' + escapeHtml(keys[i]) + '</td>' +
            '<td class="metadata-value">' + escapeHtml(metadata[keys[i]]) + '</td></tr>';
  }
  html += '</table>';
  return html;
}

function typeLabel(preview) {
  switch (preview.type) {
    case "code": return preview.language || "Code";
    case "image": return "Image";
    case "directory": return "Directory";
    case "pdf": return "PDF";
    case "audio": return "Audio";
    case "video": return "Video";
    case "archive": return "Archive";
    case "font": return "Font";
    case "hex": return "Hex Dump";
    case "binary": return "Binary";
    default: return "File";
  }
}

function typeColor(preview) {
  switch (preview.type) {
    case "code": return "code";
    case "image": return "media";
    case "directory": return "folder";
    case "pdf": return "doc";
    case "audio": return "media";
    case "video": return "media";
    case "archive": return "archive";
    case "font": return "data";
    case "hex": return "config";
    case "binary": return "archive";
    default: return "doc";
  }
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function basename(path) {
  if (!path) return "";
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

function escapeHtml(str) {
  if (!str) return "";
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  if (!str) return "";
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
