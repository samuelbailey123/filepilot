// FilePilot — Main Application State & Router

import * as api from "./services/api.js";
import { registerAction, getKeymap, formatKeyCombo } from "./services/shortcuts.js";
import { initSidebar } from "./components/sidebar.js";
import { initPreview } from "./components/preview.js";
import { initSearch, openSearch } from "./components/search.js";
import { initBreadcrumb } from "./components/breadcrumb.js";
import { initContextMenu } from "./components/context-menu.js";
import { initModal } from "./components/modal.js";
import { toggleQuickLook, closeQuickLook, isQuickLookVisible } from "./components/quicklook.js";
import { initOpQueue, handleOpProgress } from "./services/opqueue.js";
import { initSettings, openSettings, isSettingsVisible, loadKeymapFromBackend } from "./services/settings.js";
import { initRenameDialog, openRenameDialog } from "./components/rename-dialog.js";
import { initConnectionDialog } from "./components/connection-dialog.js";
import { isDualPane, toggleDualPane, switchActivePane, addPaneTab, closePaneTab, toggleSyncBrowsing, isSyncBrowsing, syncNavigation } from "./components/pane-container.js";
import { autoSaveSession, restoreLastSession, saveWorkspace, loadWorkspace, listWorkspaces } from "./services/workspaces.js";
import { initCommandPalette, openCommandPalette, closeCommandPalette, isCommandPaletteVisible, trackRecentLocation } from "./components/command-palette.js";
import { initInfoPanel, openInfoPanel, closeInfoPanel, isInfoPanelVisible } from "./components/info-panel.js";
import { initDiffView, openDiffView, closeDiffView, isDiffViewVisible } from "./components/diff-view.js";
import { initTagEditor } from "./components/tag-editor.js";
import { getGitStatusForDir, invalidateGitCache, gitStatusClass, gitStatusLabel } from "./services/gitstatus.js";
import { clipboardCopy, clipboardCut, pasteClipboard, getClipboard, clearClipboard } from "./services/clipboard.js";
import { renderColumnView } from "./views/column.js";
import { renderListView } from "./views/list.js";
import { renderGridView } from "./views/grid.js";
import { renderGraphView } from "./views/graph.js";
import { createPane, createTab, navigatePane, selectInPane, deselectInPane, togglePaneHidden, setPaneView, setPaneFilter, addTab, closeTab, switchTab, toggleSelectInPane, rangeSelectInPane, clearSelectionInPane, selectAllInPane } from "./core/pane.js";

// Global application state.
// The active pane holds the per-pane browsing state. The top-level state
// maintains backward-compatible accessors so the rest of the codebase
// continues to work unchanged in single-pane mode.
var _panes = [];
var _activePaneIndex = 0;

var state = {
  get currentPath()   { return activePane().currentPath; },
  set currentPath(v)  { activePane().currentPath = v; },
  get selectedFile()  { return activePane().selectedFile; },
  set selectedFile(v) { activePane().selectedFile = v; },
  get entries()       { return activePane().entries; },
  set entries(v)      { activePane().entries = v; },
  get view()          { return activePane().view; },
  set view(v)         { activePane().view = v; },
  get showHidden()    { return activePane().showHidden; },
  set showHidden(v)   { activePane().showHidden = v; },
  get filterExt()     { return activePane().filterExt; },
  set filterExt(v)    { activePane().filterExt = v; },
  get columnPaths()   { return activePane().columnPaths; },
  set columnPaths(v)  { activePane().columnPaths = v; },
  theme: localStorage.getItem("fp-theme") || "dark",
  homeDir: null,
  // Expose pane management for dual-pane mode.
  get panes()           { return _panes; },
  get activePaneIndex() { return _activePaneIndex; },
  set activePaneIndex(v) { _activePaneIndex = v; },
};

/**
 * Returns the currently active pane.
 * @returns {Object} The active pane object.
 */
function activePane() {
  if (_panes.length === 0) {
    _panes.push(createPane());
  }
  return _panes[_activePaneIndex] || _panes[0];
}

/**
 * Returns the active pane (exported for external access).
 * @returns {Object} The active pane object.
 */
export function getActivePane() {
  return activePane();
}

export function getState() {
  return state;
}

// --- Multi-Selection API ---

/**
 * Toggle a file in/out of multi-selection (Cmd+click).
 * @param {string} path - The file path.
 */
export function toggleSelectFile(path) {
  toggleSelectInPane(activePane(), path);
  updateStatusBar();
}

/**
 * Range-select files from anchor to target (Shift+click).
 * @param {string} path - The target file path.
 * @param {Array} [visibleEntries] - Ordered visible entries for range calculation.
 */
export function rangeSelectFile(path, visibleEntries) {
  if (!visibleEntries) visibleEntries = state.entries;
  rangeSelectInPane(activePane(), path, visibleEntries);
  updateStatusBar();
}

/**
 * Clear the multi-selection.
 */
export function clearSelection() {
  clearSelectionInPane(activePane());
  updateStatusBar();
}

/**
 * Select all files in the current directory.
 */
export function selectAllFiles() {
  selectAllInPane(activePane());
  updateStatusBar();
  announce(activePane().selectedFiles.size + " items selected");
}

/**
 * Get the current set of selected file paths.
 * @returns {Set} Set of selected paths.
 */
export function getSelectedFiles() {
  return activePane().selectedFiles;
}

// Re-export clipboard state for views.
export { getClipboard } from "./services/clipboard.js";

/**
 * Get the current effective selected paths (multi-selection or single selection).
 * @returns {string[]} Array of selected file paths.
 */
function getSelectedPaths() {
  var sel = activePane().selectedFiles;
  if (sel.size > 0) return Array.from(sel);
  if (state.selectedFile) return [state.selectedFile];
  return [];
}

/**
 * Announce a message to screen readers via the live region.
 * @param {string} message - The announcement text.
 */
export function announce(message) {
  var region = document.getElementById("live-region");
  if (region) {
    region.textContent = message;
    // Clear after a delay so repeated announcements work.
    setTimeout(function () {
      region.textContent = "";
    }, 3000);
  }
}

/**
 * Navigate to a directory.
 * @param {string} dirPath - The directory path to navigate to.
 */
export async function navigateTo(dirPath) {
  if (!dirPath) dirPath = state.homeDir || "/";
  var oldPath = state.currentPath;

  // Push to history stack if the path actually changed.
  if (oldPath && oldPath !== dirPath) {
    var pane = activePane();
    pane.historyBack.push(oldPath);
    if (pane.historyBack.length > 50) pane.historyBack.shift();
    pane.historyForward = [];
  }

  state.currentPath = dirPath;
  state.selectedFile = null;
  clearSelectionInPane(activePane());

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

    // Fetch git status for the directory (async, non-blocking for render).
    fetchGitStatus(dirPath);

    renderCurrentView();
    initBreadcrumb(dirPath);
    updateStatusBar();
    updateNavButtons();
    trackRecentLocation(dirPath);
    announce("Navigated to " + basename(dirPath));

    // Sync browsing: mirror navigation to the other pane.
    if (isSyncBrowsing() && isDualPane()) {
      syncNavigation(state.activePaneIndex, oldPath, dirPath);
    }
  } catch (err) {
    console.error("navigateTo error:", err);
    announce("Failed to open directory");
  }
}

/**
 * Navigate without pushing to history (used by back/forward).
 * @param {string} dirPath - The directory path.
 */
async function _navigateWithoutHistory(dirPath) {
  if (!dirPath) return;
  state.currentPath = dirPath;
  state.selectedFile = null;
  clearSelectionInPane(activePane());

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
    fetchGitStatus(dirPath);
    renderCurrentView();
    initBreadcrumb(dirPath);
    updateStatusBar();
    updateNavButtons();
    trackRecentLocation(dirPath);
  } catch (err) {
    console.error("navigate error:", err);
  }
}

/**
 * Navigate back in history.
 */
export function navigateBack() {
  var pane = activePane();
  if (pane.historyBack.length === 0) return;
  var prev = pane.historyBack.pop();
  if (state.currentPath) {
    pane.historyForward.push(state.currentPath);
  }
  _navigateWithoutHistory(prev);
}

/**
 * Navigate forward in history.
 */
export function navigateForward() {
  var pane = activePane();
  if (pane.historyForward.length === 0) return;
  var next = pane.historyForward.pop();
  if (state.currentPath) {
    pane.historyBack.push(state.currentPath);
  }
  _navigateWithoutHistory(next);
}

/**
 * Update the disabled state of the nav back/forward buttons.
 */
function updateNavButtons() {
  var pane = activePane();
  var backBtn = document.getElementById("nav-back");
  var fwdBtn = document.getElementById("nav-forward");
  if (backBtn) backBtn.disabled = pane.historyBack.length === 0;
  if (fwdBtn) fwdBtn.disabled = pane.historyForward.length === 0;
}

/**
 * Select a file (show preview).
 * @param {string} path - The file path to preview.
 */
export async function selectFile(path) {
  state.selectedFile = path;
  var panel = document.getElementById("preview-panel");
  panel.classList.add("open");
  initPreview(path);
  announce("Preview: " + basename(path));
}

/**
 * Deselect file (close preview).
 */
export function deselectFile() {
  state.selectedFile = null;
  var panel = document.getElementById("preview-panel");
  panel.classList.remove("open");
  announce("Preview closed");
}

/**
 * Set active view mode.
 * @param {string} viewName - One of: column, list, grid, graph.
 */
export function setView(viewName) {
  state.view = viewName;
  updateViewSwitcher();
  renderCurrentView();
  announce(viewName.charAt(0).toUpperCase() + viewName.slice(1) + " view");
}

/**
 * Toggle hidden files visibility.
 */
export function toggleHidden() {
  state.showHidden = !state.showHidden;
  navigateTo(state.currentPath);
  announce(state.showHidden ? "Showing hidden files" : "Hiding hidden files");
}

/**
 * Set extension filter.
 * @param {string} ext - The extension to filter by.
 */
export function setFilterExt(ext) {
  state.filterExt = state.filterExt === ext ? null : ext;
  navigateTo(state.currentPath);
}

/**
 * Render the current view.
 */
export function renderCurrentView() {
  var browser = document.getElementById("browser");

  // Apply quick filter if active.
  var pane = activePane();
  if (pane.filterText) {
    var savedEntries = pane.entries;
    pane.entries = applyQuickFilter(savedEntries, pane.filterText);
    renderViewInto(browser);
    pane.entries = savedEntries;
  } else {
    renderViewInto(browser);
  }
}

/**
 * Render the current view mode into the given container.
 * @param {HTMLElement} browser - The browser element.
 */
function renderViewInto(browser) {
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

/**
 * Apply quick filter text to entries.
 * Supports simple glob: "*.js" matches files ending in .js.
 * @param {Array} entries - The entries to filter.
 * @param {string} filterText - The filter query.
 * @returns {Array} Filtered entries.
 */
function applyQuickFilter(entries, filterText) {
  if (!filterText) return entries;
  var query = filterText.toLowerCase();

  // Simple glob support: *.ext
  if (query.startsWith("*.")) {
    var ext = query.slice(2);
    return entries.filter(function (e) {
      return e.isDir || (e.name.toLowerCase().endsWith("." + ext));
    });
  }

  // Plain substring match on name.
  return entries.filter(function (e) {
    return e.name.toLowerCase().indexOf(query) >= 0;
  });
}

/**
 * Update the view switcher buttons.
 */
function updateViewSwitcher() {
  var switcher = document.getElementById("view-switcher");
  var btns = switcher.querySelectorAll(".view-btn");
  btns.forEach(function (btn) {
    var isActive = btn.dataset.view === state.view;
    btn.classList.toggle("active", isActive);
    btn.setAttribute("aria-checked", isActive ? "true" : "false");
  });
}

/**
 * Update the status bar.
 */
function updateStatusBar() {
  var statusbar = document.getElementById("statusbar");
  var dirs = state.entries.filter(function (e) { return e.isDir; }).length;
  var files = state.entries.filter(function (e) { return !e.isDir; }).length;

  // Calculate visible file sizes.
  var totalSize = 0;
  state.entries.forEach(function (e) {
    if (!e.isDir) totalSize += e.size;
  });

  var sizeText = totalSize > 0 ? " \u2022 " + formatSizeCompact(totalSize) : "";

  // Multi-selection indicator.
  var selCount = activePane().selectedFiles.size;
  var selText = selCount > 0 ? '<span class="statusbar-selection">' + selCount + " selected</span> \u2022 " : "";

  statusbar.innerHTML =
    '<div class="statusbar-left">' +
      '<span>' + selText +
      files + " file" + (files !== 1 ? "s" : "") + ", " +
      dirs + " folder" + (dirs !== 1 ? "s" : "") +
      sizeText + "</span>" +
      '<span id="folder-size" class="statusbar-folder-size"></span>' +
    "</div>" +
    '<div class="statusbar-right">' +
      '<span id="disk-usage-status"></span>' +
      '<span id="undo-indicator"></span>' +
    "</div>";

  updateUndoIndicator();
  loadFolderSize();
  loadDiskUsageStatus();
}

/**
 * Load and display the total size of the current folder.
 */
async function loadFolderSize() {
  if (!state.currentPath) return;
  try {
    var size = await api.getFolderSize(state.currentPath);
    var el = document.getElementById("folder-size");
    if (el && size > 0) {
      el.textContent = "\u2022 Folder: " + formatSizeCompact(size);
    }
  } catch (e) {
    // Folder size unavailable.
  }
}

/**
 * Load disk usage for the current path and show in status bar.
 */
async function loadDiskUsageStatus() {
  if (!state.currentPath) return;
  try {
    var usage = await api.getDiskUsage(state.currentPath);
    var el = document.getElementById("disk-usage-status");
    if (el && usage) {
      el.textContent = formatSizeCompact(usage.free) + " free";
    }
  } catch (e) {
    // Disk usage unavailable.
  }
}

function formatSizeCompact(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
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

// --- Git Status ---

var _gitStatuses = {}; // path -> status string for the current directory.

/**
 * Fetch git status for the current directory and update indicators.
 * @param {string} dirPath - Directory path.
 */
async function fetchGitStatus(dirPath) {
  try {
    _gitStatuses = await getGitStatusForDir(dirPath);
  } catch (e) {
    _gitStatuses = {};
  }
  // Update git status indicators on visible entries.
  updateGitIndicators();
}

/**
 * Get the git status map for the current directory.
 * @returns {Object} Map of absolute path to status string.
 */
export function getCurrentGitStatuses() {
  return _gitStatuses;
}

/**
 * Update git status indicator elements in the current view.
 */
function updateGitIndicators() {
  var indicators = document.querySelectorAll("[data-git-path]");
  indicators.forEach(function (el) {
    var path = el.dataset.gitPath;
    var status = _gitStatuses[path];
    if (status) {
      el.className = "git-indicator " + gitStatusClass(status);
      el.textContent = gitStatusLabel(status);
      el.title = status;
    } else {
      el.className = "git-indicator";
      el.textContent = "";
      el.title = "";
    }
  });
}

/**
 * Toggle dark/light theme.
 */
export function toggleTheme() {
  state.theme = state.theme === "dark" ? "light" : "dark";
  document.documentElement.classList.toggle("light", state.theme === "light");
  localStorage.setItem("fp-theme", state.theme);
  announce(state.theme === "light" ? "Light mode" : "Dark mode");
}

// --- Quick Filter ---

var _quickFilterVisible = false;

/**
 * Toggle the quick filter bar visibility.
 */
function toggleQuickFilter() {
  var bar = document.getElementById("quick-filter-bar");
  if (!bar) {
    // Create filter bar dynamically.
    bar = document.createElement("div");
    bar.id = "quick-filter-bar";
    bar.className = "quick-filter-bar";
    bar.innerHTML =
      '<input type="text" class="quick-filter-input" placeholder="Filter files\u2026 (*.js for glob)" spellcheck="false" />' +
      '<button class="quick-filter-clear" aria-label="Clear filter">\u2715</button>';
    var toolbar = document.getElementById("toolbar");
    toolbar.parentNode.insertBefore(bar, toolbar.nextSibling);

    var input = bar.querySelector(".quick-filter-input");
    var clearBtn = bar.querySelector(".quick-filter-clear");

    input.addEventListener("input", function () {
      activePane().filterText = input.value;
      renderCurrentView();
    });

    input.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") {
        ev.preventDefault();
        hideQuickFilter();
      }
    });

    clearBtn.addEventListener("click", function () {
      hideQuickFilter();
    });
  }

  if (_quickFilterVisible) {
    hideQuickFilter();
  } else {
    bar.classList.add("visible");
    _quickFilterVisible = true;
    var input = bar.querySelector(".quick-filter-input");
    input.value = activePane().filterText || "";
    input.focus();
    input.select();
  }
}

/**
 * Hide the quick filter bar and clear filter.
 */
function hideQuickFilter() {
  var bar = document.getElementById("quick-filter-bar");
  if (bar) {
    bar.classList.remove("visible");
    var input = bar.querySelector(".quick-filter-input");
    input.value = "";
  }
  _quickFilterVisible = false;
  activePane().filterText = "";
  renderCurrentView();
}

// --- Initialization ---

async function init() {
  // Apply saved theme.
  if (state.theme === "light") {
    document.documentElement.classList.add("light");
  }

  // Initialize modal system (must be before components that use it).
  initModal();

  // Get home directory from Go backend.
  try {
    state.homeDir = await api.getHomeDir();
  } catch (e) {
    state.homeDir = "/Users";
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
    var isActive = v.id === state.view;
    var cls = isActive ? "view-btn active" : "view-btn";
    return '<button class="' + cls + '" data-view="' + v.id + '"' +
      ' role="radio" aria-checked="' + isActive + '">' +
      v.label + "</button>";
  }).join("");

  switcher.addEventListener("click", function (ev) {
    var btn = ev.target.closest(".view-btn");
    if (btn) setView(btn.dataset.view);
  });

  // Initialize components.
  initSidebar();
  initSearch();
  initContextMenu();
  initOpQueue();
  initSettings();
  initRenameDialog();
  initConnectionDialog();
  initCommandPalette();
  initInfoPanel();
  initDiffView();
  initTagEditor();

  // Load keymap from Go backend (merges into localStorage).
  loadKeymapFromBackend();

  // Register global keyboard shortcuts as named actions.
  registerAction("search", "Search files", function () {
    openSearch(false);
  }, { noOverlay: true });

  registerAction("searchInFolder", "Search in folder", function () {
    openSearch(true);
  }, { noOverlay: true });

  registerAction("quickLook", "Quick Look preview", function () {
    toggleQuickLook();
  }, { noOverlay: true });

  registerAction("switchPane", "Switch active pane", function () {
    if (isDualPane()) {
      switchActivePane();
    }
  }, { noOverlay: true });

  registerAction("toggleDualPane", "Toggle dual-pane", function () {
    toggleDualPane();
  }, { noOverlay: true });

  registerAction("escape", "Close overlay / Deselect", function () {
    var searchOverlay = document.getElementById("search-overlay");
    var shortcutsOverlay = document.getElementById("shortcuts-overlay");
    var settingsOverlay = document.getElementById("settings-overlay");
    var modalOverlay = document.getElementById("modal-overlay");

    // Don't handle Escape if a modal is visible (modal handles its own Escape).
    if (modalOverlay && !modalOverlay.classList.contains("hidden")) {
      return;
    }

    if (isSettingsVisible()) {
      // Settings overlay handles its own Escape via its keydown listener.
      return;
    }

    if (isCommandPaletteVisible()) {
      closeCommandPalette();
    } else if (isQuickLookVisible()) {
      closeQuickLook();
    } else if (searchOverlay.classList.contains("visible")) {
      searchOverlay.classList.remove("visible");
      searchOverlay.setAttribute("aria-hidden", "true");
    } else if (shortcutsOverlay.classList.contains("visible")) {
      shortcutsOverlay.classList.remove("visible");
      shortcutsOverlay.setAttribute("aria-hidden", "true");
    } else if (settingsOverlay && settingsOverlay.classList.contains("visible")) {
      settingsOverlay.classList.remove("visible");
      settingsOverlay.setAttribute("aria-hidden", "true");
    } else if (activePane().selectedFiles.size > 0) {
      clearSelection();
      renderCurrentView();
    } else if (state.selectedFile) {
      deselectFile();
    }
  }, { inInput: true });

  registerAction("showShortcuts", "Show keyboard shortcuts", function () {
    var overlay = document.getElementById("shortcuts-overlay");
    overlay.classList.toggle("visible");
    overlay.setAttribute("aria-hidden", !overlay.classList.contains("visible"));
    // Rebuild shortcuts content with current keymap when opening.
    if (overlay.classList.contains("visible")) {
      buildShortcutsOverlay();
    }
  }, { noOverlay: true });

  registerAction("undo", "Undo last operation", function () {
    api.undo().then(function (path) {
      if (path) {
        navigateTo(state.currentPath);
        announce("Undid last operation");
      }
    });
  }, { inInput: false, noOverlay: true });

  registerAction("toggleHidden", "Toggle hidden files", function () {
    toggleHidden();
  }, { noOverlay: true });

  registerAction("toggleTheme", "Toggle dark/light theme", function () {
    toggleTheme();
  }, { noOverlay: true });

  registerAction("openSettings", "Open settings", function () {
    openSettings();
  }, { noOverlay: true });

  registerAction("syncBrowsing", "Toggle sync browsing", function () {
    toggleSyncBrowsing();
  }, { noOverlay: true });

  registerAction("newTab", "New tab", function () {
    addPaneTab();
  }, { noOverlay: true });

  registerAction("closeTab", "Close tab", function () {
    closePaneTab();
  }, { noOverlay: true });

  registerAction("saveWorkspace", "Save workspace", function () {
    var name = prompt("Workspace name:");
    if (name) saveWorkspace(name);
  }, { noOverlay: true });

  registerAction("selectAll", "Select all files", function () {
    selectAllFiles();
    renderCurrentView();
  }, { noOverlay: true });

  registerAction("quickFilter", "Quick filter", function () {
    toggleQuickFilter();
  }, { noOverlay: true });

  registerAction("commandPalette", "Command palette", function () {
    openCommandPalette();
  }, { noOverlay: true });

  registerAction("getInfo", "Get Info", function () {
    var s = getState();
    if (s.selectedFile) openInfoPanel(s.selectedFile);
  }, { noOverlay: true });

  registerAction("compareFiles", "Compare Files", function () {
    var s = getState();
    if (isDualPane()) {
      var otherPane = _panes[_activePaneIndex === 0 ? 1 : 0];
      if (s.selectedFile && otherPane && otherPane.selectedFile) {
        openDiffView(s.selectedFile, otherPane.selectedFile);
      }
    }
  }, { noOverlay: true });

  registerAction("openTerminal", "Open Terminal Here", function () {
    api.openTerminal(state.currentPath);
  }, { noOverlay: true });

  registerAction("navBack", "Navigate back", function () {
    navigateBack();
  }, { noOverlay: true });

  registerAction("navForward", "Navigate forward", function () {
    navigateForward();
  }, { noOverlay: true });

  registerAction("copyFiles", "Copy files", function () {
    var paths = getSelectedPaths();
    if (paths.length > 0) {
      clipboardCopy(paths);
      announce(paths.length + " item" + (paths.length !== 1 ? "s" : "") + " copied");
      renderCurrentView();
    }
  }, { noOverlay: true });

  registerAction("cutFiles", "Cut files", function () {
    var paths = getSelectedPaths();
    if (paths.length > 0) {
      clipboardCut(paths);
      announce(paths.length + " item" + (paths.length !== 1 ? "s" : "") + " cut");
      renderCurrentView();
    }
  }, { noOverlay: true });

  registerAction("pasteFiles", "Paste files", function () {
    if (getClipboard().paths.length > 0) {
      pasteClipboard(state.currentPath);
    }
  }, { noOverlay: true });

  // Build shortcuts overlay content (uses current keymap).
  buildShortcutsOverlay();

  var shortcutsOverlay = document.getElementById("shortcuts-overlay");
  shortcutsOverlay.addEventListener("click", function (ev) {
    if (ev.target === shortcutsOverlay) {
      shortcutsOverlay.classList.remove("visible");
      shortcutsOverlay.setAttribute("aria-hidden", "true");
    }
  });

  // Nav back/forward buttons.
  var navBackBtn = document.getElementById("nav-back");
  var navFwdBtn = document.getElementById("nav-forward");
  if (navBackBtn) navBackBtn.addEventListener("click", function () { navigateBack(); });
  if (navFwdBtn) navFwdBtn.addEventListener("click", function () { navigateForward(); });

  // Theme toggle button.
  var themeBtn = document.getElementById("theme-toggle");
  if (themeBtn) {
    themeBtn.addEventListener("click", function () {
      toggleTheme();
    });
  }

  // Titlebar double-click to maximize/restore window.
  var titlebar = document.getElementById("titlebar");
  if (titlebar) {
    titlebar.addEventListener("dblclick", function () {
      if (window.runtime) {
        window.runtime.WindowToggleMaximise();
      }
    });
  }

  // Listen for Wails events (wait for runtime to be available).
  waitForRuntime(function () {
    window.runtime.EventsOn("fs:changed", function (path) {
      if (state.currentPath && path.startsWith(state.currentPath)) {
        navigateTo(state.currentPath);
      }
    });

    window.runtime.EventsOn("scan:started", function () {
      showScanToast("Indexing files...");
    });

    window.runtime.EventsOn("scan:complete", function () {
      hideScanToast();
    });

    window.runtime.EventsOn("op:progress", function (op) {
      handleOpProgress(op);
    });
  });

  // Auto-save session on window close.
  window.addEventListener("beforeunload", function () {
    autoSaveSession();
  });

  // Navigate to home directory.
  navigateTo(state.homeDir);
}

/**
 * Wait for Wails runtime to be injected before registering events.
 * @param {Function} callback - Called when window.runtime is available.
 */
function waitForRuntime(callback) {
  if (window.runtime) {
    callback();
    return;
  }
  var attempts = 0;
  var interval = setInterval(function () {
    attempts++;
    if (window.runtime) {
      clearInterval(interval);
      callback();
    } else if (attempts > 50) {
      clearInterval(interval);
      console.warn("Wails runtime not available");
    }
  }, 100);
}

function basename(path) {
  if (!path) return "";
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

/**
 * Build shortcuts overlay content using the current keymap.
 * Called on init and when the overlay is opened to reflect any rebindings.
 */
function buildShortcutsOverlay() {
  var overlay = document.getElementById("shortcuts-overlay");
  var keymap = getKeymap();

  overlay.innerHTML =
    '<div class="shortcuts-card" role="document">' +
      '<div class="shortcuts-title" id="shortcuts-title">Keyboard Shortcuts</div>' +
      shortcutRow(formatKeyCombo(keymap.search || "/"), "Search files") +
      shortcutRow(formatKeyCombo(keymap.searchInFolder || "cmd+shift+f"), "Search in folder") +
      shortcutRow(formatKeyCombo(keymap.escape || "Escape"), "Close overlay / Deselect") +
      shortcutRow(formatKeyCombo(keymap.undo || "cmd+z"), "Undo last operation") +
      shortcutRow(formatKeyCombo(keymap.toggleHidden || "."), "Toggle hidden files") +
      shortcutRow(formatKeyCombo(keymap.toggleTheme || "t"), "Toggle dark/light theme") +
      shortcutRow(formatKeyCombo(keymap.showShortcuts || "?"), "Show this help") +
      shortcutRow("\u2190 \u2192 \u2191 \u2193", "Navigate files") +
      shortcutRow("Enter", "Open file / Enter directory") +
      shortcutRow("Backspace", "Go to parent directory") +
      shortcutRow(formatKeyCombo(keymap.toggleDualPane || "cmd+\\"), "Toggle dual-pane") +
      shortcutRow(formatKeyCombo(keymap.switchPane || "Tab"), "Switch active pane") +
      shortcutRow(formatKeyCombo(keymap.syncBrowsing || "cmd+shift+s"), "Toggle sync browsing") +
      shortcutRow(formatKeyCombo(keymap.newTab || "cmd+t"), "New tab") +
      shortcutRow(formatKeyCombo(keymap.closeTab || "cmd+w"), "Close tab") +
      shortcutRow(formatKeyCombo(keymap.quickLook || " "), "Quick Look preview") +
      shortcutRow(formatKeyCombo(keymap.openSettings || "cmd+,"), "Open settings") +
      shortcutRow(formatKeyCombo(keymap.saveWorkspace || "cmd+shift+w"), "Save workspace") +
      shortcutRow("Dbl-click title", "Maximize / Restore") +
    "</div>";
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
