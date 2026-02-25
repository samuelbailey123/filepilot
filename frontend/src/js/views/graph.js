// Graph View — D3.js force-directed graph scoped to current directory.

import { getState, selectFile, navigateTo } from "../app.js";

export function renderGraphView(container) {
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

  container.innerHTML = '<div class="graph-container"><svg></svg></div>';

  var svg = d3.select(container.querySelector("svg"));
  var width = container.clientWidth;
  var height = container.clientHeight;

  svg.attr("viewBox", [0, 0, width, height]);

  // Build nodes and links.
  var parentNode = { id: state.currentPath, name: basename(state.currentPath), isDir: true, isRoot: true };
  var nodes = [parentNode];
  var links = [];

  entries.forEach(function (entry) {
    nodes.push({ id: entry.path, name: entry.name, isDir: entry.isDir });
    links.push({ source: state.currentPath, target: entry.path });
  });

  // Force simulation.
  var simulation = d3.forceSimulation(nodes)
    .force("link", d3.forceLink(links).id(function (d) { return d.id; }).distance(60))
    .force("charge", d3.forceManyBody().strength(-120))
    .force("center", d3.forceCenter(width / 2, height / 2))
    .force("collision", d3.forceCollide().radius(20));

  // Links.
  var link = svg.append("g")
    .selectAll("line")
    .data(links)
    .join("line")
    .attr("class", "graph-link");

  // Nodes.
  var node = svg.append("g")
    .selectAll("g")
    .data(nodes)
    .join("g")
    .attr("class", function (d) { return "graph-node " + (d.isDir ? "dir" : "file"); })
    .call(d3.drag()
      .on("start", dragstarted)
      .on("drag", dragged)
      .on("end", dragended));

  node.append("circle")
    .attr("r", function (d) { return d.isRoot ? 14 : d.isDir ? 10 : 6; });

  node.append("text")
    .text(function (d) { return truncateName(d.name, 20); })
    .attr("x", 14)
    .attr("y", 4);

  // Click handler.
  node.on("click", function (ev, d) {
    if (d.isDir) {
      navigateTo(d.id);
    } else {
      selectFile(d.id);
    }
  });

  // Tick function.
  simulation.on("tick", function () {
    link
      .attr("x1", function (d) { return d.source.x; })
      .attr("y1", function (d) { return d.source.y; })
      .attr("x2", function (d) { return d.target.x; })
      .attr("y2", function (d) { return d.target.y; });

    node.attr("transform", function (d) {
      return "translate(" + d.x + "," + d.y + ")";
    });
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

function basename(path) {
  var parts = path.split("/");
  return parts[parts.length - 1] || "/";
}

function truncateName(name, max) {
  if (name.length <= max) return name;
  return name.substring(0, max - 3) + "...";
}
