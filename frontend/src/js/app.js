// Decima Explorer — Main Application State & Router

import * as api from "./services/api.js";
import { register } from "./services/shortcuts.js";
import { initSidebar } from "./components/sidebar.js";
import { initPreview } from "./components/preview.js";
import { initSearch } from "./components/search.js";
import { initBreadcrumb } from "./components/breadcrumb.js";
import { initContextMenu } from "./components/context-menu.js";
import { renderColumnView } from "./views/column.js";
import { renderListView } from "./views/list.js";
import { renderGridView } from "./views/grid.js";
import { renderGraphView } from "./views/graph.js";

// Global application state.
const state = {
  currentPath: null,
  selectedFile: null,
  entries: [],
  view: "column",
  showHidden: false,
  filterExt: null,
  columnPaths: [],
  theme: localStorage.getItem("de-theme") || "dark",
};

export function getState() {
  return state;
}

// Navigate to a directory.
export async function navigateTo(dirPath) {
  state.currentPath = dirPath;
  state.selectedFile = null;

  try {
    var entries = await api.listDir(dirPath);
    state.entries = entries || [];

    if (!state.showHidden) {
      state.entries = state.entries.filter(function (e) { return !e.hidden; });
    }

    if (state.filterExt) {
      state.entries = state.entries.filter(function (e) {
        return e.isDir || e.extension === state.filterExt;
      });
    }

    renderCurrentView();
    initBreadcrumb(dirPath);
    updateStatusBar();
  } catch (err) {
    console.error("navigateTo error:", err);
  }
}

// Select a file (show preview).
export async function selectFile(path) {
  state.selectedFile = path;
  var panel = document.getElementById("preview-panel");
  panel.classList.add("open");
  initPreview(path);
}

// Deselect file (close preview).
export function deselectFile() {
  state.selectedFile = null;
  var panel = document.getElementById("preview-panel");
  panel.classList.remove("open");
}

// Set active view mode.
export function setView(viewName) {
  state.view = viewName;
  updateViewSwitcher();
  renderCurrentView();
}

// Toggle hidden files visibility.
export function toggleHidden() {
  state.showHidden = !state.showHidden;
  navigateTo(state.currentPath);
}

// Set extension filter.
export function setFilterExt(ext) {
  state.filterExt = state.filterExt === ext ? null : ext;
  navigateTo(state.currentPath);
}

// Render the current view.
function renderCurrentView() {
  var browser = document.getElementById("browser");

  switch (state.view) {
    case "column":
      renderColumnView(browser);
      break;
    case "list":
      renderListView(browser);
      break;
    case "grid":
      renderGridView(browser);
      break;
    case "graph":
      renderGraphView(browser);
      break;
  }
}

// Update the view switcher buttons.
function updateViewSwitcher() {
  var switcher = document.getElementById("view-switcher");
  var btns = switcher.querySelectorAll(".view-btn");
  btns.forEach(function (btn) {
    btn.classList.toggle("active", btn.dataset.view === state.view);
  });
}

// Update the status bar.
function updateStatusBar() {
  var statusbar = document.getElementById("statusbar");
  var dirs = state.entries.filter(function (e) { return e.isDir; }).length;
  var files = state.entries.filter(function (e) { return !e.isDir; }).length;

  statusbar.innerHTML =
    '<div class="statusbar-left">' +
      '<span>' + files + " file" + (files !== 1 ? "s" : "") + ", " +
      dirs + " folder" + (dirs !== 1 ? "s" : "") + "</span>" +
    "</div>" +
    '<div class="statusbar-right">' +
      '<span id="undo-indicator"></span>' +
    "</div>";

  updateUndoIndicator();
}

async function updateUndoIndicator() {
  try {
    var count = await api.undoCount();
    var el = document.getElementById("undo-indicator");
    if (el) {
      el.textContent = count > 0 ? count + " undoable" : "";
    }
  } catch (e) {
    // Ignore.
  }
}

// Toggle dark/light theme.
export function toggleTheme() {
  state.theme = state.theme === "dark" ? "light" : "dark";
  document.documentElement.classList.toggle("light", state.theme === "light");
  localStorage.setItem("de-theme", state.theme);
}

// --- Initialization ---

function init() {
  // Apply saved theme.
  if (state.theme === "light") {
    document.documentElement.classList.add("light");
  }

  // Build view switcher.
  var switcher = document.getElementById("view-switcher");
  var views = [
    { id: "column", label: "Column" },
    { id: "list", label: "List" },
    { id: "grid", label: "Grid" },
    { id: "graph", label: "Graph" },
  ];
  switcher.innerHTML = views.map(function (v) {
    var cls = v.id === state.view ? "view-btn active" : "view-btn";
    return '<button class="' + cls + '" data-view="' + v.id + '">' + v.label + "</button>";
  }).join("");

  switcher.addEventListener("click", function (ev) {
    var btn = ev.target.closest(".view-btn");
    if (btn) setView(btn.dataset.view);
  });

  // Initialize components.
  initSidebar();
  initSearch();
  initContextMenu();

  // Register global keyboard shortcuts.
  register("/", function () {
    var overlay = document.getElementById("search-overlay");
    overlay.classList.add("visible");
    overlay.setAttribute("aria-hidden", "false");
    var input = overlay.querySelector(".search-input");
    if (input) input.focus();
  }, { noOverlay: true });

  register("Escape", function () {
    var searchOverlay = document.getElementById("search-overlay");
    var shortcutsOverlay = document.getElementById("shortcuts-overlay");
    if (searchOverlay.classList.contains("visible")) {
      searchOverlay.classList.remove("visible");
      searchOverlay.setAttribute("aria-hidden", "true");
    } else if (shortcutsOverlay.classList.contains("visible")) {
      shortcutsOverlay.classList.remove("visible");
      shortcutsOverlay.setAttribute("aria-hidden", "true");
    } else if (state.selectedFile) {
      deselectFile();
    }
  }, { inInput: true });

  register("?", function () {
    var overlay = document.getElementById("shortcuts-overlay");
    overlay.classList.toggle("visible");
    overlay.setAttribute("aria-hidden", !overlay.classList.contains("visible"));
  }, { noOverlay: true });

  register("cmd+z", function () {
    api.undo().then(function (path) {
      if (path) navigateTo(state.currentPath);
    });
  }, { inInput: false, noOverlay: true });

  register(".", function () {
    toggleHidden();
  }, { noOverlay: true });

  register("t", function () {
    toggleTheme();
  }, { noOverlay: true });

  // Build shortcuts overlay content.
  var shortcutsOverlay = document.getElementById("shortcuts-overlay");
  shortcutsOverlay.innerHTML =
    '<div class="shortcuts-card">' +
      '<div class="shortcuts-title">Keyboard Shortcuts</div>' +
      shortcutRow("/", "Search files") +
      shortcutRow("Escape", "Close overlay / Deselect") +
      shortcutRow("\u2318Z", "Undo last operation") +
      shortcutRow(".", "Toggle hidden files") +
      shortcutRow("t", "Toggle dark/light theme") +
      shortcutRow("?", "Show this help") +
      shortcutRow("\u2190 \u2192 \u2191 \u2193", "Navigate files") +
      shortcutRow("Enter", "Open file / Enter directory") +
      shortcutRow("Backspace", "Go to parent directory") +
    "</div>";

  shortcutsOverlay.addEventListener("click", function (ev) {
    if (ev.target === shortcutsOverlay) {
      shortcutsOverlay.classList.remove("visible");
      shortcutsOverlay.setAttribute("aria-hidden", "true");
    }
  });

  // Listen for Wails events.
  if (window.runtime) {
    window.runtime.EventsOn("fs:changed", function (path) {
      // Re-render if the changed path is in the current directory.
      if (state.currentPath && path.startsWith(state.currentPath)) {
        navigateTo(state.currentPath);
      }
    });

    window.runtime.EventsOn("scan:started", function () {
      showScanToast("Indexing files...");
    });

    window.runtime.EventsOn("scan:complete", function (data) {
      hideScanToast();
    });
  }

  // Navigate to home directory.
  navigateTo("/Users/" + getUsername());
}

function getUsername() {
  // Extract from the current path or use a default.
  var parts = window.location.pathname.split("/");
  // In Wails, we can try to get from env or use a reasonable default.
  return "samuelbailey";
}

function shortcutRow(key, desc) {
  return '<div class="shortcut-row">' +
    '<span class="shortcut-desc">' + desc + "</span>" +
    '<span class="shortcut-key">' + key + "</span>" +
  "</div>";
}

function showScanToast(msg) {
  var toast = document.getElementById("scan-toast");
  toast.classList.remove("hidden");
  toast.innerHTML = '<div class="scan-spinner"></div><span>' + msg + "</span>";
}

function hideScanToast() {
  var toast = document.getElementById("scan-toast");
  toast.classList.add("hidden");
}

// Start when DOM is ready.
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
