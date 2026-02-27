// Context Menu — Right-click file operations with multi-selection awareness.

import * as api from "../services/api.js";
import { getState, navigateTo, announce, getSelectedFiles, clearSelection, getClipboard } from "../app.js";
import { clipboardCopy, clipboardCut, pasteClipboard } from "../services/clipboard.js";
import { initPreview } from "./preview.js";
import { modalConfirm, modalPrompt, modalAlert } from "./modal.js";
import { openRenameDialog } from "./rename-dialog.js";
import { openInfoPanel } from "./info-panel.js";
import { openDiffView } from "./diff-view.js";
import { openTagEditor } from "./tag-editor.js";

/**
 * Initialize the context menu system.
 */
export function initContextMenu() {
  var menu = document.getElementById("context-menu");

  // Listen for custom file context events.
  document.addEventListener("file-context", function (ev) {
    var entry = ev.detail.entry;
    var x = ev.detail.x;
    var y = ev.detail.y;
    showMenu(menu, entry, x, y);
  });

  // Close on any click outside.
  document.addEventListener("click", function () {
    menu.classList.add("hidden");
  });

  // Close on escape.
  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      menu.classList.add("hidden");
    }
  });
}

function showMenu(menu, entry, x, y) {
  var actions = [];
  var selFiles = getSelectedFiles();
  var selCount = selFiles.size;
  var isBatch = selCount > 1;

  // If the right-clicked entry is not in the selection, use it alone.
  var targetPaths = isBatch && selFiles.has(entry.path)
    ? Array.from(selFiles)
    : [entry.path];
  var targetCount = targetPaths.length;

  actions.push({
    icon: "\uD83D\uDCC2", label: "Open", shortcut: "",
    action: function () {
      if (entry.isDir) {
        navigateTo(entry.path);
      } else {
        api.openFile(entry.path);
      }
    },
  });

  actions.push({
    icon: "\uD83D\uDC41\uFE0F", label: "Preview", shortcut: "",
    action: function () {
      var panel = document.getElementById("preview-panel");
      panel.classList.add("open");
      initPreview(entry.path);
    },
  });

  // Open With submenu.
  actions.push({
    icon: "\uD83D\uDD13", label: "Open With\u2026", shortcut: "",
    action: async function () {
      try {
        var apps = await api.listOpenWithApps(entry.path);
        if (!apps || apps.length === 0) {
          announce("No applications found");
          return;
        }
        // Show a simple modal to pick an app.
        var appName = await modalPrompt("Open with:", "", {
          title: "Open With",
          confirmLabel: "Open",
          placeholder: apps.slice(0, 10).join(", "),
        });
        if (appName) {
          await api.openWithApp(entry.path, appName);
        }
      } catch (err) {
        await modalAlert("Open With failed: " + err, { danger: true });
      }
    },
  });

  actions.push({
    icon: "\uD83D\uDD17", label: "Copy Path", shortcut: "",
    action: function () {
      if (navigator.clipboard) {
        var paths = targetPaths.join("\n");
        navigator.clipboard.writeText(paths);
        announce(targetCount > 1 ? targetCount + " paths copied" : "Path copied");
      }
    },
  });

  actions.push({
    icon: "\uD83D\uDD0D", label: "Reveal in Finder", shortcut: "",
    action: function () {
      api.revealInFinder(entry.path);
    },
  });

  actions.push({ separator: true });

  // Clipboard operations.
  actions.push({
    icon: "\uD83D\uDCCB", label: targetCount > 1 ? "Copy " + targetCount + " Items" : "Copy", shortcut: "\u2318C",
    action: function () {
      clipboardCopy(targetPaths);
      announce(targetCount + " item" + (targetCount !== 1 ? "s" : "") + " copied");
    },
  });

  actions.push({
    icon: "\u2702\uFE0F", label: targetCount > 1 ? "Cut " + targetCount + " Items" : "Cut", shortcut: "\u2318X",
    action: function () {
      clipboardCut(targetPaths);
      announce(targetCount + " item" + (targetCount !== 1 ? "s" : "") + " cut");
    },
  });

  var clip = getClipboard();
  if (clip.paths.length > 0) {
    actions.push({
      icon: "\uD83D\uDCCB", label: "Paste (" + clip.paths.length + " item" + (clip.paths.length !== 1 ? "s" : "") + ")", shortcut: "\u2318V",
      action: function () {
        pasteClipboard(getState().currentPath);
      },
    });
  }

  actions.push({ separator: true });

  // Duplicate.
  actions.push({
    icon: "\uD83D\uDCCB", label: "Duplicate", shortcut: "\u2318D",
    action: async function () {
      try {
        for (var i = 0; i < targetPaths.length; i++) {
          await api.duplicateFile(targetPaths[i]);
        }
        navigateTo(getState().currentPath);
        announce(targetCount > 1 ? targetCount + " items duplicated" : "Duplicated");
      } catch (err) {
        await modalAlert("Duplicate failed: " + err, { danger: true });
      }
    },
  });

  actions.push({
    icon: "\u270F\uFE0F", label: "Rename", shortcut: "",
    action: async function () {
      var newName = await modalPrompt("Rename to:", entry.name, {
        title: "Rename",
        confirmLabel: "Rename",
      });
      if (newName && newName !== entry.name) {
        try {
          await api.renameFile(entry.path, newName);
          var s = getState();
          navigateTo(s.currentPath);
          announce("Renamed to " + newName);
        } catch (err) {
          await modalAlert("Rename failed: " + err, { danger: true });
        }
      }
    },
  });

  actions.push({
    icon: "\uD83D\uDD04", label: "Batch Rename\u2026", shortcut: "",
    action: function () {
      var s = getState();
      var files = s.entries
        .filter(function (e) { return !e.isDir; })
        .map(function (e) { return { path: e.path, name: e.name }; });
      if (files.length > 0) {
        openRenameDialog(files);
      } else {
        announce("No files to rename");
      }
    },
  });

  actions.push({
    icon: "\u2B50", label: "Add to Favorites", shortcut: "",
    action: function () {
      api.addFavorite(entry.path, entry.name);
      announce("Added to favorites");
    },
  });

  // Compress.
  actions.push({
    icon: "\uD83D\uDCE6", label: targetCount > 1 ? "Compress " + targetCount + " Items" : "Compress", shortcut: "",
    action: async function () {
      var s = getState();
      var defaultName = targetCount > 1 ? "Archive.zip" : basename(entry.path) + ".zip";
      var archiveName = await modalPrompt("Archive name:", defaultName, {
        title: "Compress",
        confirmLabel: "Create",
      });
      if (archiveName) {
        var outputPath = s.currentPath + "/" + archiveName;
        try {
          await api.compressFiles(targetPaths, outputPath);
          navigateTo(s.currentPath);
          announce("Created " + archiveName);
        } catch (err) {
          await modalAlert("Compress failed: " + err, { danger: true });
        }
      }
    },
  });

  // Create Alias (symlink).
  actions.push({
    icon: "\uD83D\uDD17", label: "Create Alias", shortcut: "",
    action: async function () {
      var s = getState();
      var linkName = entry.name + " alias";
      var aliasName = await modalPrompt("Alias name:", linkName, {
        title: "Create Alias",
        confirmLabel: "Create",
      });
      if (aliasName) {
        var linkPath = s.currentPath + "/" + aliasName;
        try {
          await api.createSymlink(entry.path, linkPath);
          navigateTo(s.currentPath);
          announce("Alias created");
        } catch (err) {
          await modalAlert("Create alias failed: " + err, { danger: true });
        }
      }
    },
  });

  // Copy To / Move To.
  actions.push({
    icon: "\uD83D\uDCC2", label: targetCount > 1 ? "Copy " + targetCount + " Items To\u2026" : "Copy To\u2026", shortcut: "",
    action: async function () {
      try {
        var dest = await api.pickDirectory();
        if (dest) {
          for (var i = 0; i < targetPaths.length; i++) {
            var name = basename(targetPaths[i]);
            await api.copyFile(targetPaths[i], dest + "/" + name);
          }
          announce(targetCount > 1 ? targetCount + " items copied" : "Copied to " + basename(dest));
        }
      } catch (err) {
        await modalAlert("Copy failed: " + err, { danger: true });
      }
    },
  });

  actions.push({
    icon: "\u27A1\uFE0F", label: targetCount > 1 ? "Move " + targetCount + " Items To\u2026" : "Move To\u2026", shortcut: "",
    action: async function () {
      try {
        var dest = await api.pickDirectory();
        if (dest) {
          for (var i = 0; i < targetPaths.length; i++) {
            var name = basename(targetPaths[i]);
            await api.moveFile(targetPaths[i], dest + "/" + name);
          }
          navigateTo(getState().currentPath);
          announce(targetCount > 1 ? targetCount + " items moved" : "Moved to " + basename(dest));
        }
      } catch (err) {
        await modalAlert("Move failed: " + err, { danger: true });
      }
    },
  });

  if (entry.isDir) {
    actions.push({
      icon: "\uD83D\uDCC4", label: "New File Here", shortcut: "",
      action: async function () {
        var name = await modalPrompt("New file name:", "", {
          title: "New File",
          confirmLabel: "Create",
          placeholder: "filename.txt",
        });
        if (name) {
          await api.createNewFile(entry.path + "/" + name);
          navigateTo(entry.path);
          announce("Created " + name);
        }
      },
    });

    actions.push({
      icon: "\uD83D\uDCC1", label: "New Folder Here", shortcut: "",
      action: async function () {
        var name = await modalPrompt("New folder name:", "", {
          title: "New Folder",
          confirmLabel: "Create",
          placeholder: "folder-name",
        });
        if (name) {
          await api.createNewDir(entry.path + "/" + name);
          navigateTo(entry.path);
          announce("Created folder " + name);
        }
      },
    });

    // Open Terminal Here for directories.
    actions.push({
      icon: "\uD83D\uDDA5\uFE0F", label: "Open Terminal Here", shortcut: "",
      action: function () {
        api.openTerminal(entry.path).catch(function (err) {
          modalAlert("Open Terminal failed: " + err, { danger: true });
        });
      },
    });
  }

  // Extract Here for archive files.
  var archiveExts = [".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2"];
  var entryPathLower = entry.path.toLowerCase();
  var isArchive = archiveExts.some(function (ext) {
    return entryPathLower.endsWith(ext);
  });
  if (!entry.isDir && isArchive) {
    actions.push({
      icon: "\uD83D\uDCE6", label: "Extract Here", shortcut: "",
      action: async function () {
        var s = getState();
        try {
          await api.extractArchive(entry.path, s.currentPath);
          navigateTo(s.currentPath);
          announce("Extracted " + entry.name);
        } catch (err) {
          await modalAlert("Extract failed: " + err, { danger: true });
        }
      },
    });
  }

  // Edit Tags for multi-selection.
  if (targetCount > 1) {
    actions.push({
      icon: "\uD83C\uDFF7\uFE0F", label: "Edit Tags\u2026", shortcut: "",
      action: function () {
        openTagEditor(targetPaths);
      },
    });
  }

  // Get Info.
  actions.push({
    icon: "\u2139\uFE0F", label: "Get Info", shortcut: "\u2318I",
    action: function () {
      openInfoPanel(entry.path);
    },
  });

  actions.push({ separator: true });

  // Batch-aware delete.
  var deleteLabel = targetCount > 1 ? "Move " + targetCount + " Items to Trash" : "Move to Trash";
  actions.push({
    icon: "\uD83D\uDDD1\uFE0F", label: deleteLabel, shortcut: "\u2318\u232B",
    danger: true,
    action: async function () {
      var msg = targetCount > 1
        ? "Move " + targetCount + " items to Trash?"
        : "Move " + entry.name + " to Trash?";
      var confirmed = await modalConfirm(msg, {
        title: "Move to Trash",
        confirmLabel: "Move to Trash",
        danger: true,
      });
      if (confirmed) {
        try {
          for (var i = 0; i < targetPaths.length; i++) {
            await api.deleteFile(targetPaths[i]);
          }
          clearSelection();
          navigateTo(getState().currentPath);
          announce(targetCount > 1 ? targetCount + " items moved to Trash" : entry.name + " moved to Trash");
        } catch (err) {
          await modalAlert("Delete failed: " + err, { danger: true });
        }
      }
    },
  });

  // Render menu.
  var html = "";
  var actionItems = [];
  actions.forEach(function (item) {
    if (item.separator) {
      html += '<div class="ctx-separator" role="separator"></div>';
    } else {
      var dangerCls = item.danger ? " danger" : "";
      var shortcut = item.shortcut ? '<span class="ctx-shortcut">' + item.shortcut + "</span>" : "";
      html +=
        '<div class="ctx-item' + dangerCls + '" role="menuitem" tabindex="-1">' +
          '<span class="ctx-icon" aria-hidden="true">' + item.icon + "</span>" +
          '<span>' + item.label + "</span>" +
          shortcut +
        "</div>";
      actionItems.push(item);
    }
  });

  menu.innerHTML = html;
  menu.classList.remove("hidden");

  // Position: keep within viewport.
  var menuW = 220;
  var menuH = menu.offsetHeight || 400;
  var posX = x + menuW > window.innerWidth ? x - menuW : x;
  var posY = y + menuH > window.innerHeight ? y - menuH : y;
  menu.style.left = Math.max(0, posX) + "px";
  menu.style.top = Math.max(0, posY) + "px";

  // Click handlers.
  var allItems = menu.querySelectorAll(".ctx-item");
  allItems.forEach(function (el, i) {
    el.addEventListener("click", function (ev) {
      ev.stopPropagation();
      menu.classList.add("hidden");
      if (actionItems[i] && actionItems[i].action) {
        actionItems[i].action();
      }
    });
  });

  // Focus the first item for keyboard navigation.
  if (allItems.length > 0) {
    allItems[0].focus();
  }

  // Arrow key navigation within context menu.
  menu.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      var focused = menu.querySelector(".ctx-item:focus");
      var idx = Array.prototype.indexOf.call(allItems, focused);
      if (ev.key === "ArrowDown") {
        idx = Math.min(idx + 1, allItems.length - 1);
      } else {
        idx = Math.max(idx - 1, 0);
      }
      allItems[idx].focus();
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      var focused = menu.querySelector(".ctx-item:focus");
      if (focused) focused.click();
    }
  });
}

function basename(path) {
  if (!path) return "";
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}
