// Preview Panel — File preview with metadata and actions.

import * as api from "../services/api.js";
import { getState, navigateTo } from "../app.js";

export async function initPreview(filePath) {
  var panel = document.getElementById("preview-panel");

  panel.innerHTML =
    '<div class="preview-header"><div class="preview-filename">Loading...</div></div>' +
    '<div class="preview-content"></div>';

  try {
    var preview = await api.getPreview(filePath);
    renderPreview(panel, preview, filePath);
  } catch (err) {
    panel.innerHTML =
      '<div class="preview-header">' +
        '<div class="preview-filename">' + escapeHtml(basename(filePath)) + "</div>" +
        '<div class="preview-meta">Error loading preview</div>' +
      "</div>";
  }
}

function renderPreview(panel, preview, filePath) {
  var header =
    '<div class="preview-header">' +
      '<div class="preview-filename">' + escapeHtml(preview.name) + "</div>" +
      '<div class="preview-meta">' +
        '<span>' + typeLabel(preview) + "</span>" +
        '<span>' + formatBytes(preview.size) + "</span>" +
        (preview.lines > 0 ? '<span>' + preview.lines + " lines</span>" : "") +
        (preview.language ? '<span class="type-pill">' + preview.language + "</span>" : "") +
      "</div>" +
    "</div>";

  var content = '<div class="preview-content">';

  switch (preview.type) {
    case "code":
      var lang = preview.language || "plaintext";
      var escaped = escapeHtml(preview.content);

      // Use highlight.js if available.
      if (window.hljs && lang !== "plaintext") {
        try {
          var result = window.hljs.highlight(preview.content, { language: lang });
          escaped = result.value;
        } catch (e) {
          // Fall back to escaped text.
        }
      }

      content += "<pre><code>" + escaped + "</code></pre>";
      if (preview.truncated) {
        content += '<div class="preview-truncated">Showing first ' + preview.lines + " lines</div>";
      }
      break;

    case "image":
      content += '<img src="' + preview.content + '" alt="' + escapeAttr(preview.name) + '">';
      break;

    case "directory":
      content += '<div class="empty-state"><div class="empty-icon">\uD83D\uDCC1</div><div class="empty-text">Directory</div></div>';
      break;

    case "binary":
      content += '<div class="empty-state"><div class="empty-icon">\uD83D\uDCC4</div><div class="empty-text">Binary file</div><div class="empty-hint">' + formatBytes(preview.size) + "</div></div>";
      break;

    default:
      content += '<div class="empty-state"><div class="empty-icon">\uD83D\uDCC4</div><div class="empty-text">No preview available</div></div>';
  }

  content += "</div>";

  var actions =
    '<div class="preview-actions">' +
      '<button class="icon-btn" data-action="open">Open</button>' +
      '<button class="icon-btn" data-action="reveal">Reveal</button>' +
      '<button class="icon-btn" data-action="rename">Rename</button>' +
      '<button class="icon-btn" data-action="copy-path">Copy Path</button>' +
      '<button class="icon-btn danger" data-action="delete">Delete</button>' +
    "</div>";

  panel.innerHTML = header + content + actions;

  // Action button handlers.
  panel.querySelectorAll(".icon-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      handleAction(btn.dataset.action, filePath, preview);
    });
  });
}

async function handleAction(action, filePath, preview) {
  switch (action) {
    case "open":
      await api.openFile(filePath);
      break;

    case "reveal":
      await api.revealInFinder(filePath);
      break;

    case "rename":
      var newName = prompt("Rename to:", preview.name);
      if (newName && newName !== preview.name) {
        try {
          var newPath = await api.renameFile(filePath, newName);
          var state = getState();
          navigateTo(state.currentPath);
          initPreview(newPath);
        } catch (err) {
          alert("Rename failed: " + err);
        }
      }
      break;

    case "copy-path":
      if (navigator.clipboard) {
        navigator.clipboard.writeText(filePath);
      }
      break;

    case "delete":
      if (confirm("Move " + preview.name + " to Trash?")) {
        try {
          await api.deleteFile(filePath);
          var state = getState();
          navigateTo(state.currentPath);
          var panel = document.getElementById("preview-panel");
          panel.classList.remove("open");
        } catch (err) {
          alert("Delete failed: " + err);
        }
      }
      break;
  }
}

function typeLabel(preview) {
  switch (preview.type) {
    case "code": return preview.language || "Code";
    case "image": return "Image";
    case "directory": return "Directory";
    case "binary": return "Binary";
    default: return "File";
  }
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function basename(path) {
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
