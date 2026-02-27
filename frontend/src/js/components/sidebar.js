// Sidebar — Favorites + Locations + Network panel with disk usage.

import * as api from "../services/api.js";
import { navigateTo, getState, announce } from "../app.js";
import { makeDropTarget } from "../services/dragdrop.js";
import { modalConfirm } from "./modal.js";
import { openConnectionDialog, quickConnect, quickDisconnect } from "./connection-dialog.js";

/**
 * Initialize the sidebar with favorites and mounted volumes.
 */
export async function initSidebar() {
  var sidebar = document.getElementById("sidebar");
  var homeDir;

  try {
    homeDir = await api.getHomeDir();
  } catch (e) {
    homeDir = "/Users";
  }

  // Default favorites based on actual home directory.
  var defaultFavs = [
    { path: homeDir + "/Desktop", label: "Desktop", icon: "\uD83D\uDDA5\uFE0F" },
    { path: homeDir + "/Downloads", label: "Downloads", icon: "\u2B07\uFE0F" },
    { path: homeDir + "/Documents", label: "Documents", icon: "\uD83D\uDCC4" },
    { path: homeDir + "/code", label: "Projects", icon: "\uD83D\uDCBB" },
    { path: homeDir, label: "Home", icon: "\uD83C\uDFE0" },
  ];

  var html = "";

  // Favorites section.
  html +=
    '<div class="sidebar-section">' +
      '<div class="sidebar-section-title" id="sidebar-favs-title">Favorites</div>' +
      '<div role="list" aria-labelledby="sidebar-favs-title">';

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

  html += "</div></div>";

  // Locations section.
  html +=
    '<div class="sidebar-section">' +
      '<div class="sidebar-section-title" id="sidebar-locs-title">Locations</div>' +
      '<div role="list" aria-labelledby="sidebar-locs-title">';

  var volumes = [];
  try {
    volumes = await api.getVolumes();
    if (volumes) {
      volumes.forEach(function (v) {
        var icon = v.name === "Macintosh HD" ? "\uD83D\uDCBF" : "\uD83D\uDCBE";
        html += sidebarItem(v.path, v.name, icon, true);
      });
    }
  } catch (e) {
    html += sidebarItem("/", "Macintosh HD", "\uD83D\uDCBF", true);
    volumes = [{ path: "/", name: "Macintosh HD" }];
  }

  html += "</div></div>";

  // Network section.
  html +=
    '<div class="sidebar-section">' +
      '<div class="sidebar-section-title sidebar-network-title" id="sidebar-net-title">' +
        '<span>Network</span>' +
        '<button class="sidebar-add-conn-btn" aria-label="Add connection" title="New connection">+</button>' +
      '</div>' +
      '<div id="sidebar-network-list" role="list" aria-labelledby="sidebar-net-title"></div>' +
    '</div>';

  sidebar.innerHTML = html;

  // Load network connections.
  loadNetworkConnections(sidebar);

  // Click handlers.
  sidebar.querySelectorAll(".sidebar-item").forEach(function (item) {
    item.addEventListener("click", function () {
      var path = item.dataset.path;
      navigateTo(path);

      // Update active state.
      sidebar.querySelectorAll(".sidebar-item").forEach(function (i) {
        i.classList.remove("active");
        i.setAttribute("aria-current", "false");
      });
      item.classList.add("active");
      item.setAttribute("aria-current", "page");
    });

    // Make sidebar items drop targets (move files into this directory).
    makeDropTarget(item, item.dataset.path);

    // Right-click to remove from favorites.
    item.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      var path = item.dataset.path;

      var section = item.closest(".sidebar-section");
      var title = section.querySelector(".sidebar-section-title");
      if (title && title.textContent === "Favorites") {
        modalConfirm(
          "Remove " + basename(path) + " from favorites?",
          { title: "Remove Favorite", confirmLabel: "Remove", danger: true }
        ).then(function (confirmed) {
          if (confirmed) {
            api.removeFavorite(path).then(function () {
              initSidebar();
              announce("Removed from favorites");
            });
          }
        });
      }
    });
  });

  // Eject button handlers.
  sidebar.querySelectorAll(".sidebar-eject-btn").forEach(function (btn) {
    btn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      var volumePath = btn.dataset.eject;
      var volumeName = basename(volumePath);
      modalConfirm(
        "Eject " + volumeName + "?",
        { title: "Eject Volume", confirmLabel: "Eject" }
      ).then(function (confirmed) {
        if (confirmed) {
          api.ejectVolume(volumePath).then(function () {
            announce(volumeName + " ejected");
            initSidebar();
          }).catch(function (err) {
            announce("Eject failed: " + err);
          });
        }
      });
    });
  });

  // Load disk usage for each volume asynchronously.
  if (volumes) {
    volumes.forEach(function (v) {
      loadDiskUsage(v.path, sidebar);
    });
  }

  // Add connection button.
  var addConnBtn = sidebar.querySelector(".sidebar-add-conn-btn");
  if (addConnBtn) {
    addConnBtn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      openConnectionDialog(null);
    });
  }

  // Refresh network list when connections change.
  document.addEventListener("connections-changed", function () {
    loadNetworkConnections(sidebar);
  });
}

/**
 * Load and render saved network connections in the sidebar.
 * @param {HTMLElement} sidebar - The sidebar element.
 */
async function loadNetworkConnections(sidebar) {
  var listEl = sidebar.querySelector("#sidebar-network-list");
  if (!listEl) return;

  var connections;
  try {
    connections = await api.listSavedConnections();
  } catch (e) {
    connections = [];
  }

  if (!connections || connections.length === 0) {
    listEl.innerHTML = '<div class="sidebar-empty">No connections</div>';
    return;
  }

  var html = "";
  connections.forEach(function (conn) {
    var icon = conn.protocol === "sftp" ? "\uD83D\uDD12" : conn.protocol === "ftp" ? "\uD83C\uDF10" : "\u2601\uFE0F";
    var statusCls = conn.active ? " conn-active" : "";
    var statusDot = conn.active ? '<span class="conn-status-dot active" title="Connected"></span>' : '<span class="conn-status-dot" title="Disconnected"></span>';

    html +=
      '<div class="sidebar-item sidebar-conn-item' + statusCls + '" data-conn-id="' + escapeAttr(conn.id) + '"' +
        ' role="listitem" tabindex="0">' +
        '<div class="sidebar-item-row">' +
          '<span class="sidebar-icon" aria-hidden="true">' + icon + '</span>' +
          '<span>' + escapeHtml(conn.name) + '</span>' +
          statusDot +
        '</div>' +
        '<div class="conn-detail">' + conn.user + '@' + conn.host + '</div>' +
      '</div>';
  });
  listEl.innerHTML = html;

  // Click handlers for network connections.
  listEl.querySelectorAll(".sidebar-conn-item").forEach(function (item) {
    var connID = item.dataset.connId;
    var conn = connections.find(function (c) { return c.id === connID; });
    if (!conn) return;

    item.addEventListener("click", function () {
      if (conn.active) {
        // Navigate to the connection's remote path.
        navigateTo("remote://" + connID + (conn.basePath || "/"));
      } else {
        quickConnect(conn);
      }
    });

    item.addEventListener("contextmenu", function (ev) {
      ev.preventDefault();
      if (conn.active) {
        modalConfirm(
          "Disconnect from " + conn.name + "?",
          { title: "Disconnect", confirmLabel: "Disconnect" }
        ).then(function (confirmed) {
          if (confirmed) quickDisconnect(connID);
        });
      } else {
        openConnectionDialog(conn);
      }
    });
  });
}

/**
 * Load and display disk usage for a volume path.
 */
async function loadDiskUsage(volumePath, sidebar) {
  try {
    var usage = await api.getDiskUsage(volumePath);
    if (!usage) return;

    var item = sidebar.querySelector('.sidebar-item[data-path="' + escapeAttr(volumePath) + '"]');
    if (!item) return;

    var bar = item.querySelector(".disk-usage-bar");
    if (!bar) return;

    var fill = bar.querySelector(".disk-usage-fill");
    var label = item.querySelector(".disk-usage-label");

    fill.style.width = usage.usedPct + "%";

    // Color the bar based on usage.
    if (usage.usedPct > 90) {
      fill.style.background = "var(--danger)";
    } else if (usage.usedPct > 75) {
      fill.style.background = "var(--warning)";
    }

    label.textContent = formatBytes(usage.free) + " free of " + formatBytes(usage.total);
  } catch (e) {
    // Disk usage unavailable — leave placeholder hidden.
  }
}

function sidebarItem(path, label, icon, showDiskUsage) {
  var diskHtml = "";
  if (showDiskUsage) {
    diskHtml =
      '<div class="disk-usage">' +
        '<div class="disk-usage-bar"><div class="disk-usage-fill"></div></div>' +
        '<div class="disk-usage-label"></div>' +
      '</div>';
  }

  // Show eject button for non-root external volumes.
  var ejectHtml = "";
  if (showDiskUsage && path !== "/" && path !== "/System/Volumes/Data") {
    ejectHtml = '<button class="sidebar-eject-btn" data-eject="' + escapeAttr(path) + '"' +
      ' aria-label="Eject ' + escapeAttr(label) + '" title="Eject">\u23CF</button>';
  }

  return '<div class="sidebar-item" data-path="' + escapeAttr(path) + '"' +
    ' role="listitem" tabindex="0" aria-current="false">' +
    '<div class="sidebar-item-row">' +
      '<span class="sidebar-icon" aria-hidden="true">' + icon + "</span>" +
      '<span>' + escapeHtml(label) + "</span>" +
      ejectHtml +
    '</div>' +
    diskHtml +
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

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB", "TB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
