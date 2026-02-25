// Sidebar — Favorites + Locations panel.

import * as api from "../services/api.js";
import { navigateTo, getState } from "../app.js";

export async function initSidebar() {
  var sidebar = document.getElementById("sidebar");

  // Default favorites.
  var defaultFavs = [
    { path: "/Users/samuelbailey/Desktop", label: "Desktop", icon: "\uD83D\uDDA5\uFE0F" },
    { path: "/Users/samuelbailey/Downloads", label: "Downloads", icon: "\u2B07\uFE0F" },
    { path: "/Users/samuelbailey/Documents", label: "Documents", icon: "\uD83D\uDCC4" },
    { path: "/Users/samuelbailey/code", label: "Projects", icon: "\uD83D\uDCBB" },
    { path: "/Users/samuelbailey", label: "Home", icon: "\uD83C\uDFE0" },
  ];

  var html = "";

  // Favorites section.
  html +=
    '<div class="sidebar-section">' +
      '<div class="sidebar-section-title">Favorites</div>';

  // Load custom favorites from DB, fall back to defaults.
  var favs;
  try {
    favs = await api.getFavorites();
  } catch (e) {
    favs = null;
  }

  if (favs && favs.length > 0) {
    favs.forEach(function (f) {
      var icon = findIcon(f.path, defaultFavs);
      var label = f.label || basename(f.path);
      html += sidebarItem(f.path, label, icon);
    });
  } else {
    defaultFavs.forEach(function (f) {
      html += sidebarItem(f.path, f.label, f.icon);
    });
  }

  html += "</div>";

  // Locations section.
  html +=
    '<div class="sidebar-section">' +
      '<div class="sidebar-section-title">Locations</div>';

  try {
    var volumes = await api.getVolumes();
    if (volumes) {
      volumes.forEach(function (v) {
        var icon = v.name === "Macintosh HD" ? "\uD83D\uDCBF" : "\uD83D\uDCBE";
        html += sidebarItem(v.path, v.name, icon);
      });
    }
  } catch (e) {
    html += sidebarItem("/", "Macintosh HD", "\uD83D\uDCBF");
  }

  html += "</div>";

  sidebar.innerHTML = html;

  // Click handlers.
  sidebar.querySelectorAll(".sidebar-item").forEach(function (item) {
    item.addEventListener("click", function () {
      var path = item.dataset.path;
      navigateTo(path);

      // Update active state.
      sidebar.querySelectorAll(".sidebar-item").forEach(function (i) {
        i.classList.remove("active");
      });
      item.classList.add("active");
    });

    // Right-click to add/remove from favorites.
    item.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      var path = item.dataset.path;

      // Simple toggle: if in favorites section, remove; otherwise could add.
      var section = item.closest(".sidebar-section");
      var title = section.querySelector(".sidebar-section-title");
      if (title && title.textContent === "Favorites") {
        if (confirm("Remove " + basename(path) + " from favorites?")) {
          api.removeFavorite(path).then(function () {
            initSidebar();
          });
        }
      }
    });
  });
}

function sidebarItem(path, label, icon) {
  return '<div class="sidebar-item" data-path="' + escapeAttr(path) + '">' +
    '<span class="sidebar-icon">' + icon + "</span>" +
    '<span>' + escapeHtml(label) + "</span>" +
  "</div>";
}

function basename(path) {
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

function findIcon(path, defaults) {
  for (var i = 0; i < defaults.length; i++) {
    if (defaults[i].path === path) return defaults[i].icon;
  }
  return "\uD83D\uDCC1";
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
