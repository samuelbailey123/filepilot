// FilePilot — Column View — Cascading Finder-style columns.

import * as api from "../services/api.js";
import { getState, navigateTo, selectFile, announce, getCurrentGitStatuses, toggleSelectFile, rangeSelectFile, clearSelection, getSelectedFiles, getClipboard } from "../app.js";
import { gitStatusClass, gitStatusLabel } from "../services/gitstatus.js";
import { makeDraggable, makeDropTarget } from "../services/dragdrop.js";

// Track active path per column depth.
var columnStack = [];

/**
 * Render the cascading column view.
 * @param {HTMLElement} container - The container to render into.
 */
export async function renderColumnView(container) {
  var state = getState();

  // Initialize column stack from current path.
  if (columnStack.length === 0 || !state.currentPath.startsWith(columnStack[0])) {
    columnStack = buildColumnStack(state.currentPath);
  }

  container.innerHTML = '<div class="column-container" role="tree" aria-label="File browser columns"></div>';
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
  col.setAttribute("role", "group");
  col.setAttribute("aria-label", basename(dirPath) || "Root");

  if (entries.length === 0) {
    col.innerHTML = '<div class="empty-state"><div class="empty-text">Empty folder</div></div>';
    wrapper.appendChild(col);
    return;
  }

  var nextDir = columnStack[depth + 1] || null;
  var selFiles = getSelectedFiles();

  // Build a path -> entry lookup for delegated event handlers.
  var entryByPath = {};
  entries.forEach(function (entry) { entryByPath[entry.path] = entry; });

  entries.forEach(function (entry) {
    var item = document.createElement("div");
    item.className = "file-item";
    item.dataset.path = entry.path;
    item.dataset.isDir = entry.isDir;
    item.setAttribute("role", "treeitem");
    item.setAttribute("tabindex", "-1");
    item.setAttribute("aria-expanded", entry.isDir && entry.path === nextDir ? "true" : "false");
    item.setAttribute("aria-label", entry.name + (entry.isDir ? ", folder" : ", " + (entry.extension || "file")));

    // Highlight if this entry is the active directory for next column.
    if (entry.isDir && entry.path === nextDir) {
      item.classList.add("active-dir");
    }

    // Highlight if in multi-selection.
    if (selFiles.has(entry.path)) {
      item.classList.add("selected");
    }

    // Highlight the focused/previewed file.
    if (entry.path === state.selectedFile) {
      item.classList.add("focused");
    }

    var icon = getFileIcon(entry);
    var colorCls = getFileColorClass(entry);
    var nameClass = entry.hidden ? "file-name hidden-file" : "file-name";
    if (entry.isSymlink) nameClass += " symlink-name";
    var arrow = entry.isDir ? '<span class="file-arrow" aria-hidden="true">\u203A</span>' : "";

    var sizeSpan = entry.isDir ? '<span class="file-size" data-size-path="' + escapeAttr(entry.path) + '"><span class="size-spinner"></span></span>' : "";

    // Symlink indicator.
    var symlinkSpan = entry.isSymlink ? '<span class="symlink-indicator" title="' + escapeAttr(entry.symlinkTarget || "") + '">\u2192</span>' : "";

    // Git status indicator.
    var gitStatuses = getCurrentGitStatuses();
    var gitStatus = gitStatuses[entry.path] || "";
    var gitSpan = "";
    if (gitStatus) {
      gitSpan = '<span class="git-indicator ' + gitStatusClass(gitStatus) + '" data-git-path="' + escapeAttr(entry.path) + '" title="' + gitStatus + '">' + gitStatusLabel(gitStatus) + '</span>';
    } else {
      gitSpan = '<span class="git-indicator" data-git-path="' + escapeAttr(entry.path) + '"></span>';
    }

    // Tag dots placeholder.
    var tagSpan = '<span class="tag-dots" data-tag-path="' + escapeAttr(entry.path) + '"></span>';

    item.innerHTML =
      '<span class="file-icon ' + colorCls + '" aria-hidden="true">' + icon + "</span>" +
      '<span class="' + nameClass + '">' + escapeHtml(entry.name) + "</span>" +
      symlinkSpan +
      tagSpan +
      gitSpan +
      sizeSpan +
      arrow;

    // Drag and drop (requires per-element setup).
    makeDraggable(item, entry);
    if (entry.isDir) {
      makeDropTarget(item, entry.path);
    }

    col.appendChild(item);
  });

  // Delegated event handlers on the column to avoid per-item listener overhead.
  col.addEventListener("click", function (ev) {
    var item = ev.target.closest(".file-item");
    if (!item) return;
    var entry = entryByPath[item.dataset.path];
    if (entry) handleItemClick(entry, depth, wrapper, ev, entries);
  });

  col.addEventListener("dblclick", function (ev) {
    var item = ev.target.closest(".file-item");
    if (!item) return;
    if (item.dataset.isDir === "false") {
      api.openFile(item.dataset.path);
    }
  });

  col.addEventListener("contextmenu", function (ev) {
    var item = ev.target.closest(".file-item");
    if (!item) return;
    ev.preventDefault();
    var entry = entryByPath[item.dataset.path];
    document.dispatchEvent(new CustomEvent("file-context", {
      detail: { entry: entry || { path: item.dataset.path, isDir: item.dataset.isDir === "true", name: "" }, x: ev.clientX, y: ev.clientY },
    }));
  });

  wrapper.appendChild(col);

  // Load folder sizes and file tags asynchronously.
  loadFolderSizes(col);
  loadFileTags(col);

  // Arrow key navigation within column.
  col.tabIndex = -1;
  col.addEventListener("keydown", function (ev) {
    handleColumnKeydown(ev, col, depth, wrapper);
  });
}

function handleItemClick(entry, depth, wrapper, ev, columnEntries) {
  // Cmd+click: toggle selection without navigating.
  if (ev && (ev.metaKey || ev.ctrlKey)) {
    toggleSelectFile(entry.path);
    updateColumnSelection(wrapper);
    return;
  }

  // Shift+click: range select within the column's entries.
  if (ev && ev.shiftKey && columnEntries) {
    rangeSelectFile(entry.path, columnEntries);
    updateColumnSelection(wrapper);
    return;
  }

  // Plain click: clear selection, standard navigation.
  clearSelection();

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
        item.classList.remove("active-dir", "selected", "focused");
        item.setAttribute("aria-expanded", "false");
        if (item.dataset.path === entry.path) {
          item.classList.add("active-dir");
          item.setAttribute("aria-expanded", "true");
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

    // Highlight focused file in current column.
    updateColumnSelection(wrapper);
  }
}

/**
 * Update file-item CSS classes across all columns to reflect selection state.
 * @param {HTMLElement} wrapper - The column-container element.
 */
function updateColumnSelection(wrapper) {
  var selFiles = getSelectedFiles();
  var state = getState();
  var clip = getClipboard();
  var cutSet = clip.mode === "cut" ? new Set(clip.paths) : new Set();
  wrapper.querySelectorAll(".file-item").forEach(function (item) {
    var path = item.dataset.path;
    item.classList.toggle("selected", selFiles.has(path));
    item.classList.toggle("focused", path === state.selectedFile);
    item.classList.toggle("cut-pending", cutSet.has(path));
  });
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
    items[next].focus();
  } else if (ev.key === "ArrowUp") {
    ev.preventDefault();
    var prev = Math.max(activeIdx - 1, 0);
    items[prev].click();
    items[prev].focus();
  } else if (ev.key === "ArrowRight") {
    ev.preventDefault();
    var nextCol = wrapper.querySelector('.column[data-depth="' + (depth + 1) + '"]');
    if (nextCol) {
      var firstItem = nextCol.querySelector(".file-item");
      if (firstItem) {
        firstItem.click();
        firstItem.focus();
      }
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

function basename(path) {
  if (!path) return "";
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

/**
 * Get the emoji icon for a file entry.
 * @param {object} entry - The file entry.
 * @returns {string} An emoji representing the file type.
 */
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

/**
 * Get the CSS color class for a file entry based on its type.
 * @param {object} entry - The file entry.
 * @returns {string} A CSS class name for file-type coloring.
 */
export function getFileColorClass(entry) {
  if (entry.isDir) return "ftype-folder";

  var ext = (entry.extension || "").toLowerCase();

  // Code files.
  var codeExts = ["go", "js", "ts", "jsx", "tsx", "py", "rs", "rb", "java", "c", "cpp", "h",
    "cs", "swift", "kt", "php", "sh", "bash", "zsh", "lua", "r", "scala", "zig",
    "html", "css", "scss", "sass", "less", "vue", "svelte", "sql", "graphql"];
  if (codeExts.indexOf(ext) >= 0) return "ftype-code";

  // Data files.
  var dataExts = ["json", "yaml", "yml", "toml", "xml", "csv", "tsv", "plist", "ini", "env", "db", "sqlite"];
  if (dataExts.indexOf(ext) >= 0) return "ftype-data";

  // Document files.
  var docExts = ["md", "txt", "rtf", "doc", "docx", "pdf", "pages", "tex", "org", "rst"];
  if (docExts.indexOf(ext) >= 0) return "ftype-doc";

  // Media files.
  var mediaExts = ["png", "jpg", "jpeg", "gif", "svg", "webp", "ico", "bmp", "tiff",
    "mp3", "wav", "flac", "aac", "ogg", "mp4", "mov", "avi", "mkv", "webm"];
  if (mediaExts.indexOf(ext) >= 0) return "ftype-media";

  // Config files.
  var configExts = ["conf", "cfg", "config", "lock", "editorconfig", "gitignore",
    "gitattributes", "dockerignore", "eslintrc", "prettierrc", "babelrc"];
  if (configExts.indexOf(ext) >= 0) return "ftype-config";

  // Archive files.
  var archiveExts = ["zip", "gz", "tar", "bz2", "xz", "7z", "rar", "dmg", "iso", "pkg", "deb", "rpm"];
  if (archiveExts.indexOf(ext) >= 0) return "ftype-archive";

  return "ftype-doc";
}

/**
 * Load folder sizes asynchronously for all directory items in a column.
 * @param {HTMLElement} col - The column element.
 */
function loadFolderSizes(col) {
  var spans = col.querySelectorAll(".file-size[data-size-path]");
  spans.forEach(function (span) {
    var path = span.dataset.sizePath;
    api.getFolderSize(path).then(function (size) {
      if (!span.isConnected) return;
      if (size >= 0) {
        span.textContent = formatSizeCompact(size);
      } else {
        span.innerHTML = "";
      }
    }).catch(function () {
      if (span.isConnected) span.innerHTML = "";
    });
  });
}

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

function formatSizeCompact(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
