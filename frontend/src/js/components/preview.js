// Preview Panel — File preview with metadata and actions.

import * as api from "../services/api.js";
import { getState, navigateTo, announce } from "../app.js";
import { modalConfirm, modalPrompt, modalAlert } from "./modal.js";
import { loadHighlightJs, loadPdfJs } from "../core/lazy-libs.js";

// Track iCloud polling timer so we can cancel it on navigation.
var _icloudPollTimer = null;

/**
 * Check if a path is a remote path (remote:// prefix).
 * @param {string} p - The path to check.
 * @returns {boolean}
 */
function isRemotePath(p) {
  return p && p.startsWith("remote://");
}

/**
 * Initialize the preview panel for a given file path.
 * @param {string} filePath - The file to preview.
 */
export async function initPreview(filePath) {
  var panel = document.getElementById("preview-panel");

  // Cancel any pending iCloud poll from a previous preview.
  if (_icloudPollTimer) {
    clearTimeout(_icloudPollTimer);
    _icloudPollTimer = null;
  }

  panel.innerHTML =
    '<div class="preview-header"><div class="preview-filename">Loading...</div></div>' +
    '<div class="preview-content"></div>';

  // Remote files get a simplified preview since getPreview only works locally.
  if (isRemotePath(filePath)) {
    renderRemotePreview(panel, filePath);
    return;
  }

  try {
    var preview = await api.getPreview(filePath);

    // If the file is an iCloud placeholder, trigger download and poll.
    if (preview.type === "icloud") {
      await renderPreview(panel, preview, filePath);
      api.requestICloudDownload(filePath).catch(function () {});
      pollICloudReady(filePath, panel, 0);
      return;
    }

    await renderPreview(panel, preview, filePath);
  } catch (err) {
    panel.innerHTML =
      '<div class="preview-header">' +
        '<div class="preview-filename">' + escapeHtml(basename(filePath)) + "</div>" +
        '<div class="preview-meta">Error loading preview</div>' +
      "</div>";
  }
}

/**
 * Render a simplified preview for remote files.
 * Downloads the file via the temp file manager and serves via asset server.
 * @param {HTMLElement} panel - The preview panel element.
 * @param {string} filePath - The remote:// file path.
 */
async function renderRemotePreview(panel, filePath) {
  var name = basename(filePath);
  var ext = name.includes(".") ? name.split(".").pop().toLowerCase() : "";

  var header =
    '<div class="preview-header">' +
      '<div class="preview-filename">' + escapeHtml(name) + '</div>' +
      '<div class="preview-meta">' +
        '<span class="type-pill type-config">Remote</span>' +
      '</div>' +
    '</div>';

  var imageExts = ["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico"];
  var pdfExts = ["pdf"];
  var codeExts = ["txt", "md", "json", "yaml", "yml", "xml", "csv", "log", "conf", "ini", "sh", "bash", "zsh", "py", "js", "ts", "go", "rs", "html", "css"];

  try {
    var localURL = await api.serveRemoteFileURL(filePath);
    if (!localURL) {
      panel.innerHTML = header + '<div class="preview-content"><div class="empty-state"><div class="empty-icon">\uD83C\uDF10</div><div class="empty-text">Remote file</div></div></div>';
      return;
    }

    var content = '<div class="preview-content">';

    if (imageExts.indexOf(ext) >= 0) {
      content += '<img src="' + localURL + '" alt="Preview of ' + escapeAttr(name) + '">';
    } else if (pdfExts.indexOf(ext) >= 0) {
      content += '<div id="pdf-render-container" data-path="' + escapeAttr(filePath) + '">' +
        '<div class="pdf-canvas-wrap"><canvas id="pdf-canvas"></canvas></div>' +
        '<div class="pdf-nav">' +
          '<button class="icon-btn" id="pdf-prev" aria-label="Previous page">&lt;</button>' +
          '<span id="pdf-page-info"></span>' +
          '<button class="icon-btn" id="pdf-next" aria-label="Next page">&gt;</button>' +
        '</div>' +
      '</div>';
    } else {
      content += '<div class="empty-state"><div class="empty-icon">\uD83C\uDF10</div><div class="empty-text">Remote file</div><div class="empty-hint">' + escapeHtml(name) + '</div></div>';
    }

    content += '</div>';

    var actions =
      '<div class="preview-actions">' +
        '<button class="icon-btn" data-action="copy-path" aria-label="Copy remote path">Copy Path</button>' +
      '</div>';

    panel.innerHTML = header + content + actions;

    // Copy path action.
    panel.querySelectorAll(".preview-actions .icon-btn").forEach(function (btn) {
      btn.addEventListener("click", function () {
        if (navigator.clipboard) {
          navigator.clipboard.writeText(filePath);
          announce("Path copied");
        }
      });
    });

    // Render PDF if applicable.
    if (pdfExts.indexOf(ext) >= 0) {
      renderPDFFromURL(localURL);
    }
  } catch (err) {
    panel.innerHTML = header + '<div class="preview-content"><div class="empty-state"><div class="empty-icon">\uD83C\uDF10</div><div class="empty-text">Preview unavailable</div><div class="empty-hint">' + escapeHtml(err.message || String(err)) + '</div></div></div>';
  }
}

/**
 * Poll for an iCloud file to finish downloading, then re-render.
 * Polls every 2s for up to 60s (30 attempts).
 */
function pollICloudReady(filePath, panel, attempt) {
  if (attempt >= 30) return;

  _icloudPollTimer = setTimeout(async function () {
    _icloudPollTimer = null;
    try {
      var preview = await api.getPreview(filePath);
      if (preview.type !== "icloud") {
        await renderPreview(panel, preview, filePath);
        return;
      }
      // Still downloading — keep polling.
      pollICloudReady(filePath, panel, attempt + 1);
    } catch (err) {
      // File might have been replaced — stop polling.
    }
  }, 2000);
}

async function renderPreview(panel, preview, filePath) {
  var header =
    '<div class="preview-header">' +
      '<div class="preview-filename">' + escapeHtml(preview.name) + "</div>" +
      '<div class="preview-meta">' +
        '<span class="type-pill type-' + typeColor(preview) + '">' + typeLabel(preview) + "</span>" +
        '<span>' + formatBytes(preview.size) + "</span>" +
        (preview.lines > 0 ? '<span>' + preview.lines + " lines</span>" : "") +
        (preview.language ? '<span class="type-pill type-code">' + preview.language + "</span>" : "") +
      "</div>" +
    "</div>";

  var content = '<div class="preview-content">';

  switch (preview.type) {
    case "code":
      var lang = preview.language || "plaintext";
      var escaped = escapeHtml(preview.content);

      if (lang !== "plaintext") {
        try {
          var hljs = await loadHighlightJs();
          var result = hljs.highlight(preview.content, { language: lang });
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
      content += '<img src="' + preview.content + '" alt="Preview of ' + escapeAttr(preview.name) + '">';
      break;

    case "directory":
      content += '<div class="empty-state"><div class="empty-icon">\uD83D\uDCC1</div><div class="empty-text">Directory</div></div>';
      break;

    case "pdf":
      content += '<div id="pdf-render-container" data-path="' + escapeAttr(filePath) + '">' +
        renderMetadataCard(preview.metadata, "PDF Document", "\uD83D\uDCC4") +
        '<div class="pdf-canvas-wrap"><canvas id="pdf-canvas"></canvas></div>' +
        '<div class="pdf-nav">' +
          '<button class="icon-btn" id="pdf-prev" aria-label="Previous page">&lt;</button>' +
          '<span id="pdf-page-info"></span>' +
          '<button class="icon-btn" id="pdf-next" aria-label="Next page">&gt;</button>' +
        '</div>' +
      '</div>';
      break;

    case "audio":
      content += renderMetadataCard(preview.metadata, "Audio File", "\uD83C\uDFB5");
      break;

    case "video":
      content += renderMetadataCard(preview.metadata, "Video File", "\uD83C\uDFAC");
      break;

    case "archive":
      content += renderArchiveList(preview);
      break;

    case "font":
      content += renderMetadataCard(preview.metadata, "Font File", "Aa");
      break;

    case "hex":
      content += '<pre class="hex-dump"><code>' + escapeHtml(preview.content) + "</code></pre>";
      break;

    case "icloud":
      content += '<div class="icloud-downloading">' +
        '<div class="icloud-spinner"></div>' +
        '<div class="icloud-text">Downloading from iCloud...</div>' +
        '<div class="icloud-hint">This file is stored in iCloud and is being downloaded.</div>' +
      '</div>';
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
      '<button class="icon-btn" data-action="open" aria-label="Open file">Open</button>' +
      '<button class="icon-btn" data-action="reveal" aria-label="Reveal in Finder">Reveal</button>' +
      '<button class="icon-btn" data-action="rename" aria-label="Rename file">Rename</button>' +
      '<button class="icon-btn" data-action="copy-path" aria-label="Copy file path">Copy Path</button>' +
      '<button class="icon-btn danger" data-action="delete" aria-label="Move file to Trash">Delete</button>' +
    "</div>";

  panel.innerHTML = header + content + actions;

  // Action button handlers.
  panel.querySelectorAll(".preview-actions .icon-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      handleAction(btn.dataset.action, filePath, preview);
    });
  });

  // Render PDF pages if this is a PDF.
  if (preview.type === "pdf") {
    renderPDF(filePath);
  }
}

/**
 * Render a PDF using PDF.js via the asset server.
 * @param {string} filePath - The local file path.
 */
async function renderPDF(filePath) {
  try {
    var fileURL;
    if (isRemotePath(filePath)) {
      fileURL = await api.serveRemoteFileURL(filePath);
    } else {
      fileURL = await api.serveFileURL(filePath);
    }
    if (!fileURL) return;
    await renderPDFFromURL(fileURL);
  } catch (err) {
    console.error("PDF render error:", err);
  }
}

/**
 * Render a PDF from a URL using PDF.js.
 * @param {string} fileURL - The URL to the PDF file.
 */
async function renderPDFFromURL(fileURL) {
  try {
    var pdfjsLib = await loadPdfJs();

    var pdf = await pdfjsLib.getDocument(fileURL).promise;
    var totalPages = pdf.numPages;
    var currentPage = 1;

    var canvas = document.getElementById("pdf-canvas");
    var pageInfo = document.getElementById("pdf-page-info");
    var prevBtn = document.getElementById("pdf-prev");
    var nextBtn = document.getElementById("pdf-next");

    if (!canvas) return;

    async function renderPage(num) {
      var page = await pdf.getPage(num);
      var viewport = page.getViewport({ scale: 1.5 });
      canvas.width = viewport.width;
      canvas.height = viewport.height;
      var ctx = canvas.getContext("2d");
      await page.render({ canvasContext: ctx, viewport: viewport }).promise;
      if (pageInfo) pageInfo.textContent = num + " / " + totalPages;
      if (prevBtn) prevBtn.disabled = num <= 1;
      if (nextBtn) nextBtn.disabled = num >= totalPages;
    }

    await renderPage(currentPage);

    if (prevBtn) {
      prevBtn.addEventListener("click", function () {
        if (currentPage > 1) {
          currentPage--;
          renderPage(currentPage);
        }
      });
    }
    if (nextBtn) {
      nextBtn.addEventListener("click", function () {
        if (currentPage < totalPages) {
          currentPage++;
          renderPage(currentPage);
        }
      });
    }
  } catch (err) {
    console.error("PDF render error:", err);
  }
}

/**
 * Render a metadata card with key-value pairs.
 * @param {Object} metadata - Key-value pairs to display.
 * @param {string} label - Fallback label if metadata is empty.
 * @param {string} icon - Icon or text for the empty state.
 * @returns {string} HTML string.
 */
function renderMetadataCard(metadata, label, icon) {
  if (!metadata || Object.keys(metadata).length === 0) {
    return '<div class="empty-state"><div class="empty-icon">' + escapeHtml(icon) + '</div><div class="empty-text">' + escapeHtml(label) + "</div></div>";
  }

  var html = '<div class="metadata-card">';
  html += '<div class="metadata-icon">' + escapeHtml(icon) + "</div>";
  html += '<table class="metadata-table">';
  var keys = Object.keys(metadata);
  for (var i = 0; i < keys.length; i++) {
    var key = keys[i];
    html += "<tr><td class=\"metadata-key\">" + escapeHtml(key) + "</td>";
    html += "<td class=\"metadata-value\">" + escapeHtml(metadata[key]) + "</td></tr>";
  }
  html += "</table></div>";
  return html;
}

/**
 * Render an archive file listing with summary.
 * @param {Object} preview - The preview object with entries and metadata.
 * @returns {string} HTML string.
 */
function renderArchiveList(preview) {
  var html = '<div class="archive-preview">';
  var totalFiles = (preview.metadata && preview.metadata.Files) || "?";

  html += '<div class="archive-summary">' + escapeHtml(totalFiles) + " files in archive</div>";

  if (preview.entries && preview.entries.length > 0) {
    html += '<div class="archive-list">';
    for (var i = 0; i < preview.entries.length; i++) {
      html += '<div class="archive-entry">' + escapeHtml(preview.entries[i]) + "</div>";
    }
    if (preview.metadata && parseInt(preview.metadata.Files, 10) > preview.entries.length) {
      var remaining = parseInt(preview.metadata.Files, 10) - preview.entries.length;
      html += '<div class="archive-more">... and ' + remaining + " more</div>";
    }
    html += "</div>";
  }

  if (preview.metadata && preview.metadata.Error) {
    html += '<div class="archive-error">' + escapeHtml(preview.metadata.Error) + "</div>";
  }

  html += "</div>";
  return html;
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
      var newName = await modalPrompt("Rename to:", preview.name, {
        title: "Rename",
        confirmLabel: "Rename",
      });
      if (newName && newName !== preview.name) {
        try {
          var newPath = await api.renameFile(filePath, newName);
          var s = getState();
          navigateTo(s.currentPath);
          initPreview(newPath);
          announce("Renamed to " + newName);
        } catch (err) {
          await modalAlert("Rename failed: " + err, { danger: true });
        }
      }
      break;

    case "copy-path":
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(filePath);
        announce("Path copied");
      }
      break;

    case "delete":
      var confirmed = await modalConfirm(
        "Move " + preview.name + " to Trash?",
        { title: "Move to Trash", confirmLabel: "Move to Trash", danger: true }
      );
      if (confirmed) {
        try {
          await api.deleteFile(filePath);
          var s = getState();
          navigateTo(s.currentPath);
          var panel = document.getElementById("preview-panel");
          panel.classList.remove("open");
          announce(preview.name + " moved to Trash");
        } catch (err) {
          await modalAlert("Delete failed: " + err, { danger: true });
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
    case "pdf": return "PDF";
    case "audio": return "Audio";
    case "video": return "Video";
    case "archive": return "Archive";
    case "font": return "Font";
    case "hex": return "Hex Dump";
    case "icloud": return "iCloud";
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
    case "icloud": return "config";
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
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
