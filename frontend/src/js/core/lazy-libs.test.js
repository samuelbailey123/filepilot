// Tests for the lazy library loader module.

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// We need to re-import with fresh module state between tests.
// Use dynamic import so each test gets a clean module.

/**
 * Import a fresh copy of lazy-libs with reset module cache.
 * @returns {Promise<Object>}
 */
async function freshImport() {
  // Bust vitest module cache by appending a unique query param.
  var id = Date.now() + "-" + Math.random();
  return import("./lazy-libs.js?" + id);
}

describe("loadD3", function () {
  beforeEach(function () {
    delete window.d3;
  });

  afterEach(function () {
    // Clean up any injected script tags.
    document.querySelectorAll("script").forEach(function (s) { s.remove(); });
    delete window.d3;
  });

  it("resolves immediately when window.d3 already exists", async function () {
    window.d3 = { version: "7.9.0" };
    var mod = await freshImport();
    var result = await mod.loadD3();
    expect(result).toBe(window.d3);
  });

  it("injects a script tag when d3 is not loaded", async function () {
    var mod = await freshImport();
    var promise = mod.loadD3();

    // A script should have been added to head.
    var scripts = document.querySelectorAll("script");
    var d3Script = Array.from(scripts).find(function (s) {
      return s.src.includes("d3.min.js");
    });
    expect(d3Script).toBeDefined();

    // Simulate the script loading.
    window.d3 = { version: "7.9.0" };
    d3Script.onload();

    var result = await promise;
    expect(result.version).toBe("7.9.0");
  });

  it("returns the same promise on subsequent calls", async function () {
    window.d3 = { version: "7.9.0" };
    var mod = await freshImport();
    var p1 = mod.loadD3();
    var p2 = mod.loadD3();
    expect(p1).toBe(p2);
  });
});

describe("loadHighlightJs", function () {
  beforeEach(function () {
    delete window.hljs;
  });

  afterEach(function () {
    document.querySelectorAll("script").forEach(function (s) { s.remove(); });
    delete window.hljs;
  });

  it("resolves immediately when window.hljs already exists", async function () {
    window.hljs = { highlight: function () {} };
    var mod = await freshImport();
    var result = await mod.loadHighlightJs();
    expect(result).toBe(window.hljs);
  });

  it("injects a script tag when hljs is not loaded", async function () {
    var mod = await freshImport();
    var promise = mod.loadHighlightJs();

    var scripts = document.querySelectorAll("script");
    var hljsScript = Array.from(scripts).find(function (s) {
      return s.src.includes("highlight.min.js");
    });
    expect(hljsScript).toBeDefined();

    window.hljs = { highlight: function () {} };
    hljsScript.onload();

    var result = await promise;
    expect(result).toBe(window.hljs);
  });

  it("caches the promise across calls", async function () {
    window.hljs = { highlight: function () {} };
    var mod = await freshImport();
    var p1 = mod.loadHighlightJs();
    var p2 = mod.loadHighlightJs();
    expect(p1).toBe(p2);
  });
});

describe("loadPdfJs", function () {
  beforeEach(function () {
    delete window["pdfjs-dist/build/pdf"];
  });

  afterEach(function () {
    delete window["pdfjs-dist/build/pdf"];
  });

  it("resolves immediately when pdfjs already exists on window", async function () {
    var fakeLib = { GlobalWorkerOptions: {} };
    window["pdfjs-dist/build/pdf"] = fakeLib;
    var mod = await freshImport();
    var result = await mod.loadPdfJs();
    expect(result).toBe(fakeLib);
  });

  it("caches the promise across calls", async function () {
    var fakeLib = { GlobalWorkerOptions: {} };
    window["pdfjs-dist/build/pdf"] = fakeLib;
    var mod = await freshImport();
    var p1 = mod.loadPdfJs();
    var p2 = mod.loadPdfJs();
    expect(p1).toBe(p2);
  });
});
