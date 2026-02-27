// Tests for the virtual scroller module.

import { describe, it, expect, beforeEach, vi } from "vitest";
import { createVirtualScroller } from "./virtual-scroller.js";

/**
 * Build a minimal scrollable container for testing.
 * @param {number} height - Container viewport height in pixels.
 * @returns {HTMLElement}
 */
function makeContainer(height) {
  var el = document.createElement("div");
  // jsdom doesn't compute layout, so we stub clientHeight.
  Object.defineProperty(el, "clientHeight", { value: height, writable: true });
  el.scrollTop = 0;
  return el;
}

describe("createVirtualScroller", function () {
  var container;
  var renderItem;

  beforeEach(function () {
    container = makeContainer(200);
    renderItem = vi.fn(function (index) {
      var div = document.createElement("div");
      div.textContent = "item-" + index;
      div.dataset.index = String(index);
      return div;
    });
  });

  it("renders initial visible items on creation", function () {
    createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 100,
      renderItem: renderItem,
      buffer: 5,
    });

    // viewport: 200px / 40px = 5 visible + 5 buffer below = 10 items (indices 0..10)
    expect(renderItem).toHaveBeenCalled();
    var calls = renderItem.mock.calls.length;
    expect(calls).toBeGreaterThanOrEqual(5);
    expect(calls).toBeLessThanOrEqual(30);
  });

  it("creates a spacer element with correct total height", function () {
    createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 50,
      renderItem: renderItem,
    });

    var spacer = container.querySelector(".vs-spacer");
    expect(spacer).not.toBeNull();
    expect(spacer.style.height).toBe("2000px"); // 50 * 40
  });

  it("update() changes total count and spacer height", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 50,
      renderItem: renderItem,
    });

    scroller.update(100);

    var spacer = container.querySelector(".vs-spacer");
    expect(spacer.style.height).toBe("4000px"); // 100 * 40
  });

  it("destroy() removes event listener and cleans up DOM", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 10,
      renderItem: renderItem,
    });

    scroller.destroy();

    var spacer = container.querySelector(".vs-spacer");
    expect(spacer).toBeNull();
  });

  it("scrollToIndex() adjusts scrollTop when item is below viewport", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 100,
      renderItem: renderItem,
    });

    // Item 20 is at y=800, viewport is 200px tall, so scrollTop should adjust.
    scroller.scrollToIndex(20);
    // target = 20*40=800, needs 800+40-200=640.
    expect(container.scrollTop).toBe(640);
  });

  it("scrollToIndex() does not scroll if item is already visible", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 100,
      renderItem: renderItem,
    });

    container.scrollTop = 0;
    // Item 2 is at y=80, viewport 0..200 so it should be visible.
    scroller.scrollToIndex(2);
    expect(container.scrollTop).toBe(0);
  });

  it("scrollToIndex() clamps to valid range", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 10,
      renderItem: renderItem,
    });

    // Negative index should clamp to 0.
    scroller.scrollToIndex(-5);
    expect(container.scrollTop).toBe(0);

    // Index beyond total should clamp to last.
    scroller.scrollToIndex(999);
    // last = 9, target = 9*40=360, needs 360+40-200=200.
    expect(container.scrollTop).toBe(200);
  });

  it("does nothing after destroy", function () {
    var scroller = createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 10,
      renderItem: renderItem,
    });

    scroller.destroy();
    var callsBefore = renderItem.mock.calls.length;

    // These should be no-ops after destroy.
    scroller.update(50);
    scroller.scrollToIndex(5);
    expect(renderItem.mock.calls.length).toBe(callsBefore);
  });

  it("uses default buffer of 20 when not specified", function () {
    createVirtualScroller({
      container: container,
      itemHeight: 40,
      totalCount: 200,
      renderItem: renderItem,
    });

    // viewport shows 5 items, plus default buffer 20 = up to 25 rendered.
    var calls = renderItem.mock.calls.length;
    expect(calls).toBeGreaterThanOrEqual(5);
    expect(calls).toBeLessThanOrEqual(30);
  });
});
