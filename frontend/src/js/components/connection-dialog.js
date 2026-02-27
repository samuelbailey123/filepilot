// Connection Manager Dialog — Add/edit/connect network storage connections.

import * as api from "../services/api.js";
import { navigateTo, announce } from "../app.js";

var _dialogEl = null;

/**
 * Initialize the connection dialog.
 */
export function initConnectionDialog() {
  _dialogEl = document.getElementById("connection-dialog-overlay");
}

/**
 * Open the connection dialog for creating or editing a connection.
 * @param {Object} [existing] - Existing connection to edit (null for new).
 */
export function openConnectionDialog(existing) {
  if (!_dialogEl) return;

  var isEdit = !!existing;
  var cfg = existing || { id: "", name: "", protocol: "sftp", host: "", port: 0, user: "", basePath: "" };

  _dialogEl.innerHTML =
    '<div class="conn-card">' +
      '<div class="conn-header">' +
        '<span class="conn-title">' + (isEdit ? "Edit Connection" : "New Connection") + '</span>' +
        '<button class="conn-close" aria-label="Close">&times;</button>' +
      '</div>' +
      '<div class="conn-body">' +
        '<div class="conn-field">' +
          '<label>Name</label>' +
          '<input type="text" id="conn-name" value="' + escapeAttr(cfg.name) + '" placeholder="My Server" />' +
        '</div>' +
        '<div class="conn-field">' +
          '<label>Protocol</label>' +
          '<select id="conn-protocol">' +
            '<option value="sftp"' + (cfg.protocol === "sftp" ? " selected" : "") + '>SFTP</option>' +
            '<option value="ftp"' + (cfg.protocol === "ftp" ? " selected" : "") + '>FTP</option>' +
            '<option value="webdav"' + (cfg.protocol === "webdav" ? " selected" : "") + '>WebDAV</option>' +
            '<option value="s3"' + (cfg.protocol === "s3" ? " selected" : "") + '>Amazon S3</option>' +
          '</select>' +
        '</div>' +
        '<div class="conn-field">' +
          '<label>Host</label>' +
          '<input type="text" id="conn-host" value="' + escapeAttr(cfg.host) + '" placeholder="example.com" />' +
        '</div>' +
        '<div class="conn-row">' +
          '<div class="conn-field conn-field-port">' +
            '<label>Port</label>' +
            '<input type="number" id="conn-port" value="' + (cfg.port || "") + '" placeholder="22" />' +
          '</div>' +
          '<div class="conn-field conn-field-user">' +
            '<label>User</label>' +
            '<input type="text" id="conn-user" value="' + escapeAttr(cfg.user) + '" placeholder="username" />' +
          '</div>' +
        '</div>' +
        '<div class="conn-field">' +
          '<label>Password</label>' +
          '<input type="password" id="conn-password" placeholder="Enter password" />' +
        '</div>' +
        '<div class="conn-field">' +
          '<label>Remote Path</label>' +
          '<input type="text" id="conn-basepath" value="' + escapeAttr(cfg.basePath || "/") + '" placeholder="/" />' +
        '</div>' +
        '<div class="conn-error" id="conn-error"></div>' +
      '</div>' +
      '<div class="conn-footer">' +
        (isEdit ? '<button class="conn-btn conn-btn-delete" id="conn-delete-btn">Delete</button>' : '') +
        '<button class="conn-btn conn-btn-save" id="conn-save-btn">Save</button>' +
        '<button class="conn-btn conn-btn-connect" id="conn-connect-btn">Save &amp; Connect</button>' +
      '</div>' +
    '</div>';

  _dialogEl.classList.add("visible");
  _dialogEl.setAttribute("aria-hidden", "false");

  // Focus name field.
  var nameInput = document.getElementById("conn-name");
  if (nameInput) nameInput.focus();

  // Close button.
  _dialogEl.querySelector(".conn-close").addEventListener("click", closeConnectionDialog);
  _dialogEl.addEventListener("click", function (ev) {
    if (ev.target === _dialogEl) closeConnectionDialog();
  });

  // Save button.
  document.getElementById("conn-save-btn").addEventListener("click", function () {
    saveFromDialog(cfg.id, false);
  });

  // Save & Connect button.
  document.getElementById("conn-connect-btn").addEventListener("click", function () {
    saveFromDialog(cfg.id, true);
  });

  // Delete button (edit mode only).
  var deleteBtn = document.getElementById("conn-delete-btn");
  if (deleteBtn) {
    deleteBtn.addEventListener("click", function () {
      deleteConnection(cfg.id);
    });
  }

  // Update field labels and defaults when protocol changes.
  var updateLabels = function () {
    var proto = document.getElementById("conn-protocol").value;
    var hostLabel = _dialogEl.querySelector("#conn-host").previousElementSibling;
    var userLabel = _dialogEl.querySelector("#conn-user").previousElementSibling;
    var pathLabel = _dialogEl.querySelector("#conn-basepath").previousElementSibling;
    var portInput = document.getElementById("conn-port");
    var hostInput = document.getElementById("conn-host");
    var userInput = document.getElementById("conn-user");
    var pathInput = document.getElementById("conn-basepath");

    if (proto === "s3") {
      if (hostLabel) hostLabel.textContent = "Region";
      if (userLabel) userLabel.textContent = "Access Key ID";
      if (pathLabel) pathLabel.textContent = "Bucket";
      hostInput.placeholder = "us-east-1";
      userInput.placeholder = "AKIAIOSFODNN7EXAMPLE";
      pathInput.placeholder = "my-bucket";
      portInput.parentElement.style.display = "none";
    } else {
      if (hostLabel) hostLabel.textContent = "Host";
      if (userLabel) userLabel.textContent = "User";
      if (pathLabel) pathLabel.textContent = "Remote Path";
      hostInput.placeholder = "example.com";
      userInput.placeholder = "username";
      pathInput.placeholder = "/";
      portInput.parentElement.style.display = "";
      if (!portInput.value) {
        var defaults = { sftp: 22, ftp: 21, webdav: 443 };
        portInput.placeholder = String(defaults[proto] || "");
      }
    }
  };
  document.getElementById("conn-protocol").addEventListener("change", updateLabels);
  updateLabels();

  // Escape to close.
  _dialogEl.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") closeConnectionDialog();
  });
}

/**
 * Close the connection dialog.
 */
export function closeConnectionDialog() {
  if (!_dialogEl) return;
  _dialogEl.classList.remove("visible");
  _dialogEl.setAttribute("aria-hidden", "true");
}

/**
 * Read dialog fields, save, and optionally connect.
 * @param {string} existingID - Existing connection ID (empty for new).
 * @param {boolean} connect - Whether to connect after saving.
 */
async function saveFromDialog(existingID, connect) {
  var name = document.getElementById("conn-name").value.trim();
  var protocol = document.getElementById("conn-protocol").value;
  var host = document.getElementById("conn-host").value.trim();
  var port = parseInt(document.getElementById("conn-port").value, 10) || 0;
  var user = document.getElementById("conn-user").value.trim();
  var password = document.getElementById("conn-password").value;
  var basePath = document.getElementById("conn-basepath").value.trim() || "/";

  var errorEl = document.getElementById("conn-error");

  if (!name) {
    errorEl.textContent = "Name is required.";
    return;
  }
  if (!host) {
    errorEl.textContent = "Host is required.";
    return;
  }

  var id = existingID || slugify(name);

  var cfg = {
    id: id,
    name: name,
    protocol: protocol,
    host: host,
    port: port,
    user: user,
    basePath: basePath,
  };

  try {
    await api.saveConnection(JSON.stringify(cfg));
    announce("Connection saved: " + name);

    if (connect) {
      errorEl.textContent = "Connecting...";
      var remotePath = await connectByProtocol(id, protocol, password);
      closeConnectionDialog();
      navigateTo(remotePath);
      announce("Connected to " + name);
    } else {
      closeConnectionDialog();
    }

    // Refresh sidebar connections.
    document.dispatchEvent(new CustomEvent("connections-changed"));
  } catch (err) {
    errorEl.textContent = "Error: " + (err.message || err);
  }
}

/**
 * Connect using the appropriate protocol.
 * @param {string} connID - Connection ID.
 * @param {string} protocol - Protocol name.
 * @param {string} password - Password.
 * @returns {Promise<string>} Remote path to navigate to.
 */
async function connectByProtocol(connID, protocol, password) {
  switch (protocol) {
    case "sftp":
      return api.connectSFTP(connID, password);
    case "ftp":
      return api.connectFTP(connID, password);
    case "webdav":
      return api.connectWebDAV(connID, password);
    case "s3":
      return api.connectS3(connID, password);
    default:
      throw new Error("Unsupported protocol: " + protocol);
  }
}

/**
 * Delete a connection.
 * @param {string} connID - Connection ID.
 */
async function deleteConnection(connID) {
  try {
    await api.deleteConnection(connID);
    closeConnectionDialog();
    announce("Connection deleted");
    document.dispatchEvent(new CustomEvent("connections-changed"));
  } catch (err) {
    var errorEl = document.getElementById("conn-error");
    if (errorEl) errorEl.textContent = "Delete failed: " + (err.message || err);
  }
}

/**
 * Quick connect to an existing saved connection.
 * @param {Object} conn - Connection info object with id, protocol, name.
 */
export async function quickConnect(conn) {
  var password = prompt("Password for " + conn.name + ":");
  if (password === null) return; // User cancelled.

  try {
    var remotePath = await connectByProtocol(conn.id, conn.protocol, password);
    navigateTo(remotePath);
    announce("Connected to " + conn.name);
    document.dispatchEvent(new CustomEvent("connections-changed"));
  } catch (err) {
    announce("Connection failed: " + (err.message || err));
  }
}

/**
 * Disconnect from a remote connection.
 * @param {string} connID - Connection ID.
 */
export async function quickDisconnect(connID) {
  try {
    await api.disconnectRemote(connID);
    announce("Disconnected");
    document.dispatchEvent(new CustomEvent("connections-changed"));
  } catch (err) {
    announce("Disconnect failed: " + (err.message || err));
  }
}

function slugify(str) {
  return str.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function escapeAttr(str) {
  if (!str) return "";
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;").replace(/</g, "&lt;");
}
