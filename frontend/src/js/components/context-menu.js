// Context Menu — Right-click file operations.

import * as api from "../services/api.js";
import { getState, navigateTo } from "../app.js";
import { initPreview } from "./preview.js";

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
  var items = [];

  items.push(menuItem("\uD83D\uDCC2", "Open", "", function () {
    if (entry.isDir) {
      navigateTo(entry.path);
    } else {
      api.openFile(entry.path);
    }
  }));

  items.push(menuItem("\uD83D\uDC41\uFE0F", "Preview", "", function () {
    var panel = document.getElementById("preview-panel");
    panel.classList.add("open");
    initPreview(entry.path);
  }));

  items.push(menuItem("\uD83D\uDCCB", "Copy Path", "\u2318C", function () {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(entry.path);
    }
  }));

  items.push(menuItem("\uD83D\uDD0D", "Reveal in Finder", "", function () {
    api.revealInFinder(entry.path);
  }));

  items.push({ separator: true });

  items.push(menuItem("\u270F\uFE0F", "Rename", "", function () {
    var newName = prompt("Rename to:", entry.name);
    if (newName && newName !== entry.name) {
      api.renameFile(entry.path, newName).then(function () {
        var state = getState();
        navigateTo(state.currentPath);
      }).catch(function (err) {
        alert("Rename failed: " + err);
      });
    }
  }));

  items.push(menuItem("\u2B50", "Add to Favorites", "", function () {
    api.addFavorite(entry.path, entry.name);
  }));

  if (entry.isDir) {
    items.push(menuItem("\uD83D\uDCC4", "New File Here", "", function () {
      var name = prompt("New file name:");
      if (name) {
        api.createNewFile(entry.path + "/" + name).then(function () {
          navigateTo(entry.path);
        });
      }
    }));

    items.push(menuItem("\uD83D\uDCC1", "New Folder Here", "", function () {
      var name = prompt("New folder name:");
      if (name) {
        api.createNewDir(entry.path + "/" + name).then(function () {
          navigateTo(entry.path);
        });
      }
    }));
  }

  items.push({ separator: true });

  items.push(menuItem("\uD83D\uDDD1\uFE0F", "Move to Trash", "\u2318\u232B", function () {
    if (confirm("Move " + entry.name + " to Trash?")) {
      api.deleteFile(entry.path).then(function () {
        var state = getState();
        navigateTo(state.currentPath);
      }).catch(function (err) {
        alert("Delete failed: " + err);
      });
    }
  }, true));

  // Render menu.
  var html = "";
  items.forEach(function (item) {
    if (item.separator) {
      html += '<div class="ctx-separator"></div>';
    } else {
      var dangerCls = item.danger ? " danger" : "";
      var shortcut = item.shortcut ? '<span class="ctx-shortcut">' + item.shortcut + "</span>" : "";
      html +=
        '<div class="ctx-item' + dangerCls + '" data-idx="' + item.idx + '">' +
          '<span class="ctx-icon">' + item.icon + "</span>" +
          '<span>' + item.label + "</span>" +
          shortcut +
        "</div>";
    }
  });

  menu.innerHTML = html;
  menu.classList.remove("hidden");

  // Position: keep within viewport.
  var menuW = 200;
  var menuH = menu.offsetHeight || 300;
  var posX = x + menuW > window.innerWidth ? x - menuW : x;
  var posY = y + menuH > window.innerHeight ? y - menuH : y;
  menu.style.left = Math.max(0, posX) + "px";
  menu.style.top = Math.max(0, posY) + "px";

  // Click handlers.
  var allItems = menu.querySelectorAll(".ctx-item");
  var actionItems = items.filter(function (i) { return !i.separator; });
  allItems.forEach(function (el, i) {
    el.addEventListener("click", function (ev) {
      ev.stopPropagation();
      menu.classList.add("hidden");
      if (actionItems[i] && actionItems[i].action) {
        actionItems[i].action();
      }
    });
  });
}

var menuItemIdx = 0;
function menuItem(icon, label, shortcut, action, danger) {
  return {
    icon: icon,
    label: label,
    shortcut: shortcut || "",
    action: action,
    danger: !!danger,
    idx: menuItemIdx++,
  };
}
