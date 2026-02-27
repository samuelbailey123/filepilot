// Lazy library loaders — load CDN scripts on demand instead of eagerly.
// Each loader returns a cached promise so subsequent calls resolve instantly.

var _d3Promise = null;
var _hljsPromise = null;
var _pdfjsPromise = null;

/**
 * Lazy-load D3.js from CDN.
 * @returns {Promise<object>} Resolves with window.d3.
 */
export function loadD3() {
  if (_d3Promise) return _d3Promise;
  _d3Promise = new Promise(function (resolve, reject) {
    if (window.d3) { resolve(window.d3); return; }
    var script = document.createElement("script");
    script.src = "https://cdnjs.cloudflare.com/ajax/libs/d3/7.9.0/d3.min.js";
    script.onload = function () { resolve(window.d3); };
    script.onerror = function () { reject(new Error("Failed to load D3.js")); };
    document.head.appendChild(script);
  });
  return _d3Promise;
}

/**
 * Lazy-load highlight.js from CDN.
 * @returns {Promise<object>} Resolves with window.hljs.
 */
export function loadHighlightJs() {
  if (_hljsPromise) return _hljsPromise;
  _hljsPromise = new Promise(function (resolve, reject) {
    if (window.hljs) { resolve(window.hljs); return; }
    var script = document.createElement("script");
    script.src = "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js";
    script.onload = function () { resolve(window.hljs); };
    script.onerror = function () { reject(new Error("Failed to load highlight.js")); };
    document.head.appendChild(script);
  });
  return _hljsPromise;
}

/**
 * Lazy-load PDF.js from CDN.
 * @returns {Promise<object>} Resolves with the pdfjsLib module.
 */
export function loadPdfJs() {
  if (_pdfjsPromise) return _pdfjsPromise;
  _pdfjsPromise = new Promise(function (resolve, reject) {
    var existing = window["pdfjs-dist/build/pdf"];
    if (existing) { resolve(existing); return; }
    import("https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.0.379/pdf.min.mjs").then(function (mod) {
      var lib = mod.default || mod;
      if (lib.GlobalWorkerOptions) {
        lib.GlobalWorkerOptions.workerSrc = "https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.0.379/pdf.worker.min.mjs";
      }
      resolve(lib);
    }).catch(function (err) {
      reject(err);
    });
  });
  return _pdfjsPromise;
}
