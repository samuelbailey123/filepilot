// Operation Queue UI — Floating progress panel for file operations.

import * as api from "./api.js";

var _operations = [];
var _panelEl = null;

/**
 * Initialize the operation queue UI.
 * Creates the floating progress panel in the DOM.
 */
export function initOpQueue() {
  _panelEl = document.createElement("div");
  _panelEl.id = "opqueue-panel";
  _panelEl.className = "opqueue-panel hidden";
  _panelEl.setAttribute("role", "log");
  _panelEl.setAttribute("aria-label", "File operations");
  document.body.appendChild(_panelEl);
}

/**
 * Handle an operation progress event from the backend.
 * Updates the local state and re-renders the panel.
 * @param {Object} op - The operation state from the backend.
 */
export function handleOpProgress(op) {
  if (!op || !op.id) return;

  var idx = _operations.findIndex(function (o) { return o.id === op.id; });
  if (idx >= 0) {
    _operations[idx] = op;
  } else {
    _operations.push(op);
  }

  renderPanel();
}

/**
 * Remove completed operations from the list.
 */
export function clearCompleted() {
  _operations = _operations.filter(function (op) {
    return op.status === "pending" || op.status === "running" || op.status === "paused";
  });
  renderPanel();
}

function renderPanel() {
  if (!_panelEl) return;

  // Filter to show only active or recently completed ops.
  var activeOps = _operations.filter(function (op) {
    return op.status !== "completed" && op.status !== "cancelled" && op.status !== "failed";
  });
  var doneOps = _operations.filter(function (op) {
    return op.status === "completed" || op.status === "cancelled" || op.status === "failed";
  });

  if (_operations.length === 0) {
    _panelEl.classList.add("hidden");
    return;
  }

  _panelEl.classList.remove("hidden");

  var html = '<div class="opqueue-header">' +
    '<span class="opqueue-title">File Operations</span>';

  if (doneOps.length > 0) {
    html += '<button class="opqueue-clear-btn" aria-label="Clear completed">Clear</button>';
  }

  html += '</div><div class="opqueue-list">';

  _operations.forEach(function (op) {
    var typeIcon = op.type === "copy" ? "cp" : op.type === "move" ? "mv" : "rm";
    var statusCls = "opqueue-status-" + op.status;
    var pct = Math.round(op.progress || 0);
    var file = op.currentFile || (op.sources && op.sources[0]) || "";
    var fileName = file.split("/").pop() || file;

    // Build per-operation control buttons.
    var controlsHtml = "";
    if (op.status === "running") {
      controlsHtml =
        '<div class="opqueue-controls">' +
          '<button class="opqueue-btn opqueue-pause-btn" data-op-id="' + escapeAttr(op.id) + '" aria-label="Pause operation" title="Pause">\u23F8</button>' +
          '<button class="opqueue-btn opqueue-cancel-btn" data-op-id="' + escapeAttr(op.id) + '" aria-label="Cancel operation" title="Cancel">\u2715</button>' +
        '</div>';
    } else if (op.status === "paused") {
      controlsHtml =
        '<div class="opqueue-controls">' +
          '<button class="opqueue-btn opqueue-resume-btn" data-op-id="' + escapeAttr(op.id) + '" aria-label="Resume operation" title="Resume">\u25B6</button>' +
          '<button class="opqueue-btn opqueue-cancel-btn" data-op-id="' + escapeAttr(op.id) + '" aria-label="Cancel operation" title="Cancel">\u2715</button>' +
        '</div>';
    }

    html += '<div class="opqueue-item ' + statusCls + '">' +
      '<div class="opqueue-item-header">' +
        '<span class="opqueue-type">' + typeIcon + '</span>' +
        '<span class="opqueue-file">' + escapeHtml(fileName) + '</span>' +
        '<span class="opqueue-pct">' + pct + '%</span>' +
        controlsHtml +
      '</div>';

    if (op.status === "running" || op.status === "paused") {
      html += '<div class="opqueue-bar"><div class="opqueue-bar-fill" style="width:' + pct + '%"></div></div>';
    }

    if (op.error) {
      html += '<div class="opqueue-error">' + escapeHtml(op.error) + '</div>';
    }

    html += '</div>';
  });

  html += '</div>';
  _panelEl.innerHTML = html;

  // Bind clear button.
  var clearBtn = _panelEl.querySelector(".opqueue-clear-btn");
  if (clearBtn) {
    clearBtn.addEventListener("click", clearCompleted);
  }

  // Event delegation for per-operation controls.
  _panelEl.querySelector(".opqueue-list").addEventListener("click", function (ev) {
    var pauseBtn = ev.target.closest(".opqueue-pause-btn");
    var resumeBtn = ev.target.closest(".opqueue-resume-btn");
    var cancelBtn = ev.target.closest(".opqueue-cancel-btn");

    if (pauseBtn) {
      ev.stopPropagation();
      api.pauseOp(pauseBtn.dataset.opId).catch(function (err) {
        console.error("PauseOp failed:", err);
      });
    } else if (resumeBtn) {
      ev.stopPropagation();
      api.resumeOp(resumeBtn.dataset.opId).catch(function (err) {
        console.error("ResumeOp failed:", err);
      });
    } else if (cancelBtn) {
      ev.stopPropagation();
      api.cancelOp(cancelBtn.dataset.opId).catch(function (err) {
        console.error("CancelOp failed:", err);
      });
    }
  });
}

function escapeHtml(str) {
  if (!str) return "";
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  if (!str) return "";
  return String(str).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
