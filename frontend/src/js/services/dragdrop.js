// Drag and Drop — Multi-file aware, with Option+drag for copy, cross-pane support.

import * as api from "./api.js";
import { getState, navigateTo, getSelectedFiles } from "../app.js";

var dragData = null;
var dragGhost = null;

/**
 * Make an element draggable (a file or folder item).
 * Supports multi-file drag when selectedFiles contains multiple entries.
 * @param {HTMLElement} el - The element to make draggable.
 * @param {Object} entry - The file entry data.
 */
export function makeDraggable(el, entry) {
  el.draggable = true;

  el.addEventListener("dragstart", function (ev) {
    var selFiles = getSelectedFiles();
    var paths;

    // If the dragged item is in the selection, drag all selected files.
    if (selFiles.size > 1 && selFiles.has(entry.path)) {
      paths = Array.from(selFiles);
    } else {
      paths = [entry.path];
    }

    dragData = {
      paths: paths,
      name: entry.name,
      isDir: entry.isDir,
      count: paths.length,
    };
    ev.dataTransfer.effectAllowed = "copyMove";
    ev.dataTransfer.setData("text/plain", paths.join("\n"));

    // Create a drag ghost with count badge.
    dragGhost = document.createElement("div");
    dragGhost.className = "drag-ghost";
    if (paths.length > 1) {
      dragGhost.innerHTML = entry.name + ' <span class="drag-ghost-badge">' + paths.length + '</span>';
    } else {
      dragGhost.textContent = entry.name;
    }
    document.body.appendChild(dragGhost);
    ev.dataTransfer.setDragImage(dragGhost, 10, 10);

    // Dim the source item.
    el.classList.add("dragging");
  });

  el.addEventListener("dragend", function () {
    el.classList.remove("dragging");
    dragData = null;
    if (dragGhost) {
      dragGhost.remove();
      dragGhost = null;
    }

    // Remove all drop indicators.
    document.querySelectorAll(".drop-target").forEach(function (t) {
      t.classList.remove("drop-target");
    });
    // Remove copy indicators.
    document.querySelectorAll(".drag-copy-indicator").forEach(function (el) {
      el.remove();
    });
  });
}

/**
 * Make a directory item a drop target.
 * Option+drag triggers copy instead of move.
 * @param {HTMLElement} el - The element to make a drop target.
 * @param {string} targetDirPath - The target directory path.
 */
export function makeDropTarget(el, targetDirPath) {
  el.addEventListener("dragover", function (ev) {
    ev.preventDefault();
    if (!dragData) return;

    // Don't allow dropping onto itself or into the same parent.
    var anyMatch = dragData.paths.some(function (p) {
      return p === targetDirPath || p.substring(0, p.lastIndexOf("/")) === targetDirPath;
    });
    if (anyMatch) return;

    // Option/Alt key means copy.
    ev.dataTransfer.dropEffect = ev.altKey ? "copy" : "move";
    el.classList.add("drop-target");
  });

  el.addEventListener("dragleave", function () {
    el.classList.remove("drop-target");
  });

  el.addEventListener("drop", function (ev) {
    ev.preventDefault();
    el.classList.remove("drop-target");

    if (!dragData) return;

    var isCopy = ev.altKey;
    var operation = isCopy ? api.copyFile : api.moveFile;

    // Execute operation for each dragged path.
    var ops = dragData.paths.map(function (srcPath) {
      var srcName = srcPath.split("/").pop();
      var dstPath = targetDirPath + "/" + srcName;

      // Don't drop onto itself.
      if (srcPath === targetDirPath) return Promise.resolve();
      var srcParent = srcPath.substring(0, srcPath.lastIndexOf("/"));
      if (srcParent === targetDirPath) return Promise.resolve();

      return operation(srcPath, dstPath);
    });

    Promise.all(ops)
      .then(function () {
        var state = getState();
        navigateTo(state.currentPath);
      })
      .catch(function (err) {
        console.error("Drop operation failed:", err);
      });

    dragData = null;
  });
}

/**
 * Make the browser area itself a drop target for the current directory.
 * @param {HTMLElement} el - The browser element.
 */
export function makeBrowserDropTarget(el) {
  el.addEventListener("dragover", function (ev) {
    if (!dragData) return;
    // Only respond if not already over a more specific target.
    if (ev.target === el || ev.target.closest("#browser") === el) {
      ev.preventDefault();
      ev.dataTransfer.dropEffect = ev.altKey ? "copy" : "move";
    }
  });

  el.addEventListener("drop", function (ev) {
    if (!dragData) return;
    if (ev.target !== el && ev.target.closest(".file-item")) return;

    ev.preventDefault();
    var state = getState();
    var isCopy = ev.altKey;
    var operation = isCopy ? api.copyFile : api.moveFile;

    var ops = dragData.paths.map(function (srcPath) {
      var srcName = srcPath.split("/").pop();
      var dstPath = state.currentPath + "/" + srcName;

      var srcParent = srcPath.substring(0, srcPath.lastIndexOf("/"));
      if (srcParent === state.currentPath) return Promise.resolve();

      return operation(srcPath, dstPath);
    });

    Promise.all(ops)
      .then(function () {
        navigateTo(state.currentPath);
      })
      .catch(function (err) {
        console.error("Drop to browser failed:", err);
      });

    dragData = null;
  });
}

/**
 * Return current drag data (for views to check).
 * @returns {Object|null} The drag data.
 */
export function getDragData() {
  return dragData;
}
