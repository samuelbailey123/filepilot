// Multi-File Rename Dialog — batch rename with pattern preview.

import * as api from "../services/api.js";
import { getState, navigateTo, announce } from "../app.js";

var _overlayEl = null;
var _files = []; // Array of { path, name } to rename.
var _mode = "findReplace"; // findReplace | numbering | caseChange | extension

/**
 * Initialize the rename dialog overlay.
 */
export function initRenameDialog() {
  _overlayEl = document.getElementById("rename-dialog-overlay");
  if (!_overlayEl) return;

  _overlayEl.addEventListener("click", function (ev) {
    if (ev.target === _overlayEl) {
      closeRenameDialog();
    }
  });

  _overlayEl.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      ev.preventDefault();
      ev.stopPropagation();
      closeRenameDialog();
    }
  });
}

/**
 * Open the rename dialog for the given files.
 * @param {Array} files - Array of { path, name } objects.
 */
export function openRenameDialog(files) {
  if (!_overlayEl || !files || files.length === 0) return;
  _files = files;
  _mode = "findReplace";
  renderDialog();
  _overlayEl.classList.add("visible");
  _overlayEl.setAttribute("aria-hidden", "false");

  // Focus the first input.
  var firstInput = _overlayEl.querySelector("input");
  if (firstInput) firstInput.focus();
}

/**
 * Close the rename dialog.
 */
export function closeRenameDialog() {
  if (!_overlayEl) return;
  _overlayEl.classList.remove("visible");
  _overlayEl.setAttribute("aria-hidden", "true");
  _files = [];
}

function renderDialog() {
  var html =
    '<div class="rename-card" role="dialog" aria-labelledby="rename-title">' +
      '<div class="rename-header">' +
        '<div class="rename-title" id="rename-title">Rename ' + _files.length + ' File' + (_files.length !== 1 ? 's' : '') + '</div>' +
        '<button class="settings-close-btn rename-close-btn" aria-label="Close">&times;</button>' +
      '</div>' +
      '<div class="rename-body">' +
        renderModeTabs() +
        renderModeForm() +
        renderPreview() +
      '</div>' +
      '<div class="rename-footer">' +
        '<button class="icon-btn rename-cancel-btn">Cancel</button>' +
        '<button class="icon-btn modal-confirm rename-apply-btn">Rename</button>' +
      '</div>' +
    '</div>';

  _overlayEl.innerHTML = html;
  bindEvents();
}

function renderModeTabs() {
  var modes = [
    { id: "findReplace", label: "Find & Replace" },
    { id: "numbering", label: "Numbering" },
    { id: "caseChange", label: "Case" },
    { id: "extension", label: "Extension" },
  ];

  var html = '<div class="rename-tabs">';
  modes.forEach(function (m) {
    var cls = m.id === _mode ? "rename-tab active" : "rename-tab";
    html += '<button class="' + cls + '" data-mode="' + m.id + '">' + m.label + '</button>';
  });
  html += '</div>';
  return html;
}

function renderModeForm() {
  switch (_mode) {
    case "findReplace":
      return '<div class="rename-form">' +
        '<div class="rename-field">' +
          '<label class="rename-field-label">Find</label>' +
          '<input class="rename-input-field" id="rename-find" type="text" placeholder="Text to find..." autocomplete="off" spellcheck="false">' +
        '</div>' +
        '<div class="rename-field">' +
          '<label class="rename-field-label">Replace with</label>' +
          '<input class="rename-input-field" id="rename-replace" type="text" placeholder="Replacement text..." autocomplete="off" spellcheck="false">' +
        '</div>' +
        '<label class="rename-checkbox-label">' +
          '<input type="checkbox" id="rename-regex"> Use regex' +
        '</label>' +
      '</div>';

    case "numbering":
      return '<div class="rename-form">' +
        '<div class="rename-field">' +
          '<label class="rename-field-label">Pattern</label>' +
          '<input class="rename-input-field" id="rename-pattern" type="text" value="{name}_{num}" placeholder="{name}_{num}" autocomplete="off" spellcheck="false">' +
        '</div>' +
        '<div class="rename-row-fields">' +
          '<div class="rename-field">' +
            '<label class="rename-field-label">Start</label>' +
            '<input class="rename-input-field" id="rename-start" type="number" value="1" min="0">' +
          '</div>' +
          '<div class="rename-field">' +
            '<label class="rename-field-label">Step</label>' +
            '<input class="rename-input-field" id="rename-step" type="number" value="1" min="1">' +
          '</div>' +
          '<div class="rename-field">' +
            '<label class="rename-field-label">Digits</label>' +
            '<input class="rename-input-field" id="rename-digits" type="number" value="2" min="1" max="8">' +
          '</div>' +
        '</div>' +
        '<div class="rename-hint">Use {name} for original name, {ext} for extension, {num} for number, {date} for today.</div>' +
      '</div>';

    case "caseChange":
      return '<div class="rename-form">' +
        '<div class="rename-case-options">' +
          '<label class="rename-radio-label"><input type="radio" name="rename-case" value="lower" checked> lowercase</label>' +
          '<label class="rename-radio-label"><input type="radio" name="rename-case" value="upper"> UPPERCASE</label>' +
          '<label class="rename-radio-label"><input type="radio" name="rename-case" value="title"> Title Case</label>' +
          '<label class="rename-radio-label"><input type="radio" name="rename-case" value="sentence"> Sentence case</label>' +
        '</div>' +
      '</div>';

    case "extension":
      return '<div class="rename-form">' +
        '<div class="rename-field">' +
          '<label class="rename-field-label">New extension</label>' +
          '<input class="rename-input-field" id="rename-ext" type="text" placeholder="e.g., txt, md, jpg" autocomplete="off" spellcheck="false">' +
        '</div>' +
      '</div>';
  }

  return '';
}

function renderPreview() {
  var results = computeRenames();
  var html = '<div class="rename-preview">' +
    '<div class="rename-preview-title">Preview</div>' +
    '<div class="rename-preview-list">';

  results.forEach(function (r) {
    var changed = r.oldName !== r.newName;
    var cls = changed ? "rename-preview-item changed" : "rename-preview-item";
    html += '<div class="' + cls + '">' +
      '<span class="rename-old">' + escapeHtml(r.oldName) + '</span>' +
      (changed ? '<span class="rename-arrow">\u2192</span><span class="rename-new">' + escapeHtml(r.newName) + '</span>' : '') +
    '</div>';
  });

  html += '</div></div>';
  return html;
}

function computeRenames() {
  var results = [];

  _files.forEach(function (f, index) {
    var parts = splitNameExt(f.name);
    var baseName = parts.name;
    var ext = parts.ext;
    var newName = f.name;

    switch (_mode) {
      case "findReplace": {
        var findVal = getInputValue("rename-find");
        var replaceVal = getInputValue("rename-replace");
        var useRegex = getChecked("rename-regex");

        if (findVal) {
          if (useRegex) {
            try {
              var re = new RegExp(findVal, "g");
              newName = f.name.replace(re, replaceVal);
            } catch (e) {
              // Invalid regex; leave unchanged.
            }
          } else {
            newName = f.name.split(findVal).join(replaceVal);
          }
        }
        break;
      }

      case "numbering": {
        var pattern = getInputValue("rename-pattern") || "{name}_{num}";
        var start = parseInt(getInputValue("rename-start") || "1", 10);
        var step = parseInt(getInputValue("rename-step") || "1", 10);
        var digits = parseInt(getInputValue("rename-digits") || "2", 10);

        var num = start + (index * step);
        var padded = String(num).padStart(digits, "0");
        var today = new Date().toISOString().slice(0, 10);

        newName = pattern
          .replace(/\{name\}/g, baseName)
          .replace(/\{ext\}/g, ext)
          .replace(/\{num\}/g, padded)
          .replace(/\{date\}/g, today);

        // Re-add extension if the pattern doesn't include {ext}.
        if (ext && pattern.indexOf("{ext}") === -1) {
          newName += "." + ext;
        }
        break;
      }

      case "caseChange": {
        var caseType = getRadioValue("rename-case") || "lower";
        switch (caseType) {
          case "lower":
            newName = f.name.toLowerCase();
            break;
          case "upper":
            newName = f.name.toUpperCase();
            break;
          case "title":
            newName = f.name.replace(/\b\w/g, function (c) { return c.toUpperCase(); });
            break;
          case "sentence":
            newName = f.name.charAt(0).toUpperCase() + f.name.slice(1).toLowerCase();
            break;
        }
        break;
      }

      case "extension": {
        var newExt = getInputValue("rename-ext");
        if (newExt !== undefined && newExt !== null) {
          newExt = newExt.replace(/^\./, ""); // Strip leading dot.
          if (newExt) {
            newName = baseName + "." + newExt;
          } else {
            newName = baseName;
          }
        }
        break;
      }
    }

    results.push({ path: f.path, oldName: f.name, newName: newName });
  });

  return results;
}

function bindEvents() {
  // Mode tabs.
  var tabs = _overlayEl.querySelectorAll(".rename-tab");
  tabs.forEach(function (tab) {
    tab.addEventListener("click", function () {
      _mode = tab.dataset.mode;
      renderDialog();
    });
  });

  // Close button.
  var closeBtn = _overlayEl.querySelector(".rename-close-btn");
  if (closeBtn) closeBtn.addEventListener("click", closeRenameDialog);

  // Cancel button.
  var cancelBtn = _overlayEl.querySelector(".rename-cancel-btn");
  if (cancelBtn) cancelBtn.addEventListener("click", closeRenameDialog);

  // Apply button.
  var applyBtn = _overlayEl.querySelector(".rename-apply-btn");
  if (applyBtn) applyBtn.addEventListener("click", applyRenames);

  // Live preview on input changes.
  var inputs = _overlayEl.querySelectorAll("input");
  inputs.forEach(function (input) {
    input.addEventListener("input", updatePreview);
    input.addEventListener("change", updatePreview);
  });
}

function updatePreview() {
  var previewEl = _overlayEl.querySelector(".rename-preview");
  if (!previewEl) return;

  var results = computeRenames();
  var html = '<div class="rename-preview-title">Preview</div>' +
    '<div class="rename-preview-list">';

  results.forEach(function (r) {
    var changed = r.oldName !== r.newName;
    var cls = changed ? "rename-preview-item changed" : "rename-preview-item";
    html += '<div class="' + cls + '">' +
      '<span class="rename-old">' + escapeHtml(r.oldName) + '</span>' +
      (changed ? '<span class="rename-arrow">\u2192</span><span class="rename-new">' + escapeHtml(r.newName) + '</span>' : '') +
    '</div>';
  });

  html += '</div>';
  previewEl.innerHTML = html;
}

async function applyRenames() {
  var results = computeRenames();
  var changed = results.filter(function (r) { return r.oldName !== r.newName; });
  if (changed.length === 0) {
    closeRenameDialog();
    return;
  }

  var ops = changed.map(function (r) {
    return { path: r.path, newName: r.newName };
  });

  try {
    await api.batchRename(ops);
    announce("Renamed " + changed.length + " file" + (changed.length !== 1 ? "s" : ""));
    closeRenameDialog();
    // Refresh current directory.
    var state = getState();
    navigateTo(state.currentPath);
  } catch (err) {
    // Show error in the dialog.
    var footer = _overlayEl.querySelector(".rename-footer");
    if (footer) {
      var existing = footer.querySelector(".rename-error");
      if (existing) existing.remove();
      var errEl = document.createElement("div");
      errEl.className = "rename-error";
      errEl.textContent = err.message || String(err);
      footer.prepend(errEl);
    }
  }
}

// Helpers.

function splitNameExt(filename) {
  var dotIdx = filename.lastIndexOf(".");
  if (dotIdx <= 0) return { name: filename, ext: "" };
  return { name: filename.substring(0, dotIdx), ext: filename.substring(dotIdx + 1) };
}

function getInputValue(id) {
  var el = _overlayEl ? _overlayEl.querySelector("#" + id) : null;
  return el ? el.value : "";
}

function getChecked(id) {
  var el = _overlayEl ? _overlayEl.querySelector("#" + id) : null;
  return el ? el.checked : false;
}

function getRadioValue(name) {
  if (!_overlayEl) return "";
  var checked = _overlayEl.querySelector('input[name="' + name + '"]:checked');
  return checked ? checked.value : "";
}

function escapeHtml(str) {
  if (!str) return "";
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
