// Settings — Keymap editor and preferences panel.

import { getActions, getPresets, applyPreset, rebind, resetKeymap, formatKeyCombo, exportKeymap } from "./shortcuts.js";
import * as api from "./api.js";

var _overlayEl = null;
var _recording = null; // { actionName, rowEl } when recording a new binding.

/**
 * Initialize the settings overlay in the DOM.
 */
export function initSettings() {
  _overlayEl = document.getElementById("settings-overlay");
  if (!_overlayEl) return;

  // Close on background click.
  _overlayEl.addEventListener("click", function (ev) {
    if (ev.target === _overlayEl) {
      closeSettings();
    }
  });

  // Close on Escape.
  _overlayEl.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      ev.preventDefault();
      ev.stopPropagation();
      if (_recording) {
        cancelRecording();
      } else {
        closeSettings();
      }
    }
  });
}

/**
 * Open the settings overlay.
 */
export function openSettings() {
  if (!_overlayEl) return;
  renderSettings();
  _overlayEl.classList.add("visible");
  _overlayEl.setAttribute("aria-hidden", "false");
}

/**
 * Close the settings overlay.
 */
export function closeSettings() {
  if (!_overlayEl) return;
  _recording = null;
  _overlayEl.classList.remove("visible");
  _overlayEl.setAttribute("aria-hidden", "true");
  // Persist to Go backend if available.
  persistKeymap();
}

/**
 * Check whether the settings overlay is visible.
 * @returns {boolean}
 */
export function isSettingsVisible() {
  return _overlayEl && _overlayEl.classList.contains("visible");
}

function renderSettings() {
  var actions = getActions();
  var presets = getPresets();

  var html =
    '<div class="settings-card" role="dialog" aria-labelledby="settings-title">' +
      '<div class="settings-header">' +
        '<div class="settings-title" id="settings-title">Settings</div>' +
        '<button class="settings-close-btn" aria-label="Close settings">&times;</button>' +
      '</div>' +
      '<div class="settings-body">' +
        renderKeymapSection(actions, presets) +
      '</div>' +
    '</div>';

  _overlayEl.innerHTML = html;

  // Bind close button.
  var closeBtn = _overlayEl.querySelector(".settings-close-btn");
  if (closeBtn) {
    closeBtn.addEventListener("click", closeSettings);
  }

  // Bind preset buttons.
  var presetBtns = _overlayEl.querySelectorAll(".settings-preset-btn");
  presetBtns.forEach(function (btn) {
    btn.addEventListener("click", function () {
      applyPreset(btn.dataset.preset);
      renderSettings();
    });
  });

  // Bind reset button.
  var resetBtn = _overlayEl.querySelector(".settings-reset-btn");
  if (resetBtn) {
    resetBtn.addEventListener("click", function () {
      resetKeymap();
      renderSettings();
    });
  }

  // Bind rebind buttons.
  var rebindBtns = _overlayEl.querySelectorAll(".keymap-rebind-btn");
  rebindBtns.forEach(function (btn) {
    btn.addEventListener("click", function () {
      startRecording(btn.dataset.action, btn.closest(".keymap-row"));
    });
  });
}

function renderKeymapSection(actions, presets) {
  var html =
    '<div class="settings-section">' +
      '<div class="settings-section-title">Keyboard Shortcuts</div>' +
      '<div class="settings-presets">';

  presets.forEach(function (name) {
    var label = name.charAt(0).toUpperCase() + name.slice(1);
    html += '<button class="settings-preset-btn" data-preset="' + name + '">' + label + '</button>';
  });

  html += '<button class="settings-reset-btn">Reset All</button>';
  html += '</div>';

  html += '<div class="keymap-list">';
  actions.forEach(function (action) {
    var displayKey = formatKeyCombo(action.key);
    var defaultCls = action.isDefault ? "" : " keymap-custom";
    html +=
      '<div class="keymap-row' + defaultCls + '" data-action="' + action.name + '">' +
        '<span class="keymap-label">' + escapeHtml(action.label) + '</span>' +
        '<button class="keymap-rebind-btn" data-action="' + action.name + '" ' +
          'aria-label="Rebind ' + escapeHtml(action.label) + '">' +
          '<span class="keymap-key">' + escapeHtml(displayKey) + '</span>' +
        '</button>' +
      '</div>';
  });
  html += '</div></div>';

  return html;
}

function startRecording(actionName, rowEl) {
  // Cancel any previous recording.
  cancelRecording();

  _recording = { actionName: actionName, rowEl: rowEl };

  var btn = rowEl.querySelector(".keymap-rebind-btn");
  var keySpan = btn.querySelector(".keymap-key");
  keySpan.textContent = "Press key...";
  keySpan.classList.add("recording");
  rowEl.classList.add("keymap-recording");

  // Listen for the next keypress.
  document.addEventListener("keydown", handleRecordKey, true);
}

function cancelRecording() {
  if (!_recording) return;
  _recording.rowEl.classList.remove("keymap-recording");
  var keySpan = _recording.rowEl.querySelector(".keymap-key");
  if (keySpan) keySpan.classList.remove("recording");
  document.removeEventListener("keydown", handleRecordKey, true);
  _recording = null;
  renderSettings();
}

function handleRecordKey(ev) {
  ev.preventDefault();
  ev.stopPropagation();

  // Ignore modifier-only presses.
  if (["Meta", "Control", "Alt", "Shift"].indexOf(ev.key) >= 0) return;

  // Escape cancels recording.
  if (ev.key === "Escape") {
    cancelRecording();
    return;
  }

  var mod = (ev.metaKey || ev.ctrlKey) ? "cmd+" : "";
  var shift = ev.shiftKey ? "shift+" : "";
  var combo = mod + shift + ev.key;

  var actionName = _recording.actionName;
  document.removeEventListener("keydown", handleRecordKey, true);
  _recording = null;

  var displaced = rebind(actionName, combo);
  renderSettings();

  // If a conflict was detected, briefly highlight the displaced row.
  if (displaced) {
    var displacedRow = _overlayEl.querySelector('.keymap-row[data-action="' + displaced + '"]');
    if (displacedRow) {
      displacedRow.classList.add("keymap-conflict");
      setTimeout(function () {
        displacedRow.classList.remove("keymap-conflict");
      }, 1500);
    }
  }
}

/**
 * Persist keymap to Go backend (fire-and-forget).
 */
function persistKeymap() {
  try {
    var json = exportKeymap();
    api.saveKeymap(json);
  } catch (e) {
    // Backend unavailable; localStorage is the primary store.
  }
}

/**
 * Load keymap from Go backend on startup.
 */
export async function loadKeymapFromBackend() {
  try {
    var json = await api.loadKeymap();
    if (json) {
      // Merge into localStorage (backend is authoritative).
      var overrides = JSON.parse(json);
      if (overrides && typeof overrides === "object") {
        localStorage.setItem("fp-keymap", json);
      }
    }
  } catch (e) {
    // Backend unavailable; use localStorage only.
  }
}

function escapeHtml(str) {
  if (!str) return "";
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
