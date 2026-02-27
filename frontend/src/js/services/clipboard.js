// File Clipboard — In-memory cut/copy/paste for file operations.

import * as api from "./api.js";
import { getState, navigateTo, announce } from "../app.js";

var _clipboard = {
  paths: [],
  mode: null,
};

/**
 * Copy file paths to the clipboard.
 * @param {string[]} paths - The file paths to copy.
 */
export function clipboardCopy(paths) {
  _clipboard = { paths: paths.slice(), mode: "copy" };
}

/**
 * Cut file paths to the clipboard.
 * @param {string[]} paths - The file paths to cut.
 */
export function clipboardCut(paths) {
  _clipboard = { paths: paths.slice(), mode: "cut" };
}

/**
 * Get the current clipboard state.
 * @returns {{ paths: string[], mode: string|null }}
 */
export function getClipboard() {
  return _clipboard;
}

/**
 * Clear the clipboard.
 */
export function clearClipboard() {
  _clipboard = { paths: [], mode: null };
}

/**
 * Paste clipboard contents into a destination directory.
 * Copies or moves files depending on the clipboard mode.
 * @param {string} destDir - The destination directory.
 * @returns {Promise<number>} The number of files pasted.
 */
export async function pasteClipboard(destDir) {
  if (_clipboard.paths.length === 0 || !_clipboard.mode) return 0;

  var paths = _clipboard.paths;
  var mode = _clipboard.mode;
  var count = 0;

  for (var i = 0; i < paths.length; i++) {
    var src = paths[i];
    var name = src.split("/").pop();
    var dst = destDir + "/" + name;

    try {
      if (mode === "cut") {
        await api.moveFile(src, dst);
      } else {
        await api.copyFile(src, dst);
      }
      count++;
    } catch (err) {
      console.error("Paste failed for " + src + ":", err);
    }
  }

  if (mode === "cut") {
    clearClipboard();
  }

  var s = getState();
  navigateTo(s.currentPath);
  announce(count + " item" + (count !== 1 ? "s" : "") + " pasted");
  return count;
}
