// Column View — Cascading Finder-style columns.

import * as api from "../services/api.js";
import { getState, navigateTo, selectFile } from "../app.js";

// Track active path per column depth.
let columnStack = [];

export async function renderColumnView(container) {
  var state = getState();

  // Initialize column stack from current path.
  if (columnStack.length === 0 || !state.currentPath.startsWith(columnStack[0])) {
    columnStack = buildColumnStack(state.currentPath);
  }

  container.innerHTML = '<div class="column-container"></div>';
  var wrapper = container.querySelector(".column-container");

  // Render each column.
  for (var i = 0; i < columnStack.length; i++) {
    await renderColumn(wrapper, columnStack[i], i);
  }

  // Scroll to last column.
  wrapper.scrollLeft = wrapper.scrollWidth;
}

async function renderColumn(wrapper, dirPath, depth) {
  var state = getState();
  var entries;

  try {
    entries = await api.listDir(dirPath);
    if (!entries) entries = [];
  } catch (e) {
    entries = [];
  }

  if (!state.showHidden) {
    entries = entries.filter(function (e) { return !e.hidden; });
  }

  var col = document.createElement("div");
  col.className = "column";
  col.dataset.path = dirPath;
  col.dataset.depth = depth;

  if (entries.length === 0) {
    col.innerHTML = '<div class="empty-state"><div class="empty-text">Empty folder</div></div>';
    wrapper.appendChild(col);
    return;
  }

  var nextDir = columnStack[depth + 1] || null;

  entries.forEach(function (entry) {
    var item = document.createElement("div");
    item.className = "file-item";
    item.dataset.path = entry.path;
    item.dataset.isDir = entry.isDir;

    // Highlight if this entry is the active directory for next column.
    if (entry.isDir && entry.path === nextDir) {
      item.classList.add("active-dir");
    }

    // Highlight if selected.
    if (entry.path === state.selectedFile) {
      item.classList.add("selected");
    }

    var icon = getFileIcon(entry);
    var nameClass = entry.hidden ? "file-name hidden-file" : "file-name";
    var arrow = entry.isDir ? '<span class="file-arrow">\u203A</span>' : "";

    item.innerHTML =
      '<span class="file-icon">' + icon + "</span>" +
      '<span class="' + nameClass + '">' + escapeHtml(entry.name) + "</span>" +
      arrow;

    item.addEventListener("click", function () {
      handleItemClick(entry, depth, wrapper);
    });

    item.addEventListener("dblclick", function () {
      if (!entry.isDir) {
        api.openFile(entry.path);
      }
    });

    item.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      var event = new CustomEvent("file-context", {
        detail: { entry: entry, x: ev.clientX, y: ev.clientY },
      });
      document.dispatchEvent(event);
    });

    col.appendChild(item);
  });

  wrapper.appendChild(col);

  // Arrow key navigation within column.
  col.tabIndex = -1;
  col.addEventListener("keydown", function (ev) {
    handleColumnKeydown(ev, col, depth, wrapper);
  });
}

function handleItemClick(entry, depth, wrapper) {
  // Truncate column stack to current depth + 1.
  columnStack = columnStack.slice(0, depth + 1);

  if (entry.isDir) {
    columnStack.push(entry.path);

    // Remove columns after current depth.
    var cols = wrapper.querySelectorAll(".column");
    for (var i = depth + 1; i < cols.length; i++) {
      cols[i].remove();
    }

    // Mark active in current column.
    var currentCol = wrapper.querySelector('.column[data-depth="' + depth + '"]');
    if (currentCol) {
      currentCol.querySelectorAll(".file-item").forEach(function (item) {
        item.classList.remove("active-dir", "selected");
        if (item.dataset.path === entry.path) {
          item.classList.add("active-dir");
        }
      });
    }

    renderColumn(wrapper, entry.path, depth + 1).then(function () {
      wrapper.scrollLeft = wrapper.scrollWidth;
    });

    // Update state without full re-render.
    var state = getState();
    state.currentPath = entry.path;
    import("../components/breadcrumb.js").then(function (mod) {
      mod.initBreadcrumb(entry.path);
    });
  } else {
    selectFile(entry.path);

    // Highlight selection in current column.
    var currentCol = wrapper.querySelector('.column[data-depth="' + depth + '"]');
    if (currentCol) {
      currentCol.querySelectorAll(".file-item").forEach(function (item) {
        item.classList.toggle("selected", item.dataset.path === entry.path);
      });
    }
  }
}

function handleColumnKeydown(ev, col, depth, wrapper) {
  var items = col.querySelectorAll(".file-item");
  if (items.length === 0) return;

  var activeIdx = -1;
  items.forEach(function (item, i) {
    if (item.classList.contains("selected") || item.classList.contains("active-dir")) {
      activeIdx = i;
    }
  });

  if (ev.key === "ArrowDown") {
    ev.preventDefault();
    var next = Math.min(activeIdx + 1, items.length - 1);
    items[next].click();
  } else if (ev.key === "ArrowUp") {
    ev.preventDefault();
    var prev = Math.max(activeIdx - 1, 0);
    items[prev].click();
  } else if (ev.key === "ArrowRight") {
    ev.preventDefault();
    var nextCol = wrapper.querySelector('.column[data-depth="' + (depth + 1) + '"]');
    if (nextCol) {
      var firstItem = nextCol.querySelector(".file-item");
      if (firstItem) firstItem.click();
      nextCol.focus();
    }
  } else if (ev.key === "ArrowLeft") {
    ev.preventDefault();
    var prevCol = wrapper.querySelector('.column[data-depth="' + (depth - 1) + '"]');
    if (prevCol) {
      prevCol.focus();
    }
  } else if (ev.key === "Enter") {
    ev.preventDefault();
    if (activeIdx >= 0) {
      var entry = items[activeIdx];
      if (entry.dataset.isDir === "false") {
        api.openFile(entry.dataset.path);
      }
    }
  } else if (ev.key === "Backspace") {
    ev.preventDefault();
    var state = getState();
    var parent = getParentPath(state.currentPath);
    if (parent) navigateTo(parent);
  }
}

function buildColumnStack(path) {
  var parts = path.split("/").filter(Boolean);
  var stack = ["/"];
  var current = "";
  for (var i = 0; i < parts.length; i++) {
    current += "/" + parts[i];
    stack.push(current);
  }
  return stack;
}

function getParentPath(path) {
  if (path === "/") return null;
  var parent = path.substring(0, path.lastIndexOf("/"));
  return parent || "/";
}

export function getFileIcon(entry) {
  if (entry.isDir) return "\uD83D\uDCC1";

  var ext = (entry.extension || "").toLowerCase();
  var icons = {
    go: "\uD83D\uDC39", js: "\uD83D\uDFE8", ts: "\uD83D\uDD37", py: "\uD83D\uDC0D",
    rs: "\u2699\uFE0F", rb: "\uD83D\uDC8E", java: "\u2615",
    md: "\uD83D\uDCDD", txt: "\uD83D\uDCC4", json: "\uD83D\uDCCB",
    yaml: "\uD83D\uDCCB", yml: "\uD83D\uDCCB", toml: "\uD83D\uDCCB",
    html: "\uD83C\uDF10", css: "\uD83C\uDFA8", scss: "\uD83C\uDFA8",
    png: "\uD83D\uDDBC\uFE0F", jpg: "\uD83D\uDDBC\uFE0F", jpeg: "\uD83D\uDDBC\uFE0F",
    gif: "\uD83D\uDDBC\uFE0F", svg: "\uD83D\uDDBC\uFE0F", webp: "\uD83D\uDDBC\uFE0F",
    pdf: "\uD83D\uDCC4", zip: "\uD83D\uDCE6", gz: "\uD83D\uDCE6",
    tar: "\uD83D\uDCE6", dmg: "\uD83D\uDCE6",
    sh: "\uD83D\uDCDC", bash: "\uD83D\uDCDC", zsh: "\uD83D\uDCDC",
    sql: "\uD83D\uDEE2\uFE0F", db: "\uD83D\uDEE2\uFE0F",
    mp3: "\uD83C\uDFB5", wav: "\uD83C\uDFB5", flac: "\uD83C\uDFB5",
    mp4: "\uD83C\uDFAC", mov: "\uD83C\uDFAC", avi: "\uD83C\uDFAC",
  };

  return icons[ext] || "\uD83D\uDCC4";
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
