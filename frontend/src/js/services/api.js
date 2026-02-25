// Wails binding client — wraps Go backend methods.

export async function listDir(path) {
  return window.go.main.App.ListDir(path);
}

export async function getEntry(path) {
  return window.go.main.App.GetEntry(path);
}

export async function search(query, limit) {
  return window.go.main.App.Search(query, limit || 50);
}

export async function getPreview(path) {
  return window.go.main.App.GetPreview(path);
}

export async function moveFile(src, dst) {
  return window.go.main.App.MoveFile(src, dst);
}

export async function copyFile(src, dst) {
  return window.go.main.App.CopyFile(src, dst);
}

export async function renameFile(path, newName) {
  return window.go.main.App.RenameFile(path, newName);
}

export async function deleteFile(path) {
  return window.go.main.App.DeleteFile(path);
}

export async function undo() {
  return window.go.main.App.Undo();
}

export async function undoCount() {
  return window.go.main.App.UndoCount();
}

export async function openFile(path) {
  return window.go.main.App.OpenFile(path);
}

export async function revealInFinder(path) {
  return window.go.main.App.RevealInFinder(path);
}

export async function createNewDir(path) {
  return window.go.main.App.CreateNewDir(path);
}

export async function createNewFile(path) {
  return window.go.main.App.CreateNewFile(path);
}

export async function getFavorites() {
  return window.go.main.App.GetFavorites();
}

export async function addFavorite(path, label) {
  return window.go.main.App.AddFavorite(path, label);
}

export async function removeFavorite(path) {
  return window.go.main.App.RemoveFavorite(path);
}

export async function getVolumes() {
  return window.go.main.App.GetVolumes();
}

export async function getScanProgress() {
  return window.go.main.App.GetScanProgress();
}

export async function getStats() {
  return window.go.main.App.GetStats();
}

export async function formatSize(bytes) {
  return window.go.main.App.FormatSize(bytes);
}

export async function formatTime(unix) {
  return window.go.main.App.FormatTime(unix);
}
