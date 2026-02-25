// Keyboard shortcuts manager.

const handlers = {};
let enabled = true;

export function register(key, handler, opts = {}) {
  if (!handlers[key]) handlers[key] = [];
  handlers[key].push({ handler, ...opts });
}

export function setEnabled(state) {
  enabled = state;
}

function isInput(el) {
  if (!el) return false;
  var tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
}

document.addEventListener("keydown", function (ev) {
  if (!enabled) return;

  var key = ev.key;
  var mod = (ev.metaKey || ev.ctrlKey) ? "cmd+" : "";
  var shift = ev.shiftKey ? "shift+" : "";
  var combo = mod + shift + key;

  // Check registered handlers.
  var entries = handlers[combo] || handlers[key];
  if (!entries) return;

  for (var i = 0; i < entries.length; i++) {
    var entry = entries[i];

    // Skip if input is focused and handler doesn't want input events.
    if (!entry.inInput && isInput(document.activeElement)) continue;

    // Skip if an overlay is required to be open/closed.
    if (entry.requireOverlay && !document.querySelector(entry.requireOverlay + ".visible")) continue;
    if (entry.noOverlay) {
      var anyOpen = document.querySelector("#search-overlay.visible, #shortcuts-overlay.visible");
      if (anyOpen) continue;
    }

    ev.preventDefault();
    entry.handler(ev);
    return;
  }
});
