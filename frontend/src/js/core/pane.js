// Pane — Encapsulates the state and behaviour of a single browser pane.
// Each pane has its own path, entries, selection, view mode, and filters.
// This enables dual-pane mode where two panes operate independently.
// Panes support multiple tabs, each tab being an independent browsing context.

import * as api from "../services/api.js";

var _paneId = 0;
var _tabId = 0;

/**
 * Create a new tab with default state.
 * @param {Object} opts - Optional initial overrides.
 * @returns {Object} Tab state object.
 */
export function createTab(opts) {
  opts = opts || {};
  return {
    id: _tabId++,
    currentPath: opts.currentPath || null,
    selectedFile: opts.selectedFile || null,
    selectedFiles: new Set(),
    anchorFile: null,
    entries: [],
    view: opts.view || "column",
    showHidden: opts.showHidden || false,
    filterExt: opts.filterExt || null,
    columnPaths: [],
    filterText: "",
    label: opts.label || null,
    historyBack: [],
    historyForward: [],
  };
}

/**
 * Create a new Pane with default state.
 * A pane contains one or more tabs. The active tab provides the pane's state.
 * @param {Object} opts - Optional initial overrides.
 * @returns {Object} Pane state object.
 */
export function createPane(opts) {
  opts = opts || {};
  var firstTab = createTab(opts);
  var pane = {
    id: _paneId++,
    tabs: [firstTab],
    activeTabIndex: 0,
    element: opts.element || null,
  };

  // Proxy tab state through the pane for backward compatibility.
  Object.defineProperties(pane, {
    currentPath:  { get: function () { return activeTab(pane).currentPath; },
                    set: function (v) { activeTab(pane).currentPath = v; }, enumerable: true },
    selectedFile: { get: function () { return activeTab(pane).selectedFile; },
                    set: function (v) { activeTab(pane).selectedFile = v; }, enumerable: true },
    entries:      { get: function () { return activeTab(pane).entries; },
                    set: function (v) { activeTab(pane).entries = v; }, enumerable: true },
    view:         { get: function () { return activeTab(pane).view; },
                    set: function (v) { activeTab(pane).view = v; }, enumerable: true },
    showHidden:   { get: function () { return activeTab(pane).showHidden; },
                    set: function (v) { activeTab(pane).showHidden = v; }, enumerable: true },
    filterExt:    { get: function () { return activeTab(pane).filterExt; },
                    set: function (v) { activeTab(pane).filterExt = v; }, enumerable: true },
    columnPaths:    { get: function () { return activeTab(pane).columnPaths; },
                      set: function (v) { activeTab(pane).columnPaths = v; }, enumerable: true },
    filterText:     { get: function () { return activeTab(pane).filterText; },
                      set: function (v) { activeTab(pane).filterText = v; }, enumerable: true },
    selectedFiles:  { get: function () { return activeTab(pane).selectedFiles; },
                      set: function (v) { activeTab(pane).selectedFiles = v; }, enumerable: true },
    anchorFile:     { get: function () { return activeTab(pane).anchorFile; },
                      set: function (v) { activeTab(pane).anchorFile = v; }, enumerable: true },
    historyBack:    { get: function () { return activeTab(pane).historyBack; },
                      set: function (v) { activeTab(pane).historyBack = v; }, enumerable: true },
    historyForward: { get: function () { return activeTab(pane).historyForward; },
                      set: function (v) { activeTab(pane).historyForward = v; }, enumerable: true },
  });

  return pane;
}

/**
 * Get the active tab of a pane.
 * @param {Object} pane - The pane object.
 * @returns {Object} The active tab.
 */
function activeTab(pane) {
  return pane.tabs[pane.activeTabIndex] || pane.tabs[0];
}

/**
 * Add a new tab to a pane.
 * @param {Object} pane - The pane object.
 * @param {Object} opts - Optional tab overrides.
 * @returns {Object} The new tab.
 */
export function addTab(pane, opts) {
  opts = opts || {};
  if (!opts.currentPath) {
    // Default to the current tab's path.
    var current = activeTab(pane);
    opts.currentPath = current ? current.currentPath : null;
  }
  var tab = createTab(opts);
  pane.tabs.push(tab);
  pane.activeTabIndex = pane.tabs.length - 1;
  return tab;
}

/**
 * Close a tab in a pane by index. Cannot close the last tab.
 * @param {Object} pane - The pane object.
 * @param {number} tabIndex - The tab index to close.
 * @returns {boolean} True if the tab was closed.
 */
export function closeTab(pane, tabIndex) {
  if (pane.tabs.length <= 1) return false;
  pane.tabs.splice(tabIndex, 1);
  if (pane.activeTabIndex >= pane.tabs.length) {
    pane.activeTabIndex = pane.tabs.length - 1;
  }
  return true;
}

/**
 * Switch to a tab by index.
 * @param {Object} pane - The pane object.
 * @param {number} tabIndex - The tab index to switch to.
 */
export function switchTab(pane, tabIndex) {
  if (tabIndex >= 0 && tabIndex < pane.tabs.length) {
    pane.activeTabIndex = tabIndex;
  }
}

/**
 * Navigate a pane to a directory.
 * Loads entries, applies filters, and returns the updated pane state.
 * @param {Object} pane - The pane object to navigate.
 * @param {string} dirPath - The directory path.
 * @returns {Promise<Object>} The updated pane.
 */
export async function navigatePane(pane, dirPath) {
  if (!dirPath) return pane;
  pane.currentPath = dirPath;
  pane.selectedFile = null;
  pane.selectedFiles = new Set();
  pane.anchorFile = null;

  var entries = await api.listDir(dirPath);
  pane.entries = entries || [];

  if (!pane.showHidden) {
    pane.entries = pane.entries.filter(function (e) { return !e.hidden; });
  }

  if (pane.filterExt) {
    pane.entries = pane.entries.filter(function (e) {
      return e.isDir || e.extension === pane.filterExt;
    });
  }

  return pane;
}

/**
 * Select a file in the pane.
 * @param {Object} pane - The pane object.
 * @param {string} path - The file path to select.
 * @returns {Object} The updated pane.
 */
export function selectInPane(pane, path) {
  pane.selectedFile = path;
  return pane;
}

/**
 * Deselect the current file in the pane.
 * @param {Object} pane - The pane object.
 * @returns {Object} The updated pane.
 */
export function deselectInPane(pane) {
  pane.selectedFile = null;
  return pane;
}

/**
 * Toggle hidden file visibility for a pane.
 * @param {Object} pane - The pane object.
 * @returns {Object} The updated pane.
 */
export function togglePaneHidden(pane) {
  pane.showHidden = !pane.showHidden;
  return pane;
}

/**
 * Set the view mode for a pane.
 * @param {Object} pane - The pane object.
 * @param {string} viewName - One of: column, list, grid, graph.
 * @returns {Object} The updated pane.
 */
export function setPaneView(pane, viewName) {
  pane.view = viewName;
  return pane;
}

/**
 * Set or toggle the extension filter for a pane.
 * @param {Object} pane - The pane object.
 * @param {string} ext - The extension to filter by.
 * @returns {Object} The updated pane.
 */
export function setPaneFilter(pane, ext) {
  pane.filterExt = pane.filterExt === ext ? null : ext;
  return pane;
}

/**
 * Toggle a file in/out of the multi-selection set.
 * Used for Cmd+click behaviour.
 * @param {Object} pane - The pane object.
 * @param {string} path - The file path to toggle.
 * @returns {Object} The updated pane.
 */
export function toggleSelectInPane(pane, path) {
  if (pane.selectedFiles.has(path)) {
    pane.selectedFiles.delete(path);
  } else {
    pane.selectedFiles.add(path);
  }
  pane.anchorFile = path;
  pane.selectedFile = path;
  return pane;
}

/**
 * Range-select files from the anchor to the target path.
 * Used for Shift+click behaviour. Selects all entries between anchor and target inclusive.
 * @param {Object} pane - The pane object.
 * @param {string} path - The target file path.
 * @param {Array} visibleEntries - The currently visible entries in display order.
 * @returns {Object} The updated pane.
 */
export function rangeSelectInPane(pane, path, visibleEntries) {
  var anchor = pane.anchorFile;
  if (!anchor) {
    pane.selectedFiles = new Set([path]);
    pane.anchorFile = path;
    pane.selectedFile = path;
    return pane;
  }

  var paths = visibleEntries.map(function (e) { return e.path; });
  var anchorIdx = paths.indexOf(anchor);
  var targetIdx = paths.indexOf(path);

  if (anchorIdx < 0) anchorIdx = 0;
  if (targetIdx < 0) return pane;

  var start = Math.min(anchorIdx, targetIdx);
  var end = Math.max(anchorIdx, targetIdx);

  pane.selectedFiles = new Set();
  for (var i = start; i <= end; i++) {
    pane.selectedFiles.add(paths[i]);
  }
  pane.selectedFile = path;
  return pane;
}

/**
 * Clear the multi-selection set.
 * @param {Object} pane - The pane object.
 * @returns {Object} The updated pane.
 */
export function clearSelectionInPane(pane) {
  pane.selectedFiles = new Set();
  pane.anchorFile = null;
  return pane;
}

/**
 * Select all entries in the pane.
 * @param {Object} pane - The pane object.
 * @returns {Object} The updated pane.
 */
export function selectAllInPane(pane) {
  pane.selectedFiles = new Set();
  pane.entries.forEach(function (e) {
    pane.selectedFiles.add(e.path);
  });
  return pane;
}
