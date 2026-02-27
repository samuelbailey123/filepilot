// Modal — Accessible custom dialogs replacing confirm()/prompt()/alert().

var _resolveModal = null;

export function initModal() {
  var overlay = document.getElementById("modal-overlay");

  // Close on background click.
  overlay.addEventListener("click", function (ev) {
    if (ev.target === overlay) {
      closeModal(null);
    }
  });

  // Close on Escape.
  overlay.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      ev.preventDefault();
      ev.stopPropagation();
      closeModal(null);
    }
  });
}

/**
 * Show a confirmation dialog. Returns a Promise<boolean>.
 * @param {string} message - The message to display.
 * @param {object} [opts] - Options: { title, confirmLabel, cancelLabel, danger }.
 */
export function modalConfirm(message, opts) {
  opts = opts || {};
  var title = opts.title || "Confirm";
  var confirmLabel = opts.confirmLabel || "OK";
  var cancelLabel = opts.cancelLabel || "Cancel";
  var dangerCls = opts.danger ? " danger" : "";

  var html =
    '<div class="modal-card" role="alertdialog" aria-labelledby="modal-title" aria-describedby="modal-message">' +
      '<div class="modal-title" id="modal-title">' + escapeHtml(title) + "</div>" +
      '<div class="modal-message" id="modal-message">' + escapeHtml(message) + "</div>" +
      '<div class="modal-actions">' +
        '<button class="icon-btn modal-cancel" data-action="cancel">' + escapeHtml(cancelLabel) + "</button>" +
        '<button class="icon-btn' + dangerCls + ' modal-confirm" data-action="confirm">' + escapeHtml(confirmLabel) + "</button>" +
      "</div>" +
    "</div>";

  return showModal(html).then(function (result) {
    return result === "confirm";
  });
}

/**
 * Show a prompt dialog. Returns a Promise<string|null>.
 * @param {string} message - The prompt message.
 * @param {string} [defaultValue] - Default input value.
 * @param {object} [opts] - Options: { title, confirmLabel, cancelLabel, placeholder }.
 */
export function modalPrompt(message, defaultValue, opts) {
  opts = opts || {};
  var title = opts.title || "Input";
  var confirmLabel = opts.confirmLabel || "OK";
  var cancelLabel = opts.cancelLabel || "Cancel";
  var placeholder = opts.placeholder || "";

  var html =
    '<div class="modal-card" role="dialog" aria-labelledby="modal-title" aria-describedby="modal-message">' +
      '<div class="modal-title" id="modal-title">' + escapeHtml(title) + "</div>" +
      '<div class="modal-message" id="modal-message">' + escapeHtml(message) + "</div>" +
      '<input class="modal-input" type="text" value="' + escapeAttr(defaultValue || "") + '"' +
        ' placeholder="' + escapeAttr(placeholder) + '" autocomplete="off" spellcheck="false">' +
      '<div class="modal-actions">' +
        '<button class="icon-btn modal-cancel" data-action="cancel">' + escapeHtml(cancelLabel) + "</button>" +
        '<button class="icon-btn modal-confirm" data-action="confirm">' + escapeHtml(confirmLabel) + "</button>" +
      "</div>" +
    "</div>";

  return showModal(html, true);
}

/**
 * Show an alert dialog. Returns a Promise<void>.
 * @param {string} message - The alert message.
 * @param {object} [opts] - Options: { title, label, danger }.
 */
export function modalAlert(message, opts) {
  opts = opts || {};
  var title = opts.title || (opts.danger ? "Error" : "Notice");
  var label = opts.label || "OK";
  var dangerCls = opts.danger ? " danger" : "";

  var html =
    '<div class="modal-card" role="alertdialog" aria-labelledby="modal-title" aria-describedby="modal-message">' +
      '<div class="modal-title' + dangerCls + '" id="modal-title">' + escapeHtml(title) + "</div>" +
      '<div class="modal-message" id="modal-message">' + escapeHtml(message) + "</div>" +
      '<div class="modal-actions">' +
        '<button class="icon-btn modal-confirm" data-action="confirm">' + escapeHtml(label) + "</button>" +
      "</div>" +
    "</div>";

  return showModal(html).then(function () {
    return undefined;
  });
}

function showModal(html, hasInput) {
  // Cancel any pending modal.
  if (_resolveModal) {
    _resolveModal(null);
    _resolveModal = null;
  }

  var overlay = document.getElementById("modal-overlay");
  overlay.innerHTML = html;
  overlay.classList.remove("hidden");
  overlay.setAttribute("aria-hidden", "false");

  // Focus the input or the confirm button.
  var input = overlay.querySelector(".modal-input");
  var confirmBtn = overlay.querySelector(".modal-confirm");
  var cancelBtn = overlay.querySelector(".modal-cancel");

  if (hasInput && input) {
    input.focus();
    input.select();
  } else if (confirmBtn) {
    confirmBtn.focus();
  }

  return new Promise(function (resolve) {
    _resolveModal = resolve;

    if (confirmBtn) {
      confirmBtn.addEventListener("click", function () {
        if (hasInput && input) {
          closeModal(input.value);
        } else {
          closeModal("confirm");
        }
      });
    }

    if (cancelBtn) {
      cancelBtn.addEventListener("click", function () {
        closeModal(null);
      });
    }

    // Enter key submits.
    if (hasInput && input) {
      input.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter") {
          ev.preventDefault();
          closeModal(input.value);
        }
      });
    }
  });
}

function closeModal(result) {
  var overlay = document.getElementById("modal-overlay");
  overlay.classList.add("hidden");
  overlay.setAttribute("aria-hidden", "true");
  overlay.innerHTML = "";

  if (_resolveModal) {
    var resolve = _resolveModal;
    _resolveModal = null;
    resolve(result);
  }
}

function escapeHtml(str) {
  var div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

function escapeAttr(str) {
  return str.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}
