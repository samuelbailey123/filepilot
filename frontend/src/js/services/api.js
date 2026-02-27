// Wails binding client — wraps Go backend methods.
// Waits for window.go bindings to be available before calling.

var _ready = null;

function waitForBindings() {
  if (_ready) return _ready;
  _ready = new Promise(function (resolve) {
    if (window.go && window.go.main && window.go.main.App) {
      resolve();
      return;
    }
    // Use rAF loop (~16ms checks) instead of setInterval polling.
    var start = performance.now();
    function check() {
      if (window.go && window.go.main && window.go.main.App) {
        resolve();
      } else if (performance.now() - start > 10000) {
        console.error("Wails bindings not available after 10s");
        resolve(); // resolve anyway so callers fail with clear errors
      } else {
        requestAnimationFrame(check);
      }
    }
    requestAnimationFrame(check);
  });
  return _ready;
}

async function call(method) {
  await waitForBindings();
  var args = Array.prototype.slice.call(arguments, 1);
  var fn = window.go.main.App[method];
  if (!fn) throw new Error("Unknown binding: App." + method);
  return fn.apply(null, args);
}

export async function getHomeDir() {
  return call("GetHomeDir");
}

export async function listDir(path) {
  return call("ListDir", path);
}

export async function getEntry(path) {
  return call("GetEntry", path);
}

export async function search(query, limit) {
  return call("Search", query, limit || 50);
}

export async function searchInDir(query, dirPath, limit) {
  return call("SearchInDir", query, dirPath, limit || 50);
}

export async function substringSearch(query, limit) {
  return call("SubstringSearch", query, limit || 50);
}

export async function substringSearchInDir(query, dirPath, limit) {
  return call("SubstringSearchInDir", query, dirPath, limit || 50);
}

export async function regexSearch(pattern, limit) {
  return call("RegexSearch", pattern, limit || 50);
}

export async function regexSearchInDir(pattern, dirPath, limit) {
  return call("RegexSearchInDir", pattern, dirPath, limit || 50);
}

export async function serveFileURL(path) {
  return call("ServeFileURL", path);
}

export async function getPreview(path) {
  return call("GetPreview", path);
}

export async function requestICloudDownload(path) {
  return call("RequestICloudDownload", path);
}

export async function moveFile(src, dst) {
  return call("MoveFile", src, dst);
}

export async function copyFile(src, dst) {
  return call("CopyFile", src, dst);
}

export async function renameFile(path, newName) {
  return call("RenameFile", path, newName);
}

export async function deleteFile(path) {
  return call("DeleteFile", path);
}

export async function undo() {
  return call("Undo");
}

export async function undoCount() {
  return call("UndoCount");
}

export async function openFile(path) {
  return call("OpenFile", path);
}

export async function revealInFinder(path) {
  return call("RevealInFinder", path);
}

export async function createNewDir(path) {
  return call("CreateNewDir", path);
}

export async function createNewFile(path) {
  return call("CreateNewFile", path);
}

export async function getFavorites() {
  return call("GetFavorites");
}

export async function addFavorite(path, label) {
  return call("AddFavorite", path, label);
}

export async function removeFavorite(path) {
  return call("RemoveFavorite", path);
}

export async function getVolumes() {
  return call("GetVolumes");
}

export async function ejectVolume(mountPath) {
  return call("EjectVolume", mountPath);
}

export async function getDiskUsage(path) {
  return call("GetDiskUsage", path);
}

export async function getFolderSize(path) {
  return call("GetFolderSize", path);
}

export async function getScanProgress() {
  return call("GetScanProgress");
}

export async function getStats() {
  return call("GetStats");
}

export async function formatSize(bytes) {
  return call("FormatSize", bytes);
}

export async function formatTime(unix) {
  return call("FormatTime", unix);
}

export async function saveKeymap(keymapJSON) {
  return call("SaveKeymap", keymapJSON);
}

export async function loadKeymap() {
  return call("LoadKeymap");
}

export async function batchRename(ops) {
  return call("BatchRename", ops);
}

export async function saveWorkspace(name, stateJSON) {
  return call("SaveWorkspace", name, stateJSON);
}

export async function loadWorkspace(name) {
  return call("LoadWorkspace", name);
}

export async function listWorkspaces() {
  return call("ListWorkspaces");
}

export async function deleteWorkspace(name) {
  return call("DeleteWorkspace", name);
}

export async function getGitStatus(dirPath) {
  return call("GetGitStatus", dirPath);
}

export async function gitRepoRoot(dirPath) {
  return call("GitRepoRoot", dirPath);
}

export async function listSavedConnections() {
  return call("ListSavedConnections");
}

export async function saveConnection(connJSON) {
  return call("SaveConnection", connJSON);
}

export async function deleteConnection(connID) {
  return call("DeleteConnectionByID", connID);
}

export async function connectSFTP(connID, password) {
  return call("ConnectSFTP", connID, password);
}

export async function connectFTP(connID, password) {
  return call("ConnectFTP", connID, password);
}

export async function connectWebDAV(connID, password) {
  return call("ConnectWebDAV", connID, password);
}

export async function connectS3(connID, secretKey) {
  return call("ConnectS3", connID, secretKey);
}

export async function disconnectRemote(connID) {
  return call("DisconnectRemote", connID);
}

export async function serveRemoteFileURL(remotePath) {
  return call("ServeRemoteFileURL", remotePath);
}

export async function uploadRemoteFile(remotePath) {
  return call("UploadRemoteFile", remotePath);
}

export async function compareFolders(leftPath, rightPath, ignoreHidden) {
  return call("CompareFolders", leftPath, rightPath, ignoreHidden || false);
}

export async function syncFolders(leftPath, rightPath, direction, ignoreHidden) {
  return call("SyncFolders", leftPath, rightPath, direction, ignoreHidden || false);
}

// --- Phase 5 Bindings ---

export async function completePath(partial) {
  return call("CompletePath", partial);
}

export async function createSymlink(target, linkPath) {
  return call("CreateSymlink", target, linkPath);
}

export async function compressFiles(paths, outputPath) {
  return call("CompressFiles", paths, outputPath);
}

export async function setPermissions(path, mode) {
  return call("SetPermissions", path, mode);
}

export async function getFileInfo(path) {
  return call("GetFileInfo", path);
}

export async function getFileTags(path) {
  return call("GetFileTags", path);
}

export async function setFileTags(path, tags) {
  return call("SetFileTags", path, tags);
}

export async function diffFiles(pathA, pathB) {
  return call("DiffFiles", pathA, pathB);
}

export async function listOpenWithApps(path) {
  return call("ListOpenWithApps", path);
}

export async function openWithApp(path, appName) {
  return call("OpenWithApp", path, appName);
}

export async function duplicateFile(path) {
  return call("DuplicateFile", path);
}

export async function pickDirectory() {
  return call("PickDirectory");
}

// --- Phase 6 Bindings ---

export async function extractArchive(archivePath, destDir) { return call("ExtractArchive", archivePath, destDir); }
export async function openTerminal(path) { return call("OpenTerminal", path); }
export async function getThumbnail(path, size) { return call("GetThumbnail", path, size || 128); }
export async function grepInDir(query, dirPath, limit) { return call("GrepInDir", query, dirPath, limit || 50); }

// --- Op Queue Controls ---

export async function pauseOp(id) { return call("PauseOp", id); }
export async function resumeOp(id) { return call("ResumeOp", id); }
export async function cancelOp(id) { return call("CancelOp", id); }
export async function listOps() { return call("ListOps"); }
