// Keyboard shortcuts manager — configurable keymap system.
//
// Actions are registered by name with registerAction(). The keymap maps action
// names to key combos. User overrides are stored in localStorage and can
// optionally be persisted to the Go backend via SaveKeymap/LoadKeymap.

var _actions = {};
var _enabled = true;
var _userOverrides = {};

// Default keymap: action name -> key combo.
var _defaultKeymap = {
  "search":           "/",
  "searchInFolder":   "cmd+shift+f",
  "quickLook":        " ",
  "switchPane":       "Tab",
  "toggleDualPane":   "cmd+\\",
  "escape":           "Escape",
  "showShortcuts":    "?",
  "undo":             "cmd+z",
  "toggleHidden":     ".",
  "toggleTheme":      "t",
  "openSettings":     "cmd+,",
  "syncBrowsing":     "cmd+shift+s",
  "newTab":           "cmd+t",
  "closeTab":         "cmd+w",
  "saveWorkspace":    "cmd+shift+w",
  "selectAll":        "cmd+a",
  "quickFilter":      "cmd+f",
  "commandPalette":   "cmd+k",
  "getInfo":          "cmd+i",
  "compareFiles":     "cmd+shift+d",
  "copyFiles":        "cmd+c",
  "cutFiles":         "cmd+x",
  "pasteFiles":       "cmd+v",
  "navBack":          "cmd+[",
  "navForward":       "cmd+]",
  "openTerminal":     "cmd+`",
};

// Preset keymaps. Each preset specifies overrides on top of the default keymap.
var _presets = {
  "default": {},
  "commander": {
    "search":       "cmd+f",
    "toggleHidden": "cmd+.",
    "toggleTheme":  "cmd+t",
  },
  "finder": {
    "search":         "cmd+f",
    "toggleHidden":   "cmd+shift+.",
    "undo":           "cmd+z",
    "toggleTheme":    "cmd+shift+t",
  },
};

/**
 * Register an action handler. The action is bound to whatever key the keymap
 * maps its name to.
 * @param {string} name - Unique action identifier (e.g., "search").
 * @param {string} label - Human-readable label (e.g., "Search files").
 * @param {Function} handler - The callback to invoke.
 * @param {Object} [opts] - Options: { inInput, noOverlay, requireOverlay }.
 */
export function registerAction(name, label, handler, opts) {
  _actions[name] = { name: name, label: label, handler: handler, opts: opts || {} };
}

/**
 * Execute a named action's handler directly.
 * Used by the command palette to trigger actions.
 * @param {string} name - The action name.
 * @returns {boolean} True if the action was found and executed.
 */
export function executeAction(name) {
  var entry = _actions[name];
  if (entry && entry.handler) {
    entry.handler();
    return true;
  }
  return false;
}

/**
 * Legacy register function for backward compatibility.
 * @param {string} key - Key combo string.
 * @param {Function} handler - Callback.
 * @param {Object} [opts] - Options.
 */
export function register(key, handler, opts) {
  // Create an auto-generated action name from the key.
  var name = "_legacy_" + key;
  _actions[name] = { name: name, label: key, handler: handler, opts: opts || {}, legacyKey: key };
}

/**
 * Enable or disable shortcut handling.
 * @param {boolean} state - Whether shortcuts are enabled.
 */
export function setEnabled(state) {
  _enabled = state;
}

/**
 * Get the current effective keymap (defaults merged with user overrides).
 * @returns {Object} Map of action name -> key combo.
 */
export function getKeymap() {
  var keymap = {};
  // Start with defaults.
  for (var key in _defaultKeymap) {
    keymap[key] = _defaultKeymap[key];
  }
  // Apply user overrides.
  for (var key in _userOverrides) {
    keymap[key] = _userOverrides[key];
  }
  // Add legacy actions with their fixed keys.
  for (var name in _actions) {
    if (_actions[name].legacyKey && !keymap[name]) {
      keymap[name] = _actions[name].legacyKey;
    }
  }
  return keymap;
}

/**
 * Get the default keymap (without user overrides).
 * @returns {Object} Map of action name -> key combo.
 */
export function getDefaultKeymap() {
  var keymap = {};
  for (var key in _defaultKeymap) {
    keymap[key] = _defaultKeymap[key];
  }
  return keymap;
}

/**
 * Get all registered actions with their current key bindings.
 * @returns {Array} List of { name, label, key, isDefault } objects.
 */
export function getActions() {
  var keymap = getKeymap();
  var result = [];
  for (var name in _actions) {
    if (_actions[name].legacyKey) continue; // Skip legacy entries.
    result.push({
      name: name,
      label: _actions[name].label,
      key: keymap[name] || "",
      isDefault: !_userOverrides[name],
    });
  }
  return result;
}

/**
 * Get the available preset names.
 * @returns {Array} List of preset name strings.
 */
export function getPresets() {
  return Object.keys(_presets);
}

/**
 * Apply a preset keymap by name.
 * @param {string} presetName - One of the preset names.
 */
export function applyPreset(presetName) {
  var preset = _presets[presetName];
  if (!preset) return;
  _userOverrides = {};
  for (var key in preset) {
    _userOverrides[key] = preset[key];
  }
  _saveOverrides();
}

/**
 * Rebind a single action to a new key combo.
 * @param {string} actionName - The action to rebind.
 * @param {string} newKey - The new key combo string.
 * @returns {string|null} Name of any displaced action, or null.
 */
export function rebind(actionName, newKey) {
  if (!_actions[actionName]) return null;

  // Check if this key is already bound to another action.
  var keymap = getKeymap();
  var displaced = null;
  for (var name in keymap) {
    if (name !== actionName && keymap[name] === newKey && _actions[name]) {
      displaced = name;
      break;
    }
  }

  // If the new key matches the default, remove the override.
  if (_defaultKeymap[actionName] === newKey) {
    delete _userOverrides[actionName];
  } else {
    _userOverrides[actionName] = newKey;
  }

  // If another action was displaced, unbind it.
  if (displaced) {
    _userOverrides[displaced] = "";
  }

  _saveOverrides();
  return displaced;
}

/**
 * Reset all keybindings to defaults.
 */
export function resetKeymap() {
  _userOverrides = {};
  _saveOverrides();
}

/**
 * Load user overrides from localStorage.
 */
export function loadKeymap() {
  try {
    var stored = localStorage.getItem("fp-keymap");
    if (stored) {
      _userOverrides = JSON.parse(stored);
    }
  } catch (e) {
    _userOverrides = {};
  }
}

/**
 * Load user overrides from a JSON string (e.g., from Go backend).
 * @param {string} json - JSON string of keymap overrides.
 */
export function loadKeymapFromJSON(json) {
  if (!json) return;
  try {
    _userOverrides = JSON.parse(json);
    _saveOverrides();
  } catch (e) {
    // Ignore invalid JSON.
  }
}

/**
 * Export the current user overrides as a JSON string.
 * @returns {string} JSON string of keymap overrides.
 */
export function exportKeymap() {
  return JSON.stringify(_userOverrides);
}

/**
 * Format a key combo for display (e.g., "cmd+shift+f" -> "⌘⇧F").
 * @param {string} combo - The raw key combo string.
 * @returns {string} Human-readable display string.
 */
export function formatKeyCombo(combo) {
  if (!combo) return "—";
  return combo
    .replace(/cmd\+/g, "\u2318")
    .replace(/shift\+/g, "\u21E7")
    .replace(/alt\+/g, "\u2325")
    .replace(/ctrl\+/g, "\u2303")
    .replace(/Escape/, "Esc")
    .replace(/Backspace/, "\u232B")
    .replace(/Tab/, "\u21E5")
    .replace(/ $/, "Space")
    .replace(/^ $/, "Space");
}

// Internal: save overrides to localStorage.
function _saveOverrides() {
  try {
    localStorage.setItem("fp-keymap", JSON.stringify(_userOverrides));
  } catch (e) {
    // Storage quota exceeded or unavailable.
  }
}

function isInput(el) {
  if (!el) return false;
  var tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
}

// Build a reverse lookup: key combo -> action name.
function buildKeyIndex() {
  var keymap = getKeymap();
  var index = {};
  for (var name in keymap) {
    var key = keymap[name];
    if (key && _actions[name]) {
      if (!index[key]) index[key] = [];
      index[key].push(name);
    }
  }
  return index;
}

// Global keydown listener.
document.addEventListener("keydown", function (ev) {
  if (!_enabled) return;

  var key = ev.key;
  var mod = (ev.metaKey || ev.ctrlKey) ? "cmd+" : "";
  var shift = ev.shiftKey ? "shift+" : "";
  var combo = mod + shift + key;

  var keyIndex = buildKeyIndex();

  // Try the full combo first, then the key alone.
  var matches = keyIndex[combo] || keyIndex[key];
  if (!matches) return;

  for (var i = 0; i < matches.length; i++) {
    var actionName = matches[i];
    var entry = _actions[actionName];
    if (!entry) continue;

    var opts = entry.opts;

    // Skip if input is focused and handler doesn't want input events.
    if (!opts.inInput && isInput(document.activeElement)) continue;

    // Skip if an overlay is required to be open/closed.
    if (opts.requireOverlay && !document.querySelector(opts.requireOverlay + ".visible")) continue;
    if (opts.noOverlay) {
      var anyOpen = document.querySelector(
        "#search-overlay.visible, #shortcuts-overlay.visible, #quicklook-overlay.visible, #settings-overlay.visible"
      );
      if (anyOpen) continue;
    }

    ev.preventDefault();
    entry.handler(ev);
    return;
  }
});

// Load saved overrides on module init.
loadKeymap();
