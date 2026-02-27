// Pane Container — Manages dual-pane layout, tabs, pane switching, and sync browsing.

import { getState, navigateTo, announce, getActivePane } from "../app.js";
import { createPane, navigatePane, addTab, closeTab, switchTab } from "../core/pane.js";
import { renderColumnView } from "../views/column.js";
import { renderListView } from "../views/list.js";
import { renderGridView } from "../views/grid.js";
import { renderGraphView } from "../views/graph.js";

var _dualPaneEnabled = false;
var _syncBrowsing = false;
var _dividerDragging = false;
var _leftWidth = 50; // Percentage.

/**
 * Check whether dual-pane mode is active.
 * @returns {boolean}
 */
export function isDualPane() {
  return _dualPaneEnabled;
}

/**
 * Check whether sync browsing is active.
 * @returns {boolean}
 */
export function isSyncBrowsing() {
  return _syncBrowsing;
}

/**
 * Toggle sync browsing on/off.
 * When active, navigating in one pane mirrors the navigation in the other.
 */
export function toggleSyncBrowsing() {
  _syncBrowsing = !_syncBrowsing;
  announce(_syncBrowsing ? "Sync browsing enabled" : "Sync browsing disabled");
}

/**
 * Handle a navigation in sync browsing mode.
 * Call this after navigating one pane to mirror the path in the other.
 * @param {number} sourcePaneIndex - The pane that was navigated.
 * @param {string} oldPath - The previous path before navigation.
 * @param {string} newPath - The new path after navigation.
 */
export function syncNavigation(sourcePaneIndex, oldPath, newPath) {
  if (!_syncBrowsing || !_dualPaneEnabled) return;
  if (!oldPath || !newPath) return;

  var state = getState();
  var targetIndex = sourcePaneIndex === 0 ? 1 : 0;
  var targetPane = state.panes[targetIndex];
  if (!targetPane) return;

  // Calculate the relative path change.
  var relPath = "";
  if (newPath.startsWith(oldPath + "/")) {
    relPath = newPath.substring(oldPath.length);
  } else if (newPath.length < oldPath.length) {
    // Navigated up: mirror by going up in the target pane too.
    var targetDir = targetPane.currentPath;
    if (targetDir) {
      var parts = targetDir.split("/");
      parts.pop();
      var newTarget = parts.join("/") || "/";
      navigatePane(targetPane, newTarget).then(function () {
        renderPaneView(targetIndex);
      });
    }
    return;
  }

  if (relPath) {
    var targetNewPath = targetPane.currentPath + relPath;
    navigatePane(targetPane, targetNewPath).then(function () {
      renderPaneView(targetIndex);
    });
  }
}

/**
 * Toggle dual-pane mode on/off.
 */
export function toggleDualPane() {
  var state = getState();
  _dualPaneEnabled = !_dualPaneEnabled;

  if (_dualPaneEnabled) {
    // Ensure we have two panes.
    if (state.panes.length < 2) {
      var secondPane = createPane({
        currentPath: state.currentPath,
        view: state.view,
      });
      state.panes.push(secondPane);
    }
    buildDualLayout();
    announce("Dual-pane mode enabled");
  } else {
    // Switch back to single-pane.
    state.activePaneIndex = 0;
    _syncBrowsing = false;
    teardownDualLayout();
    announce("Single-pane mode");
  }
}

/**
 * Switch the active pane (e.g., via Tab key).
 */
export function switchActivePane() {
  var state = getState();
  if (!_dualPaneEnabled) return;

  state.activePaneIndex = state.activePaneIndex === 0 ? 1 : 0;
  updatePaneActiveState();
  announce("Pane " + (state.activePaneIndex + 1) + " active");
}

/**
 * Add a new tab to the active pane.
 */
export function addPaneTab() {
  var pane = getActivePane();
  if (!pane) return;
  addTab(pane);
  renderTabBar(pane);
  navigatePane(pane, pane.currentPath).then(function () {
    renderActivePaneView();
  });
  announce("New tab");
}

/**
 * Close the active tab in the active pane.
 */
export function closePaneTab() {
  var pane = getActivePane();
  if (!pane || pane.tabs.length <= 1) return;
  var closed = closeTab(pane, pane.activeTabIndex);
  if (closed) {
    renderTabBar(pane);
    navigatePane(pane, pane.currentPath).then(function () {
      renderActivePaneView();
    });
    announce("Tab closed");
  }
}

/**
 * Render the active pane's current view into its browser element.
 * @param {number} paneIndex - Which pane to render. Defaults to active.
 */
export function renderPaneView(paneIndex) {
  var state = getState();
  if (paneIndex === undefined) paneIndex = state.activePaneIndex;

  var pane = state.panes[paneIndex];
  if (!pane || !pane.element) return;

  // Render into the content area (below tab bar if present).
  var contentEl = pane.element.querySelector(".pane-content") || pane.element;

  switch (pane.view) {
    case "column":
      renderColumnView(contentEl);
      break;
    case "list":
      renderListView(contentEl);
      break;
    case "grid":
      renderGridView(contentEl);
      break;
    case "graph":
      renderGraphView(contentEl);
      break;
  }
}

/**
 * Render the active pane view (convenience helper).
 */
function renderActivePaneView() {
  var state = getState();
  renderPaneView(state.activePaneIndex);
}

/**
 * Render the tab bar for a pane.
 * @param {Object} pane - The pane object.
 */
function renderTabBar(pane) {
  if (!pane || !pane.element) return;

  // Only show tab bar if there are multiple tabs.
  var existingBar = pane.element.querySelector(".pane-tabs");
  if (pane.tabs.length <= 1) {
    if (existingBar) existingBar.remove();
    // Ensure content area exists.
    if (!pane.element.querySelector(".pane-content")) {
      pane.element.innerHTML = '<div class="pane-content"></div>';
    }
    return;
  }

  var html = '<div class="pane-tabs">';
  pane.tabs.forEach(function (tab, i) {
    var label = tabLabel(tab);
    var active = i === pane.activeTabIndex ? " active" : "";
    html += '<div class="pane-tab' + active + '" data-tab-index="' + i + '">' +
      '<span class="pane-tab-label">' + escapeHtml(label) + '</span>' +
      '<button class="pane-tab-close" data-tab-index="' + i + '" aria-label="Close tab">&times;</button>' +
    '</div>';
  });
  html += '<button class="pane-tab-add" aria-label="New tab">+</button>';
  html += '</div><div class="pane-content"></div>';

  pane.element.innerHTML = html;

  // Bind tab click handlers.
  var tabs = pane.element.querySelectorAll(".pane-tab");
  tabs.forEach(function (tabEl) {
    tabEl.addEventListener("click", function (ev) {
      if (ev.target.classList.contains("pane-tab-close")) return;
      var idx = parseInt(tabEl.dataset.tabIndex, 10);
      switchTab(pane, idx);
      renderTabBar(pane);
      navigatePane(pane, pane.currentPath).then(function () {
        renderActivePaneView();
      });
    });
  });

  // Bind close buttons.
  var closeBtns = pane.element.querySelectorAll(".pane-tab-close");
  closeBtns.forEach(function (btn) {
    btn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      var idx = parseInt(btn.dataset.tabIndex, 10);
      closeTab(pane, idx);
      renderTabBar(pane);
      navigatePane(pane, pane.currentPath).then(function () {
        renderActivePaneView();
      });
    });
  });

  // Bind add button.
  var addBtn = pane.element.querySelector(".pane-tab-add");
  if (addBtn) {
    addBtn.addEventListener("click", function () {
      addTab(pane);
      renderTabBar(pane);
      navigatePane(pane, pane.currentPath).then(function () {
        renderActivePaneView();
      });
    });
  }
}

function tabLabel(tab) {
  if (tab.label) return tab.label;
  if (!tab.currentPath) return "New Tab";
  var parts = tab.currentPath.split("/");
  return parts[parts.length - 1] || "/";
}

/**
 * Build the dual-pane DOM layout.
 */
function buildDualLayout() {
  var browser = document.getElementById("browser");
  var state = getState();

  browser.innerHTML =
    '<div id="pane-left" class="pane active-pane" role="region" aria-label="Left pane">' +
      '<div class="pane-content"></div>' +
    '</div>' +
    '<div id="pane-divider" role="separator" aria-orientation="vertical" aria-label="Resize panes"></div>' +
    '<div id="pane-right" class="pane" role="region" aria-label="Right pane">' +
      '<div class="pane-content"></div>' +
    '</div>';

  var leftEl = document.getElementById("pane-left");
  var rightEl = document.getElementById("pane-right");
  var divider = document.getElementById("pane-divider");

  // Bind pane elements.
  state.panes[0].element = leftEl;
  state.panes[1].element = rightEl;

  // Apply saved widths.
  updateDividerPosition();

  // Render tab bars if needed.
  renderTabBar(state.panes[0]);
  renderTabBar(state.panes[1]);

  // Click to switch active pane.
  leftEl.addEventListener("mousedown", function () {
    if (state.activePaneIndex !== 0) {
      state.activePaneIndex = 0;
      updatePaneActiveState();
    }
  });
  rightEl.addEventListener("mousedown", function () {
    if (state.activePaneIndex !== 1) {
      state.activePaneIndex = 1;
      updatePaneActiveState();
    }
  });

  // Divider drag handler.
  divider.addEventListener("mousedown", function (ev) {
    ev.preventDefault();
    _dividerDragging = true;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  });

  document.addEventListener("mousemove", function (ev) {
    if (!_dividerDragging) return;
    var browser = document.getElementById("browser");
    var rect = browser.getBoundingClientRect();
    _leftWidth = ((ev.clientX - rect.left) / rect.width) * 100;
    _leftWidth = Math.max(20, Math.min(80, _leftWidth));
    updateDividerPosition();
  });

  document.addEventListener("mouseup", function () {
    if (_dividerDragging) {
      _dividerDragging = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    }
  });

  // Render both panes.
  navigatePane(state.panes[0], state.panes[0].currentPath).then(function () {
    renderPaneView(0);
  });
  navigatePane(state.panes[1], state.panes[1].currentPath).then(function () {
    renderPaneView(1);
  });
}

/**
 * Remove dual-pane layout and restore single-pane.
 */
function teardownDualLayout() {
  var browser = document.getElementById("browser");
  browser.innerHTML = "";

  var state = getState();
  state.panes[0].element = browser;

  // Re-render single-pane.
  navigateTo(state.currentPath);
}

/**
 * Update CSS for divider position.
 */
function updateDividerPosition() {
  var leftEl = document.getElementById("pane-left");
  var rightEl = document.getElementById("pane-right");
  if (!leftEl || !rightEl) return;

  leftEl.style.width = _leftWidth + "%";
  rightEl.style.width = (100 - _leftWidth) + "%";
}

/**
 * Update visual active state for panes.
 */
function updatePaneActiveState() {
  var left = document.getElementById("pane-left");
  var right = document.getElementById("pane-right");
  var state = getState();

  if (left && right) {
    left.classList.toggle("active-pane", state.activePaneIndex === 0);
    right.classList.toggle("active-pane", state.activePaneIndex === 1);
  }
}

function escapeHtml(str) {
  if (!str) return "";
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
