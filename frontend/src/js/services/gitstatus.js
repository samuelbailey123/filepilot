// Git Status Service — Fetches and caches git status for file views.

import * as api from "./api.js";

var _cache = {};       // repoRoot -> { statuses: map, timestamp: number }
var _repoRoots = {};   // dirPath -> repoRoot (cached lookups)
var _cacheTTL = 5000;  // 5 seconds before stale.

/**
 * Get the git status map for a directory path.
 * Returns a map of absolute file path -> status string.
 * @param {string} dirPath - The directory being viewed.
 * @returns {Promise<Object>} Map of path to status, or empty object.
 */
export async function getGitStatusForDir(dirPath) {
  if (!dirPath) return {};

  try {
    var root = _repoRoots[dirPath];
    if (root === null) return {}; // Known non-repo.
    if (!root) {
      root = await api.gitRepoRoot(dirPath);
      if (!root) {
        _repoRoots[dirPath] = null;
        return {};
      }
      _repoRoots[dirPath] = root;
    }

    // Check cache.
    var cached = _cache[root];
    if (cached && (Date.now() - cached.timestamp) < _cacheTTL) {
      return cached.statuses;
    }

    var statuses = await api.getGitStatus(root);
    if (!statuses) statuses = {};

    _cache[root] = { statuses: statuses, timestamp: Date.now() };
    return statuses;
  } catch (e) {
    // Not a git repo or git not available.
    _repoRoots[dirPath] = null;
    return {};
  }
}

/**
 * Invalidate the cache for a directory path.
 * @param {string} dirPath - The directory path.
 */
export function invalidateGitCache(dirPath) {
  var root = _repoRoots[dirPath];
  if (root && _cache[root]) {
    delete _cache[root];
  }
}

/**
 * Get a CSS class name for a git file status value.
 * @param {string} status - The status string from the backend.
 * @returns {string} CSS class name.
 */
export function gitStatusClass(status) {
  switch (status) {
    case "modified":  return "git-modified";
    case "added":     return "git-added";
    case "staged":    return "git-staged";
    case "deleted":   return "git-deleted";
    case "renamed":   return "git-renamed";
    case "untracked": return "git-untracked";
    case "ignored":   return "git-ignored";
    default:          return "";
  }
}

/**
 * Get a short label for a git status value.
 * @param {string} status - The status string.
 * @returns {string} Short label (e.g., "M", "A", "?").
 */
export function gitStatusLabel(status) {
  switch (status) {
    case "modified":  return "M";
    case "added":     return "A";
    case "staged":    return "S";
    case "deleted":   return "D";
    case "renamed":   return "R";
    case "untracked": return "?";
    case "ignored":   return "!";
    default:          return "";
  }
}
