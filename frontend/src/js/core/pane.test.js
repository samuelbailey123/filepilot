// Tests for the pane module (tab and pane state management).

import { describe, it, expect, beforeEach } from "vitest";
import {
  createTab,
  createPane,
  addTab,
  closeTab,
  switchTab,
  selectInPane,
  deselectInPane,
  togglePaneHidden,
  setPaneView,
  setPaneFilter,
  toggleSelectInPane,
  rangeSelectInPane,
  clearSelectionInPane,
  selectAllInPane,
} from "./pane.js";

describe("createTab", function () {
  it("returns a tab with default state", function () {
    var tab = createTab();
    expect(tab.currentPath).toBeNull();
    expect(tab.selectedFile).toBeNull();
    expect(tab.selectedFiles).toBeInstanceOf(Set);
    expect(tab.selectedFiles.size).toBe(0);
    expect(tab.view).toBe("column");
    expect(tab.showHidden).toBe(false);
    expect(tab.filterExt).toBeNull();
    expect(tab.entries).toEqual([]);
    expect(tab.historyBack).toEqual([]);
    expect(tab.historyForward).toEqual([]);
  });

  it("accepts initial overrides", function () {
    var tab = createTab({ currentPath: "/tmp", view: "list", showHidden: true });
    expect(tab.currentPath).toBe("/tmp");
    expect(tab.view).toBe("list");
    expect(tab.showHidden).toBe(true);
  });

  it("assigns unique IDs", function () {
    var a = createTab();
    var b = createTab();
    expect(a.id).not.toBe(b.id);
  });
});

describe("createPane", function () {
  it("creates a pane with one default tab", function () {
    var pane = createPane();
    expect(pane.tabs.length).toBe(1);
    expect(pane.activeTabIndex).toBe(0);
  });

  it("proxies state from the active tab", function () {
    var pane = createPane({ currentPath: "/home", view: "grid" });
    expect(pane.currentPath).toBe("/home");
    expect(pane.view).toBe("grid");
  });

  it("assigns unique IDs", function () {
    var a = createPane();
    var b = createPane();
    expect(a.id).not.toBe(b.id);
  });
});

describe("addTab", function () {
  it("adds a tab and switches to it", function () {
    var pane = createPane({ currentPath: "/a" });
    addTab(pane, { currentPath: "/b" });
    expect(pane.tabs.length).toBe(2);
    expect(pane.activeTabIndex).toBe(1);
    expect(pane.currentPath).toBe("/b");
  });

  it("defaults to current tab path when no path specified", function () {
    var pane = createPane({ currentPath: "/orig" });
    addTab(pane);
    expect(pane.currentPath).toBe("/orig");
  });
});

describe("closeTab", function () {
  it("removes a tab by index", function () {
    var pane = createPane();
    addTab(pane, { currentPath: "/b" });
    addTab(pane, { currentPath: "/c" });
    expect(pane.tabs.length).toBe(3);

    var closed = closeTab(pane, 1);
    expect(closed).toBe(true);
    expect(pane.tabs.length).toBe(2);
  });

  it("does not close the last tab", function () {
    var pane = createPane();
    var closed = closeTab(pane, 0);
    expect(closed).toBe(false);
    expect(pane.tabs.length).toBe(1);
  });

  it("adjusts activeTabIndex when closing active tab at end", function () {
    var pane = createPane();
    addTab(pane);
    addTab(pane);
    // Active = 2 (last), close it.
    closeTab(pane, 2);
    expect(pane.activeTabIndex).toBe(1);
  });
});

describe("switchTab", function () {
  it("switches to the given tab index", function () {
    var pane = createPane({ currentPath: "/a" });
    addTab(pane, { currentPath: "/b" });
    switchTab(pane, 0);
    expect(pane.activeTabIndex).toBe(0);
    expect(pane.currentPath).toBe("/a");
  });

  it("ignores out-of-range indices", function () {
    var pane = createPane();
    switchTab(pane, 5);
    expect(pane.activeTabIndex).toBe(0);
    switchTab(pane, -1);
    expect(pane.activeTabIndex).toBe(0);
  });
});

describe("selectInPane / deselectInPane", function () {
  it("selects and deselects a file", function () {
    var pane = createPane();
    selectInPane(pane, "/a/file.txt");
    expect(pane.selectedFile).toBe("/a/file.txt");

    deselectInPane(pane);
    expect(pane.selectedFile).toBeNull();
  });
});

describe("togglePaneHidden", function () {
  it("toggles hidden file visibility", function () {
    var pane = createPane();
    expect(pane.showHidden).toBe(false);
    togglePaneHidden(pane);
    expect(pane.showHidden).toBe(true);
    togglePaneHidden(pane);
    expect(pane.showHidden).toBe(false);
  });
});

describe("setPaneView", function () {
  it("sets the view mode", function () {
    var pane = createPane();
    setPaneView(pane, "list");
    expect(pane.view).toBe("list");
    setPaneView(pane, "graph");
    expect(pane.view).toBe("graph");
  });
});

describe("setPaneFilter", function () {
  it("sets the extension filter", function () {
    var pane = createPane();
    setPaneFilter(pane, "js");
    expect(pane.filterExt).toBe("js");
  });

  it("toggles off when set to the same extension", function () {
    var pane = createPane();
    setPaneFilter(pane, "js");
    setPaneFilter(pane, "js");
    expect(pane.filterExt).toBeNull();
  });
});

describe("multi-selection", function () {
  it("toggleSelectInPane adds and removes from selection", function () {
    var pane = createPane();
    toggleSelectInPane(pane, "/a");
    expect(pane.selectedFiles.has("/a")).toBe(true);
    expect(pane.anchorFile).toBe("/a");

    toggleSelectInPane(pane, "/a");
    expect(pane.selectedFiles.has("/a")).toBe(false);
  });

  it("rangeSelectInPane selects a range of files", function () {
    var pane = createPane();
    var entries = [
      { path: "/a" }, { path: "/b" }, { path: "/c" }, { path: "/d" }, { path: "/e" },
    ];

    // Set anchor first.
    toggleSelectInPane(pane, "/b");

    rangeSelectInPane(pane, "/d", entries);
    expect(pane.selectedFiles.size).toBe(3);
    expect(pane.selectedFiles.has("/b")).toBe(true);
    expect(pane.selectedFiles.has("/c")).toBe(true);
    expect(pane.selectedFiles.has("/d")).toBe(true);
  });

  it("rangeSelectInPane without anchor selects single file", function () {
    var pane = createPane();
    var entries = [{ path: "/a" }, { path: "/b" }];

    rangeSelectInPane(pane, "/b", entries);
    expect(pane.selectedFiles.size).toBe(1);
    expect(pane.selectedFiles.has("/b")).toBe(true);
    expect(pane.anchorFile).toBe("/b");
  });

  it("clearSelectionInPane clears everything", function () {
    var pane = createPane();
    toggleSelectInPane(pane, "/a");
    toggleSelectInPane(pane, "/b");
    clearSelectionInPane(pane);
    expect(pane.selectedFiles.size).toBe(0);
    expect(pane.anchorFile).toBeNull();
  });

  it("selectAllInPane selects all entries", function () {
    var pane = createPane();
    pane.entries = [{ path: "/a" }, { path: "/b" }, { path: "/c" }];
    selectAllInPane(pane);
    expect(pane.selectedFiles.size).toBe(3);
    expect(pane.selectedFiles.has("/a")).toBe(true);
    expect(pane.selectedFiles.has("/b")).toBe(true);
    expect(pane.selectedFiles.has("/c")).toBe(true);
  });
});
