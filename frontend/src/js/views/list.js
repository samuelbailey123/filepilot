// List View — Sortable table-style file listing.

import { getState, selectFile, navigateTo } from "../app.js";
import { getFileIcon } from "./column.js";
import * as api from "../services/api.js";

let sortField = "name";
let sortAsc = true;

export function renderListView(container) {
  var state = getState();
  var entries = sortEntries(state.entries.slice());

  var html =
    '<div class="list-container">' +
      '<div class="list-header">' +
        listColHeader("name", "Name", "list-col-name") +
        listColHeader("size", "Size", "list-col-size") +
        listColHeader("modTime", "Modified", "list-col-modified") +
        listColHeader("extension", "Kind", "list-col-kind") +
      "</div>";

  entries.forEach(function (entry) {
    var icon = getFileIcon(entry);
    var size = entry.isDir ? "--" : formatBytes(entry.size);
    var modified = formatRelTime(entry.modTime);
    var kind = entry.isDir ? "Folder" : (entry.extension || "File").toUpperCase();
    var selected = entry.path === state.selectedFile ? " selected" : "";

    html +=
      '<div class="list-row' + selected + '" data-path="' + escapeAttr(entry.path) + '" data-is-dir="' + entry.isDir + '">' +
        '<div class="list-cell list-cell-name">' +
          '<span class="file-icon">' + icon + "</span>" +
          '<span>' + escapeHtml(entry.name) + "</span>" +
        "</div>" +
        '<div class="list-cell list-cell-size">' + size + "</div>" +
        '<div class="list-cell list-cell-modified">' + modified + "</div>" +
        '<div class="list-cell list-cell-kind"><span class="type-pill">' + kind + "</span></div>" +
      "</div>";
  });

  html += "</div>";
  container.innerHTML = html;

  // Column sort handlers.
  container.querySelectorAll(".list-col").forEach(function (col) {
    col.addEventListener("click", function () {
      var field = col.dataset.field;
      if (sortField === field) {
        sortAsc = !sortAsc;
      } else {
        sortField = field;
        sortAsc = true;
      }
      renderListView(container);
    });
  });

  // Row click handlers.
  container.querySelectorAll(".list-row").forEach(function (row) {
    row.addEventListener("click", function () {
      var path = row.dataset.path;
      var isDir = row.dataset.isDir === "true";
      if (isDir) {
        navigateTo(path);
      } else {
        selectFile(path);
        container.querySelectorAll(".list-row").forEach(function (r) {
          r.classList.toggle("selected", r === row);
        });
      }
    });

    row.addEventListener("dblclick", function () {
      if (row.dataset.isDir === "false") {
        api.openFile(row.dataset.path);
      }
    });

    row.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      document.dispatchEvent(new CustomEvent("file-context", {
        detail: {
          entry: { path: row.dataset.path, isDir: row.dataset.isDir === "true", name: row.querySelector("span:last-child").textContent },
          x: ev.clientX, y: ev.clientY,
        },
      }));
    });
  });
}

function listColHeader(field, label, cls) {
  var sortIndicator = sortField === field ? (sortAsc ? " \u25B2" : " \u25BC") : "";
  var sortedCls = sortField === field ? " sorted" : "";
  return '<div class="list-col ' + cls + sortedCls + '" data-field="' + field + '">' + label + sortIndicator + "</div>";
}

function sortEntries(entries) {
  return entries.sort(function (a, b) {
    // Directories first always.
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;

    var va, vb;
    switch (sortField) {
      case "name":
        va = a.name.toLowerCase();
        vb = b.name.toLowerCase();
        break;
      case "size":
        va = a.size;
        vb = b.size;
        break;
      case "modTime":
        va = a.modTime;
        vb = b.modTime;
        break;
      case "extension":
        va = (a.extension || "").toLowerCase();
        vb = (b.extension || "").toLowerCase();
        break;
      default:
        va = a.name.toLowerCase();
        vb = b.name.toLowerCase();
    }

    var cmp = va < vb ? -1 : va > vb ? 1 : 0;
    return sortAsc ? cmp : -cmp;
  });
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function formatRelTime(unix) {
  var diff = (Date.now() / 1000) - unix;
  if (diff < 60) return "just now";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  if (diff < 604800) return Math.floor(diff / 86400) + "d ago";
  return new Date(unix * 1000).toLocaleDateString();
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
