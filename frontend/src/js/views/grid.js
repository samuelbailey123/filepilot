// Grid View — Icon grid layout.

import { getState, selectFile, navigateTo } from "../app.js";
import { getFileIcon } from "./column.js";
import * as api from "../services/api.js";

export function renderGridView(container) {
  var state = getState();
  var entries = state.entries.slice();

  // Sort: directories first, then alphabetical.
  entries.sort(function (a, b) {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.toLowerCase().localeCompare(b.name.toLowerCase());
  });

  if (entries.length === 0) {
    container.innerHTML =
      '<div class="empty-state">' +
        '<div class="empty-icon">\uD83D\uDCC2</div>' +
        '<div class="empty-text">Empty folder</div>' +
      "</div>";
    return;
  }

  var html = '<div class="grid-container">';
  entries.forEach(function (entry) {
    var icon = getFileIcon(entry);
    var selected = entry.path === state.selectedFile ? " selected" : "";

    html +=
      '<div class="grid-item' + selected + '" data-path="' + escapeAttr(entry.path) + '" data-is-dir="' + entry.isDir + '">' +
        '<div class="grid-icon">' + icon + "</div>" +
        '<div class="grid-name">' + escapeHtml(entry.name) + "</div>" +
      "</div>";
  });
  html += "</div>";

  container.innerHTML = html;

  // Click handlers.
  container.querySelectorAll(".grid-item").forEach(function (item) {
    item.addEventListener("click", function () {
      var path = item.dataset.path;
      var isDir = item.dataset.isDir === "true";
      if (isDir) {
        navigateTo(path);
      } else {
        selectFile(path);
        container.querySelectorAll(".grid-item").forEach(function (g) {
          g.classList.toggle("selected", g === item);
        });
      }
    });

    item.addEventListener("dblclick", function () {
      if (item.dataset.isDir === "false") {
        api.openFile(item.dataset.path);
      }
    });

    item.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      document.dispatchEvent(new CustomEvent("file-context", {
        detail: {
          entry: { path: item.dataset.path, isDir: item.dataset.isDir === "true", name: item.querySelector(".grid-name").textContent },
          x: ev.clientX, y: ev.clientY,
        },
      }));
    });
  });
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
