// Graph View — D3.js force-directed graph with Apple-inspired design.

import { getState, selectFile, navigateTo } from "../app.js";
import { getFileColorClass } from "./column.js";
import { loadD3 } from "../core/lazy-libs.js";

var currentSimulation = null;

// Soft, distinct folder colors — Apple-inspired palette.
var folderPalette = [
  { fill: "#5AC8FA", stroke: "#3AA0D8" },  // blue
  { fill: "#AF52DE", stroke: "#8E3CB8" },  // purple
  { fill: "#FF9500", stroke: "#D47A00" },  // orange
  { fill: "#34C759", stroke: "#28A745" },  // green
  { fill: "#FF6482", stroke: "#D94A66" },  // pink
  { fill: "#5856D6", stroke: "#4240B0" },  // indigo
  { fill: "#00C7BE", stroke: "#009E97" },  // teal
  { fill: "#FFD60A", stroke: "#D4B000" },  // yellow
  { fill: "#FF453A", stroke: "#D43830" },  // red
  { fill: "#64D2FF", stroke: "#4AB0D8" },  // cyan
];

// File-type color mapping for vibrant fills.
var fileTypeColors = {
  code:    { fill: "#5AC8FA", stroke: "#3AA0D8" },
  data:    { fill: "#AF52DE", stroke: "#8E3CB8" },
  doc:     { fill: "#34C759", stroke: "#28A745" },
  media:   { fill: "#FF9500", stroke: "#D47A00" },
  config:  { fill: "#FFD60A", stroke: "#D4B000" },
  archive: { fill: "#8E8E93", stroke: "#6E6E73" },
};

export async function renderGraphView(container) {
  if (currentSimulation) {
    currentSimulation.stop();
    currentSimulation = null;
  }

  var state = getState();
  var entries = state.entries.slice();

  if (entries.length === 0) {
    container.innerHTML =
      '<div class="empty-state">' +
        '<div class="empty-icon">\uD83C\uDF10</div>' +
        '<div class="empty-text">No items to graph</div>' +
      "</div>";
    return;
  }

  container.innerHTML =
    '<div class="graph-container">' +
      '<svg></svg>' +
      '<div class="graph-controls">' +
        '<button class="graph-ctrl-btn" data-action="zoom-in" title="Zoom in (+)">+</button>' +
        '<button class="graph-ctrl-btn" data-action="zoom-out" title="Zoom out (\u2212)">\u2212</button>' +
        '<button class="graph-ctrl-btn" data-action="fit" title="Fit to view (0)">Fit</button>' +
        '<button class="graph-ctrl-btn" data-action="reset" title="Reset layout (R)">Reset</button>' +
      '</div>' +
      '<div class="graph-tooltip"></div>' +
      '<div class="graph-hint">Scroll to zoom \u2022 Drag to pan \u2022 Click to select \u2022 Double-click folder to open</div>' +
    '</div>';

  var svgEl = container.querySelector("svg");
  var tooltip = container.querySelector(".graph-tooltip");

  var d3 = await loadD3();
  var svg = d3.select(svgEl);
  var width = container.clientWidth;
  var height = container.clientHeight;

  svg.attr("width", width).attr("height", height);

  // Build nodes and links.
  var parentNode = {
    id: state.currentPath,
    name: basename(state.currentPath),
    isDir: true,
    isRoot: true,
    size: 0,
    extension: "",
    colorIdx: -1,
  };
  var nodes = [parentNode];
  var links = [];
  var dirIndex = 0;

  entries.forEach(function (entry) {
    var colorIdx = -1;
    if (entry.isDir) {
      colorIdx = dirIndex % folderPalette.length;
      dirIndex++;
    }
    nodes.push({
      id: entry.path,
      name: entry.name,
      isDir: entry.isDir,
      size: entry.size,
      extension: entry.extension || "",
      colorIdx: colorIdx,
    });
    links.push({ source: state.currentPath, target: entry.path });
  });

  var nodeCount = nodes.length;
  var showLabels = nodeCount <= 80;
  var showExtLabels = nodeCount <= 200;

  // --- SVG Defs: gradients, shadows, blur ---
  var defs = svg.append("defs");

  // Drop shadow filter for nodes.
  var shadow = defs.append("filter")
    .attr("id", "node-shadow")
    .attr("x", "-50%").attr("y", "-50%")
    .attr("width", "200%").attr("height", "200%");
  shadow.append("feDropShadow")
    .attr("dx", 0).attr("dy", 1)
    .attr("stdDeviation", 3)
    .attr("flood-color", "rgba(0,0,0,0.18)");

  // Glow filter for root node.
  var glow = defs.append("filter")
    .attr("id", "root-glow")
    .attr("x", "-100%").attr("y", "-100%")
    .attr("width", "300%").attr("height", "300%");
  glow.append("feGaussianBlur")
    .attr("in", "SourceGraphic")
    .attr("stdDeviation", 6)
    .attr("result", "blur");
  var glowMerge = glow.append("feMerge");
  glowMerge.append("feMergeNode").attr("in", "blur");
  glowMerge.append("feMergeNode").attr("in", "SourceGraphic");

  // Radial gradients for folder palette colors.
  folderPalette.forEach(function (c, i) {
    var grad = defs.append("radialGradient")
      .attr("id", "grad-folder-" + i)
      .attr("cx", "35%").attr("cy", "35%").attr("r", "65%");
    grad.append("stop").attr("offset", "0%").attr("stop-color", lighten(c.fill, 0.3));
    grad.append("stop").attr("offset", "100%").attr("stop-color", c.fill);
  });

  // Radial gradients for file types.
  Object.keys(fileTypeColors).forEach(function (type) {
    var c = fileTypeColors[type];
    var grad = defs.append("radialGradient")
      .attr("id", "grad-file-" + type)
      .attr("cx", "35%").attr("cy", "35%").attr("r", "65%");
    grad.append("stop").attr("offset", "0%").attr("stop-color", lighten(c.fill, 0.3));
    grad.append("stop").attr("offset", "100%").attr("stop-color", c.fill);
  });

  // Root node gradient.
  var rootGrad = defs.append("radialGradient")
    .attr("id", "grad-root")
    .attr("cx", "35%").attr("cy", "35%").attr("r", "65%");
  rootGrad.append("stop").attr("offset", "0%").attr("stop-color", "#FFD8A8");
  rootGrad.append("stop").attr("offset", "100%").attr("stop-color", "#D4A574");

  // --- Zoom ---
  var zoomBehaviour = d3.zoom()
    .scaleExtent([0.1, 8])
    .on("zoom", function (event) {
      worldGroup.attr("transform", event.transform);
    });
  svg.call(zoomBehaviour);
  svg.on("dblclick.zoom", null);

  var worldGroup = svg.append("g").attr("class", "graph-world");

  // Force layout parameters.
  var linkDist = Math.max(80, Math.min(200, 1400 / Math.sqrt(nodeCount)));
  var chargeStr = Math.min(-120, Math.max(-600, -18000 / Math.sqrt(nodeCount)));
  var collisionPadding = nodeCount > 100 ? 14 : nodeCount > 50 ? 10 : 8;

  var simulation = d3.forceSimulation(nodes)
    .force("link", d3.forceLink(links).id(function (d) { return d.id; }).distance(linkDist))
    .force("charge", d3.forceManyBody().strength(chargeStr))
    .force("center", d3.forceCenter(width / 2, height / 2))
    .force("collision", d3.forceCollide().radius(function (d) {
      return nodeRadius(d) + collisionPadding;
    }))
    .force("x", d3.forceX(width / 2).strength(0.03))
    .force("y", d3.forceY(height / 2).strength(0.03));

  currentSimulation = simulation;

  // --- Links: curved paths ---
  var link = worldGroup.append("g").attr("class", "graph-links")
    .selectAll("path")
    .data(links)
    .join("path")
    .attr("class", "graph-link");

  // --- Nodes ---
  var node = worldGroup.append("g").attr("class", "graph-nodes")
    .selectAll("g")
    .data(nodes)
    .join("g")
    .attr("class", function (d) {
      var cls = "graph-node";
      if (d.isRoot) return cls + " root";
      if (d.isDir) return cls + " dir";
      return cls + " file";
    })
    .call(d3.drag()
      .on("start", dragstarted)
      .on("drag", dragged)
      .on("end", dragended));

  // Node circles with gradients and shadows.
  node.append("circle")
    .attr("r", function (d) { return nodeRadius(d); })
    .attr("fill", function (d) {
      if (d.isRoot) return "url(#grad-root)";
      if (d.isDir && d.colorIdx >= 0) return "url(#grad-folder-" + d.colorIdx + ")";
      // File nodes — use type gradient.
      var fileType = getFileType(d.extension);
      if (fileType && fileTypeColors[fileType]) return "url(#grad-file-" + fileType + ")";
      return "url(#grad-file-doc)";
    })
    .attr("stroke", function (d) {
      if (d.isRoot) return "#D4A574";
      if (d.isDir && d.colorIdx >= 0) return folderPalette[d.colorIdx].stroke;
      var fileType = getFileType(d.extension);
      if (fileType && fileTypeColors[fileType]) return fileTypeColors[fileType].stroke;
      return "#6E6E73";
    })
    .attr("stroke-width", function (d) {
      return d.isRoot ? 2.5 : 1.5;
    })
    .attr("filter", function (d) {
      return d.isRoot ? "url(#root-glow)" : "url(#node-shadow)";
    });

  // Extension labels in file nodes.
  if (showExtLabels) {
    node.filter(function (d) { return !d.isDir && d.extension; })
      .append("text")
      .attr("class", "graph-node-ext")
      .text(function (d) { return d.extension.substring(0, 3); })
      .attr("text-anchor", "middle")
      .attr("dy", "0.35em");
  }

  // Name labels.
  if (showLabels) {
    node.append("text")
      .attr("class", "graph-node-label")
      .text(function (d) { return truncateName(d.name, 24); })
      .attr("x", function (d) { return nodeRadius(d) + 8; })
      .attr("y", 4);
  }

  // --- Interactions ---
  node.on("click", function (ev, d) {
    ev.stopPropagation();
    highlightSelection(node, link, d);
    if (!d.isDir) selectFile(d.id);
  });

  node.on("dblclick", function (ev, d) {
    ev.stopPropagation();
    if (d.isDir) navigateTo(d.id);
    else selectFile(d.id);
  });

  node.on("mouseenter", function (ev, d) {
    var info = d.name;
    if (!d.isDir && d.size > 0) info += " \u2022 " + formatBytes(d.size);
    if (d.isDir) info += " \u2022 Folder";
    if (d.isRoot) info += " (current)";
    tooltip.textContent = info;
    tooltip.classList.add("visible");
  });

  node.on("mousemove", function (ev) {
    tooltip.style.left = (ev.clientX + 14) + "px";
    tooltip.style.top = (ev.clientY - 10) + "px";
  });

  node.on("mouseleave", function () {
    tooltip.classList.remove("visible");
  });

  svg.on("click", function () {
    node.classed("selected", false).classed("dimmed", false);
    link.classed("highlighted", false).classed("dimmed", false);
  });

  // --- Tick: update positions with curved links ---
  simulation.on("tick", function () {
    link.attr("d", function (d) {
      var dx = d.target.x - d.source.x;
      var dy = d.target.y - d.source.y;
      var dr = Math.sqrt(dx * dx + dy * dy) * 1.5;
      return "M" + d.source.x + "," + d.source.y +
        "A" + dr + "," + dr + " 0 0,1 " + d.target.x + "," + d.target.y;
    });

    node.attr("transform", function (d) {
      return "translate(" + d.x + "," + d.y + ")";
    });
  });

  simulation.on("end", function () {
    fitToView(svg, worldGroup, nodes, width, height, zoomBehaviour);
  });

  // --- Controls ---
  container.querySelectorAll(".graph-ctrl-btn").forEach(function (btn) {
    btn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      switch (btn.dataset.action) {
        case "zoom-in":
          svg.transition().duration(300).call(zoomBehaviour.scaleBy, 1.4);
          break;
        case "zoom-out":
          svg.transition().duration(300).call(zoomBehaviour.scaleBy, 0.7);
          break;
        case "fit":
          fitToView(svg, worldGroup, nodes, width, height, zoomBehaviour);
          break;
        case "reset":
          renderGraphView(container);
          break;
      }
    });
  });

  svgEl.tabIndex = 0;
  svgEl.addEventListener("keydown", function (ev) {
    if (ev.key === "+" || ev.key === "=") {
      ev.preventDefault();
      svg.transition().duration(200).call(zoomBehaviour.scaleBy, 1.3);
    } else if (ev.key === "-") {
      ev.preventDefault();
      svg.transition().duration(200).call(zoomBehaviour.scaleBy, 0.75);
    } else if (ev.key === "0") {
      ev.preventDefault();
      fitToView(svg, worldGroup, nodes, width, height, zoomBehaviour);
    } else if (ev.key === "r") {
      ev.preventDefault();
      renderGraphView(container);
    }
  });

  function dragstarted(event) {
    if (!event.active) simulation.alphaTarget(0.3).restart();
    event.subject.fx = event.subject.x;
    event.subject.fy = event.subject.y;
  }

  function dragged(event) {
    event.subject.fx = event.x;
    event.subject.fy = event.y;
  }

  function dragended(event) {
    if (!event.active) simulation.alphaTarget(0);
    event.subject.fx = null;
    event.subject.fy = null;
  }
}

/**
 * Highlight a selected node and its connections.
 */
function highlightSelection(nodeSelection, linkSelection, selected) {
  var connectedIds = new Set();
  connectedIds.add(selected.id);

  linkSelection.each(function (d) {
    var srcId = typeof d.source === "object" ? d.source.id : d.source;
    var tgtId = typeof d.target === "object" ? d.target.id : d.target;
    if (srcId === selected.id) connectedIds.add(tgtId);
    if (tgtId === selected.id) connectedIds.add(srcId);
  });

  nodeSelection
    .classed("selected", function (d) { return d.id === selected.id; })
    .classed("dimmed", function (d) { return !connectedIds.has(d.id); });

  linkSelection
    .classed("highlighted", function (d) {
      var srcId = typeof d.source === "object" ? d.source.id : d.source;
      var tgtId = typeof d.target === "object" ? d.target.id : d.target;
      return srcId === selected.id || tgtId === selected.id;
    })
    .classed("dimmed", function (d) {
      var srcId = typeof d.source === "object" ? d.source.id : d.source;
      var tgtId = typeof d.target === "object" ? d.target.id : d.target;
      return srcId !== selected.id && tgtId !== selected.id;
    });
}

/**
 * Fit all nodes into the viewport with padding.
 */
function fitToView(svg, worldGroup, nodes, width, height, zoomBehaviour) {
  if (nodes.length === 0) return;

  var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  nodes.forEach(function (d) {
    if (d.x < minX) minX = d.x;
    if (d.y < minY) minY = d.y;
    if (d.x > maxX) maxX = d.x;
    if (d.y > maxY) maxY = d.y;
  });

  var padding = 100;
  var graphWidth = maxX - minX + padding * 2;
  var graphHeight = maxY - minY + padding * 2;

  if (graphWidth <= 0 || graphHeight <= 0) return;

  var scale = Math.min(width / graphWidth, height / graphHeight, 2);
  var cx = (minX + maxX) / 2;
  var cy = (minY + maxY) / 2;
  var tx = width / 2 - cx * scale;
  var ty = height / 2 - cy * scale;

  svg.transition().duration(600).ease(d3.easeCubicOut)
    .call(zoomBehaviour.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
}

/**
 * Get file type key from extension.
 */
function getFileType(ext) {
  if (!ext) return "doc";
  ext = ext.toLowerCase();
  var colorCls = getFileColorClass({ isDir: false, extension: ext });
  return colorCls.replace("ftype-", "");
}

function nodeRadius(d) {
  if (d.isRoot) return 22;
  if (d.isDir) return 14;
  return 9;
}

/**
 * Lighten a hex color by a factor (0-1).
 */
function lighten(hex, factor) {
  var r = parseInt(hex.slice(1, 3), 16);
  var g = parseInt(hex.slice(3, 5), 16);
  var b = parseInt(hex.slice(5, 7), 16);
  r = Math.min(255, Math.round(r + (255 - r) * factor));
  g = Math.min(255, Math.round(g + (255 - g) * factor));
  b = Math.min(255, Math.round(b + (255 - b) * factor));
  return "#" + ((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1);
}

function basename(path) {
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

function truncateName(name, max) {
  if (name.length <= max) return name;
  return name.substring(0, max - 3) + "\u2026";
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  var units = ["B", "KB", "MB", "GB"];
  var i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + " " + units[i];
}
