// Workspaces — Save and restore complete window state.

import * as api from "./api.js";
import { getState, getActivePane, announce } from "../app.js";

/**
 * Save the current application state as a workspace.
 * @param {string} name - Workspace name.
 */
export async function saveWorkspace(name) {
  if (!name) return;

  var state = getState();
  var workspace = {
    version: 1,
    timestamp: new Date().toISOString(),
    theme: state.theme,
    panes: state.panes.map(function (pane) {
      return {
        tabs: pane.tabs.map(function (tab) {
          return {
            currentPath: tab.currentPath,
            view: tab.view,
            showHidden: tab.showHidden,
            filterExt: tab.filterExt,
          };
        }),
        activeTabIndex: pane.activeTabIndex,
      };
    }),
    activePaneIndex: state.activePaneIndex,
  };

  try {
    await api.saveWorkspace(name, JSON.stringify(workspace));
    announce("Workspace saved: " + name);
  } catch (err) {
    console.error("Failed to save workspace:", err);
    announce("Failed to save workspace");
  }
}

/**
 * Load a workspace by name and restore the application state.
 * @param {string} name - Workspace name.
 * @returns {Object|null} The workspace object, or null.
 */
export async function loadWorkspace(name) {
  try {
    var json = await api.loadWorkspace(name);
    if (!json) return null;
    var workspace = JSON.parse(json);
    announce("Workspace loaded: " + name);
    return workspace;
  } catch (err) {
    console.error("Failed to load workspace:", err);
    announce("Failed to load workspace");
    return null;
  }
}

/**
 * List all saved workspace names.
 * @returns {Array<string>}
 */
export async function listWorkspaces() {
  try {
    return (await api.listWorkspaces()) || [];
  } catch (err) {
    return [];
  }
}

/**
 * Delete a saved workspace.
 * @param {string} name - Workspace name.
 */
export async function deleteWorkspace(name) {
  try {
    await api.deleteWorkspace(name);
    announce("Workspace deleted: " + name);
  } catch (err) {
    console.error("Failed to delete workspace:", err);
  }
}

/**
 * Auto-save the current session as the "last-session" workspace.
 * Called on app shutdown.
 */
export async function autoSaveSession() {
  await saveWorkspace("_last-session");
}

/**
 * Attempt to restore the last session workspace.
 * @returns {Object|null} The workspace object, or null.
 */
export async function restoreLastSession() {
  return loadWorkspace("_last-session");
}
