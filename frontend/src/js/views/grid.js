// FilePilot — Grid View — Icon grid layout.

import { getState, selectFile, navigateTo, getCurrentGitStatuses, toggleSelectFile, rangeSelectFile, clearSelection, getSelectedFiles, renderCurrentView, getClipboard } from "../app.js";
import { getFileIcon, getFileColorClass } from "./column.js";
import { gitStatusClass, gitStatusLabel } from "../services/gitstatus.js";
import * as api from "../services/api.js";
import { makeDraggable, makeDropTarget } from "../services/dragdrop.js";
import { createVirtualScroller } from "../core/virtual-scroller.js";

/**
 * Render the icon grid view.
 * @param {HTMLElement} container - The container to render into.
 */
export function renderGridView(container) {
  var state = getState();
  var entries = state.entries.slice();

  // Sort: directories first, then alphabetical.
  entries.sort(function (a, b) {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.toLowerCase().localeCompare(b.name.toLowerCase());
  });

  if (entries.length === 0) {
    container.innerHTML =
      '<div class="empty-state">' +
        '<div class="empty-icon">\uD83D\uDCC2</div>' +
        '<div class="empty-text">Empty folder</div>' +
      "</div>";
    return;
  }

  var gitStatuses = getCurrentGitStatuses();
  var selFiles = getSelectedFiles();
  var clip = getClipboard();
  var cutSet = clip.mode === "cut" ? new Set(clip.paths) : new Set();

  var IMAGE_EXTS = new Set(["jpg", "jpeg", "png", "gif", "webp", "heic", "tiff", "bmp", "svg"]);
  var GRID_ITEM_WIDTH = 120;
  var GRID_ROW_HEIGHT = 120;
  var GRID_VIRTUAL_THRESHOLD = 300;

  /**
   * Build a single .grid-item DOM element for the entry at the given index.
   * @param {number} idx - Entry index.
   * @returns {HTMLElement}
   */
  function buildGridItem(idx) {
    var entry = entries[idx];
    var icon = getFileIcon(entry);
    var colorCls = getFileColorClass(entry);
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

    var item = document.createElement("div");
    item.className = "grid-item" + selected;
    item.dataset.path = entry.path;
    item.dataset.isDir = entry.isDir;
    item.setAttribute("role", "gridcell");
    item.setAttribute("tabindex", "0");
    item.setAttribute("aria-label", entry.name + (entry.isDir ? ", folder" : ""));

    item.innerHTML =
      '<div class="grid-icon ' + colorCls + '" aria-hidden="true">' + icon + "</div>" +
      '<div class="grid-name"><span class="' + nameCls + '">' + escapeHtml(entry.name) + '</span>' + symlinkSpan + tagSpan + gitSpan + "</div>" +
      (entry.isDir
        ? '<div class="grid-size" data-grid-size="' + escapeAttr(entry.path) + '"><span class="size-spinner"></span></div>'
        : '<div class="grid-size">' + formatBytes(entry.size) + '</div>');

    attachGridItemHandlers(item, entry);
    return item;
  }

  /**
   * Build a row element containing grid items for indices [startIdx, endIdx).
   * Used as a virtual scroller "row" when entries > threshold.
   * @param {number} rowIdx - Row index (each row = itemsPerRow entries).
   * @param {number} itemsPerRow - Number of items per row.
   * @returns {HTMLElement}
   */
  function buildGridRow(rowIdx, itemsPerRow) {
    var startIdx = rowIdx * itemsPerRow;
    var endIdx = Math.min(startIdx + itemsPerRow, entries.length);
    var rowEl = document.createElement("div");
    rowEl.className = "grid-row-virtual";
    rowEl.style.display = "flex";
    rowEl.style.flexWrap = "nowrap";
    for (var j = startIdx; j < endIdx; j++) {
      rowEl.appendChild(buildGridItem(j));
    }
    return rowEl;
  }

  // Create the grid wrapper.
  var gridWrapper = document.createElement("div");
  gridWrapper.className = "grid-container";
  gridWrapper.setAttribute("role", "grid");
  gridWrapper.setAttribute("aria-label", "File grid");
  container.innerHTML = "";
  container.appendChild(gridWrapper);

  // Delegated event handlers on gridWrapper to avoid per-item listener overhead.
  gridWrapper.addEventListener("click", function (ev) {
    var item = ev.target.closest(".grid-item");
    if (!item) return;
    var path = item.dataset.path;
    var isDir = item.dataset.isDir === "true";
    var outerContainer = gridWrapper.parentNode;

    if (ev.metaKey || ev.ctrlKey) {
      toggleSelectFile(path);
      if (outerContainer) updateGridSelection(outerContainer);
      return;
    }

    if (ev.shiftKey) {
      rangeSelectFile(path, entries);
      if (outerContainer) updateGridSelection(outerContainer);
      return;
    }

    clearSelection();
    if (isDir) {
      navigateTo(path);
    } else {
      selectFile(path);
      if (outerContainer) updateGridSelection(outerContainer);
    }
  });

  gridWrapper.addEventListener("dblclick", function (ev) {
    var item = ev.target.closest(".grid-item");
    if (!item) return;
    if (item.dataset.isDir === "false") {
      api.openFile(item.dataset.path);
    }
  });

  gridWrapper.addEventListener("contextmenu", function (ev) {
    var item = ev.target.closest(".grid-item");
    if (!item) return;
    ev.preventDefault();
    var entry = entries.find(function (e) { return e.path === item.dataset.path; });
    document.dispatchEvent(new CustomEvent("file-context", {
      detail: {
        entry: { path: item.dataset.path, isDir: item.dataset.isDir === "true", name: entry ? entry.name : "" },
        x: ev.clientX, y: ev.clientY,
      },
    }));
  });

  if (entries.length > GRID_VIRTUAL_THRESHOLD) {
    // Calculate items per row from container width.
    var containerWidth = container.clientWidth || 600;
    var itemsPerRow = Math.max(1, Math.floor(containerWidth / GRID_ITEM_WIDTH));
    var rowCount = Math.ceil(entries.length / itemsPerRow);

    gridWrapper.style.overflowY = "auto";
    gridWrapper.style.height = "100%";

    createVirtualScroller({
      container: gridWrapper,
      itemHeight: GRID_ROW_HEIGHT,
      totalCount: rowCount,
      renderItem: function (rowIdx) {
        return buildGridRow(rowIdx, itemsPerRow);
      },
      buffer: 5,
    });
  } else {
    // Standard path: render all items directly.
    var frag = document.createDocumentFragment();
    for (var i = 0; i < entries.length; i++) {
      frag.appendChild(buildGridItem(i));
    }
    gridWrapper.appendChild(frag);

    // Load folder sizes.
    gridWrapper.querySelectorAll("[data-grid-size]").forEach(function (cell) {
      var path = cell.dataset.gridSize;
      api.getFolderSize(path).then(function (size) {
        if (!cell.isConnected) return;
        if (size >= 0) cell.textContent = formatBytes(size);
        else cell.innerHTML = "";
      }).catch(function () {
        if (cell.isConnected) cell.innerHTML = "";
      });
    });

    loadFileTags(gridWrapper);
    loadThumbnails(gridWrapper, entries);
  }
}

/**
 * Attach drag-and-drop handlers to a grid item.
 * Click/dblclick/contextmenu are handled via delegation on the gridWrapper.
 * @param {HTMLElement} item - The grid item element.
 * @param {Object} entry - The file entry.
 */
function attachGridItemHandlers(item, entry) {
  makeDraggable(item, entry);
  if (entry.isDir) {
    makeDropTarget(item, entry.path);
  }
}

/**
 * Load image thumbnails for grid items, processing at most 6 concurrently.
 * Replaces the emoji icon with an <img> element on success.
 * @param {HTMLElement} container - The grid container.
 * @param {Array} entries - The sorted entries array.
 */
function loadThumbnails(container, entries) {
  var IMAGE_EXTS = new Set(["jpg", "jpeg", "png", "gif", "webp", "heic", "tiff", "bmp", "svg"]);
  var CONCURRENCY = 6;

  // Collect items that need thumbnails.
  var imageItems = [];
  container.querySelectorAll(".grid-item").forEach(function (item) {
    var path = item.dataset.path;
    if (!path) return;
    var ext = path.split(".").pop().toLowerCase();
    if (IMAGE_EXTS.has(ext)) {
      imageItems.push({ item: item, path: path });
    }
  });

  if (imageItems.length === 0) return;

  var idx = 0;

  function processNext() {
    if (idx >= imageItems.length) return;
    var current = imageItems[idx++];
    var item = current.item;
    var path = current.path;

    api.getThumbnail(path, 128).then(function (dataUrl) {
      if (!item.isConnected || !dataUrl) {
        processNext();
        return;
      }
      var iconEl = item.querySelector(".grid-icon");
      if (iconEl) {
        var img = document.createElement("img");
        img.className = "grid-thumb";
        img.src = dataUrl;
        img.alt = "";
        img.setAttribute("aria-hidden", "true");
        iconEl.innerHTML = "";
        iconEl.appendChild(img);
      }
      processNext();
    }).catch(function () {
      processNext();
    });
  }

  // Launch up to CONCURRENCY workers.
  var workers = Math.min(CONCURRENCY, imageItems.length);
  for (var w = 0; w < workers; w++) {
    processNext();
  }
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

/**
 * Load Finder tags for visible files and render colored dots.
 * @param {HTMLElement} container - The grid container.
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
 * Update grid item CSS classes to reflect current selection state.
 * @param {HTMLElement} container - The grid container.
 */
function updateGridSelection(container) {
  var selFiles = getSelectedFiles();
  var state = getState();
  container.querySelectorAll(".grid-item").forEach(function (item) {
    var path = item.dataset.path;
    item.classList.toggle("selected", selFiles.has(path));
    item.classList.toggle("focused", path === state.selectedFile);
  });
}
