// Virtual Scroller — Efficient rendering for large lists and grids.

/**
 * Create a virtual scroller for efficient rendering of large lists.
 * @param {Object} opts
 * @param {HTMLElement} opts.container - Scrollable container element
 * @param {number} opts.itemHeight - Height of each item in pixels
 * @param {number} opts.totalCount - Total number of items
 * @param {Function} opts.renderItem - Function(index) that returns an HTMLElement
 * @param {number} [opts.buffer] - Number of extra items above/below viewport (default: 20)
 * @returns {{ update: Function, scrollToIndex: Function, destroy: Function }}
 */
export function createVirtualScroller(opts) {
  var container = opts.container;
  var itemHeight = opts.itemHeight;
  var totalCount = opts.totalCount;
  var renderItem = opts.renderItem;
  var buffer = opts.buffer !== undefined ? opts.buffer : 20;

  // Track rendered elements by index.
  var _rendered = new Map(); // index -> HTMLElement
  var _destroyed = false;
  var _rafId = null;

  // Create inner wrapper that provides scroll height.
  var _inner = document.createElement("div");
  _inner.className = "vs-inner";
  _inner.style.position = "relative";
  _inner.style.height = (totalCount * itemHeight) + "px";
  _inner.style.pointerEvents = "none";

  // Spacer is the sentinel div that establishes total scroll height.
  var _spacer = document.createElement("div");
  _spacer.className = "vs-spacer";
  _spacer.style.height = (totalCount * itemHeight) + "px";
  _spacer.style.pointerEvents = "none";

  container.style.position = "relative";
  container.style.overflow = "auto";
  container.appendChild(_spacer);

  /**
   * Calculate which item indices are within the visible viewport range.
   * @returns {{ start: number, end: number }}
   */
  function getVisibleRange() {
    var scrollTop = container.scrollTop;
    var viewHeight = container.clientHeight;
    var start = Math.max(0, Math.floor(scrollTop / itemHeight) - buffer);
    var end = Math.min(totalCount - 1, Math.ceil((scrollTop + viewHeight) / itemHeight) + buffer);
    return { start: start, end: end };
  }

  /**
   * Render items in the visible range and remove items outside of it.
   */
  function renderVisible() {
    if (_destroyed) return;
    var range = getVisibleRange();

    // Remove items that are now outside the visible range.
    _rendered.forEach(function (el, idx) {
      if (idx < range.start || idx > range.end) {
        if (el.parentNode === container) {
          container.removeChild(el);
        }
        _rendered.delete(idx);
      }
    });

    // Add items that are now visible but not yet rendered.
    var frag = document.createDocumentFragment();
    var hasNew = false;

    for (var i = range.start; i <= range.end; i++) {
      if (!_rendered.has(i)) {
        var el = renderItem(i);
        el.style.position = "absolute";
        el.style.top = (i * itemHeight) + "px";
        el.style.left = "0";
        el.style.right = "0";
        el.style.height = itemHeight + "px";
        el.style.boxSizing = "border-box";
        _rendered.set(i, el);
        frag.appendChild(el);
        hasNew = true;
      }
    }

    if (hasNew) {
      container.appendChild(frag);
    }
  }

  /**
   * Scroll event handler using rAF to batch updates.
   */
  function onScroll() {
    if (_rafId !== null) return;
    _rafId = requestAnimationFrame(function () {
      _rafId = null;
      renderVisible();
    });
  }

  container.addEventListener("scroll", onScroll);

  // Initial render.
  renderVisible();

  /**
   * Update the scroller with a new total item count.
   * Clears all rendered items and re-renders from current scroll position.
   * @param {number} newCount - New total item count.
   */
  function update(newCount) {
    if (_destroyed) return;
    totalCount = newCount;

    // Update sentinel height.
    _spacer.style.height = (totalCount * itemHeight) + "px";

    // Clear all rendered items.
    _rendered.forEach(function (el) {
      if (el.parentNode === container) {
        container.removeChild(el);
      }
    });
    _rendered.clear();

    renderVisible();
  }

  /**
   * Scroll the container so that the item at the given index is visible.
   * @param {number} index - Zero-based item index.
   */
  function scrollToIndex(index) {
    if (_destroyed) return;
    var clamped = Math.max(0, Math.min(index, totalCount - 1));
    var targetTop = clamped * itemHeight;
    var scrollTop = container.scrollTop;
    var viewHeight = container.clientHeight;

    if (targetTop < scrollTop) {
      container.scrollTop = targetTop;
    } else if (targetTop + itemHeight > scrollTop + viewHeight) {
      container.scrollTop = targetTop + itemHeight - viewHeight;
    }
  }

  /**
   * Tear down the virtual scroller, removing event listeners and DOM nodes.
   */
  function destroy() {
    _destroyed = true;
    if (_rafId !== null) {
      cancelAnimationFrame(_rafId);
      _rafId = null;
    }
    container.removeEventListener("scroll", onScroll);

    _rendered.forEach(function (el) {
      if (el.parentNode === container) {
        container.removeChild(el);
      }
    });
    _rendered.clear();

    if (_spacer.parentNode === container) {
      container.removeChild(_spacer);
    }
  }

  return { update: update, scrollToIndex: scrollToIndex, destroy: destroy };
}
