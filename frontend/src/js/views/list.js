// FilePilot — List View — Sortable table-style file listing.

import { getState, selectFile, navigateTo, getCurrentGitStatuses, toggleSelectFile, rangeSelectFile, clearSelection, getSelectedFiles, renderCurrentView, getClipboard } from "../app.js";
import { getFileIcon, getFileColorClass } from "./column.js";
import { gitStatusClass, gitStatusLabel } from "../services/gitstatus.js";
import * as api from "../services/api.js";
import { makeDraggable, makeDropTarget } from "../services/dragdrop.js";
import { createVirtualScroller } from "../core/virtual-scroller.js";

var sortField = "name";
var sortAsc = true;
var _columnWidths = loadColumnWidths();

/**
 * Render the list/table view.
 * @param {HTMLElement} container - The container to render into.
 */
export function renderListView(container) {
  var state = getState();

  // Load persisted sort for this directory.
  var savedSort = loadSortPrefs(state.currentPath);
  if (savedSort) {
    sortField = savedSort.field;
    sortAsc = savedSort.asc;
  }

  var entries = sortEntries(state.entries.slice());

  var gitStatuses = getCurrentGitStatuses();
  var selFiles = getSelectedFiles();
  var clip = getClipboard();
  var cutSet = clip.mode === "cut" ? new Set(clip.paths) : new Set();

  // Build the static header and a scroll body container.
  container.innerHTML =
    '<div class="list-container" role="table" aria-label="File listing">' +
      '<div class="list-header" role="row">' +
        listColHeader("name", "Name", "list-col-name") +
        listColHeader("size", "Size", "list-col-size") +
        listColHeader("modTime", "Modified", "list-col-modified") +
        listColHeader("extension", "Kind", "list-col-kind") +
      '</div>' +
      '<div class="list-body"></div>' +
    '</div>';

  var listContainer = container.querySelector(".list-container");
  var listBody = container.querySelector(".list-body");

  // Delegated event handlers on listBody to avoid per-item listener overhead.
  listBody.addEventListener("click", function (ev) {
    var row = ev.target.closest(".list-row");
    if (!row) return;
    var path = row.dataset.path;
    var isDir = row.dataset.isDir === "true";

    if (ev.metaKey || ev.ctrlKey) {
      toggleSelectFile(path);
      updateRowSelection(listContainer.parentNode || listContainer);
      return;
    }

    if (ev.shiftKey) {
      rangeSelectFile(path, entries);
      updateRowSelection(listContainer.parentNode || listContainer);
      return;
    }

    clearSelection();
    if (isDir) {
      navigateTo(path);
    } else {
      selectFile(path);
      updateRowSelection(listContainer.parentNode || listContainer);
    }
  });

  listBody.addEventListener("dblclick", function (ev) {
    var row = ev.target.closest(".list-row");
    if (!row) return;
    if (row.dataset.isDir === "false") {
      api.openFile(row.dataset.path);
    }
  });

  listBody.addEventListener("contextmenu", function (ev) {
    var row = ev.target.closest(".list-row");
    if (!row) return;
    ev.preventDefault();
    var entry = entries.find(function (e) { return e.path === row.dataset.path; });
    document.dispatchEvent(new CustomEvent("file-context", {
      detail: {
        entry: { path: row.dataset.path, isDir: row.dataset.isDir === "true", name: entry ? entry.name : "" },
        x: ev.clientX, y: ev.clientY,
      },
    }));
  });

  // Apply persisted column widths.
  applyColumnWidths(container);

  // Column sort handlers.
  container.querySelectorAll(".list-col").forEach(function (col) {
    col.addEventListener("click", function () {
      var field = col.dataset.field;
      if (sortField === field) {
        sortAsc = !sortAsc;
      } else {
        sortField = field;
        sortAsc = true;
      }
      saveSortPrefs(state.currentPath, sortField, sortAsc);
      renderListView(container);
    });
  });

  // Column resize handles.
  initColumnResize(container);

  /**
   * Build a single .list-row DOM element for the entry at the given index.
   * @param {number} idx - Entry index in the sorted entries array.
   * @returns {HTMLElement}
   */
  function buildRow(idx) {
    var entry = entries[idx];
    var icon = getFileIcon(entry);
    var colorCls = getFileColorClass(entry);
    var size = entry.isDir ? '<span class="size-spinner"></span>' : formatBytes(entry.size);
    var modified = formatRelTime(entry.modTime);
    var kind = entry.isDir ? "Folder" : (entry.extension || "File").toUpperCase();
    var isSel = selFiles.has(entry.path);
    var isFocused = entry.path === state.selectedFile;
    var selected = isSel ? " selected" : "";
    selected += isFocused ? " focused" : "";
    selected += cutSet.has(entry.path) ? " cut-pending" : "";

    var gitStatus = gitStatuses[entry.path] || "";
    var gitSpan = gitStatus
      ? '<span class="git-indicator ' + gitStatusClass(gitStatus) + '" data-git-path="' + escapeAttr(entry.path) + '" title="' + gitStatus + '">' + gitStatusLabel(gitStatus) + '</span>'
      : '<span class="git-indicator" data-git-path="' + escapeAttr(entry.path) + '"></span>';

    var nameCls = entry.isSymlink ? "symlink-name" : "";
    var symlinkSpan = entry.isSymlink
      ? '<span class="symlink-indicator" title="' + escapeAttr(entry.symlinkTarget || "") + '">\u2192</span>'
      : "";

    var tagSpan = '<span class="tag-dots" data-tag-path="' + escapeAttr(entry.path) + '"></span>';

    var row = document.createElement("div");
    row.className = "list-row" + selected;
    row.dataset.path = entry.path;
    row.dataset.isDir = entry.isDir;
    row.setAttribute("role", "row");
    row.setAttribute("tabindex", "0");
    row.setAttribute("aria-label", entry.name + (entry.isDir ? ", folder" : ", " + kind));

    row.innerHTML =
      '<div class="list-cell list-cell-name" role="cell">' +
        '<span class="file-icon ' + colorCls + '" aria-hidden="true">' + icon + "</span>" +
        '<span class="' + nameCls + '">' + escapeHtml(entry.name) + "</span>" +
        symlinkSpan +
        tagSpan +
        gitSpan +
      "</div>" +
      '<div class="list-cell list-cell-size" role="cell"' +
        (entry.isDir ? ' data-dir-size="' + escapeAttr(entry.path) + '"' : '') + '>' + size + "</div>" +
      '<div class="list-cell list-cell-modified" role="cell">' + modified + "</div>" +
      '<div class="list-cell list-cell-kind" role="cell"><span class="type-pill type-' + getTypeColor(entry) + '">' + kind + "</span></div>";

    attachRowHandlers(row, entry);
    return row;
  }

  var LIST_VIRTUAL_THRESHOLD = 300;
  var LIST_ROW_HEIGHT = 36;

  if (entries.length > LIST_VIRTUAL_THRESHOLD) {
    // Virtual scroll path: only render rows in the viewport.
    listBody.style.overflowY = "auto";
    listBody.style.flex = "1";
    listBody.style.minHeight = "0";

    createVirtualScroller({
      container: listBody,
      itemHeight: LIST_ROW_HEIGHT,
      totalCount: entries.length,
      renderItem: buildRow,
      buffer: 20,
    });
  } else {
    // Standard path: render all rows via innerHTML substitution with DOM elements.
    var frag = document.createDocumentFragment();
    for (var i = 0; i < entries.length; i++) {
      frag.appendChild(buildRow(i));
    }
    listBody.appendChild(frag);

    // Load folder sizes and file tags asynchronously.
    loadDirSizes(listBody);
    loadFileTags(listBody);
  }
}

/**
 * Attach drag-and-drop handlers to a list row.
 * Click/dblclick/contextmenu are handled via delegation on the listBody.
 * @param {HTMLElement} row - The row element.
 * @param {Object} entry - The file entry.
 */
function attachRowHandlers(row, entry) {
  makeDraggable(row, entry);
  if (entry.isDir) {
    makeDropTarget(row, entry.path);
  }
}

/**
 * Load folder sizes asynchronously for all directory rows in the list.
 * @param {HTMLElement} container - The list container element.
 */
function loadDirSizes(container) {
  var cells = container.querySelectorAll("[data-dir-size]");
  cells.forEach(function (cell) {
    var path = cell.dataset.dirSize;
    api.getFolderSize(path).then(function (size) {
      if (!cell.isConnected) return;
      if (size >= 0) {
        cell.textContent = formatBytes(size);
      } else {
        cell.innerHTML = "--";
      }
    }).catch(function () {
      if (cell.isConnected) cell.innerHTML = "--";
    });
  });
}

function listColHeader(field, label, cls) {
  var sortIndicator = sortField === field ? (sortAsc ? " \u25B2" : " \u25BC") : "";
  var sortedCls = sortField === field ? " sorted" : "";
  var ariaSort = sortField === field ? (sortAsc ? "ascending" : "descending") : "none";
  return '<div class="list-col ' + cls + sortedCls + '" data-field="' + field + '"' +
    ' role="columnheader" tabindex="0" aria-sort="' + ariaSort + '">' +
    label + sortIndicator + "</div>";
}

function getTypeColor(entry) {
  if (entry.isDir) return "folder";
  var cls = getFileColorClass(entry);
  return cls.replace("ftype-", "");
}

function sortEntries(entries) {
  return entries.sort(function (a, b) {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;

    var va, vb;
    switch (sortField) {
      case "name":
        va = a.name.toLowerCase();
        vb = b.name.toLowerCase();
        break;
      case "size":
        va = a.size;
        vb = b.size;
        break;
      case "modTime":
        va = a.modTime;
        vb = b.modTime;
        break;
      case "extension":
        va = (a.extension || "").toLowerCase();
        vb = (b.extension || "").toLowerCase();
        break;
      default:
        va = a.name.toLowerCase();
        vb = b.name.toLowerCase();
    }

    var cmp = va < vb ? -1 : va > vb ? 1 : 0;
    return sortAsc ? cmp : -cmp;
  });
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function formatRelTime(unix) {
  var diff = (Date.now() / 1000) - unix;
  if (diff < 60) return "just now";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  if (diff < 604800) return Math.floor(diff / 86400) + "d ago";
  return new Date(unix * 1000).toLocaleDateString();
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

// --- Sort & Column Persistence ---

/**
 * Load persisted sort preferences for a directory.
 * @param {string} dirPath - The directory path.
 * @returns {Object|null} {field, asc} or null if none saved.
 */
function loadSortPrefs(dirPath) {
  try {
    var raw = localStorage.getItem("fp-sort:" + dirPath);
    if (raw) return JSON.parse(raw);
  } catch (e) { /* ignore */ }
  return null;
}

/**
 * Save sort preferences for a directory.
 * @param {string} dirPath - The directory path.
 * @param {string} field - The sort field name.
 * @param {boolean} asc - Sort direction.
 */
function saveSortPrefs(dirPath, field, asc) {
  try {
    localStorage.setItem("fp-sort:" + dirPath, JSON.stringify({ field: field, asc: asc }));
  } catch (e) { /* ignore */ }
}

/**
 * Load persisted column widths from localStorage.
 * @returns {Object} Map of column class -> width in px.
 */
function loadColumnWidths() {
  try {
    var raw = localStorage.getItem("fp-col-widths");
    if (raw) return JSON.parse(raw);
  } catch (e) { /* ignore */ }
  return {};
}

/**
 * Save column widths to localStorage.
 * @param {Object} widths - Map of column class -> width in px.
 */
function saveColumnWidths(widths) {
  try {
    localStorage.setItem("fp-col-widths", JSON.stringify(widths));
  } catch (e) { /* ignore */ }
}

/**
 * Apply persisted column widths to the list header and rows.
 * @param {HTMLElement} container - The list container.
 */
function applyColumnWidths(container) {
  if (!_columnWidths || Object.keys(_columnWidths).length === 0) return;
  var cols = ["list-col-name", "list-col-size", "list-col-modified", "list-col-kind"];
  cols.forEach(function (cls) {
    var w = _columnWidths[cls];
    if (!w) return;
    container.querySelectorAll("." + cls + ", .list-cell-" + cls.replace("list-col-", "")).forEach(function (el) {
      el.style.width = w + "px";
      el.style.minWidth = w + "px";
      el.style.maxWidth = w + "px";
    });
  });
}

/**
 * Initialize drag-based column resizing on header cells.
 * @param {HTMLElement} container - The list container.
 */
function initColumnResize(container) {
  container.querySelectorAll(".list-col").forEach(function (col) {
    var handle = document.createElement("div");
    handle.className = "col-resizer";
    col.style.position = "relative";
    col.appendChild(handle);

    var startX, startW, colCls;

    handle.addEventListener("mousedown", function (ev) {
      ev.preventDefault();
      ev.stopPropagation();
      startX = ev.clientX;
      startW = col.offsetWidth;
      colCls = Array.from(col.classList).find(function (c) { return c.startsWith("list-col-"); });

      function onMouseMove(e) {
        var delta = e.clientX - startX;
        var newW = Math.max(60, startW + delta);
        col.style.width = newW + "px";
        col.style.minWidth = newW + "px";
        col.style.maxWidth = newW + "px";
        // Also resize matching cells.
        if (colCls) {
          var cellCls = "list-cell-" + colCls.replace("list-col-", "");
          container.querySelectorAll("." + cellCls).forEach(function (cell) {
            cell.style.width = newW + "px";
            cell.style.minWidth = newW + "px";
            cell.style.maxWidth = newW + "px";
          });
        }
      }

      function onMouseUp() {
        document.removeEventListener("mousemove", onMouseMove);
        document.removeEventListener("mouseup", onMouseUp);
        // Persist.
        if (colCls) {
          _columnWidths[colCls] = col.offsetWidth;
          saveColumnWidths(_columnWidths);
        }
      }

      document.addEventListener("mousemove", onMouseMove);
      document.addEventListener("mouseup", onMouseUp);
    });
  });
}

/**
 * Load Finder tags for visible files and render colored dots.
 * @param {HTMLElement} container - The list container.
 */
function loadFileTags(container) {
  var spans = container.querySelectorAll(".tag-dots[data-tag-path]");
  spans.forEach(function (span) {
    var path = span.dataset.tagPath;
    api.getFileTags(path).then(function (tags) {
      if (!span.isConnected || !tags || tags.length === 0) return;
      span.innerHTML = tags.map(function (t) {
        var cls = tagColorClass(t);
        return '<span class="tag-dot ' + cls + '" title="' + escapeAttr(t) + '"></span>';
      }).join("");
    }).catch(function () { /* ignore */ });
  });
}

var _tagColorMap = {
  "red": "tag-red", "orange": "tag-orange", "yellow": "tag-yellow",
  "green": "tag-green", "blue": "tag-blue", "purple": "tag-purple",
  "gray": "tag-gray", "grey": "tag-gray",
};

function tagColorClass(tag) {
  var lower = tag.toLowerCase().replace(/\n\d+$/, "");
  return _tagColorMap[lower] || "tag-gray";
}

/**
 * Update row CSS classes to reflect current selection state.
 * @param {HTMLElement} container - The outer container element (browser).
 */
function updateRowSelection(container) {
  var selFiles = getSelectedFiles();
  var st = getState();
  container.querySelectorAll(".list-row").forEach(function (row) {
    var path = row.dataset.path;
    row.classList.toggle("selected", selFiles.has(path));
    row.classList.toggle("focused", path === st.selectedFile);
  });
}
