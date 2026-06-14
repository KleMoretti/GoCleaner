const fs = require('fs');
const path = require('path');

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

const css = fs.readFileSync(path.join(__dirname, '..', 'src', 'App.css'), 'utf8');

function block(selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = css.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`, 'm'));
  return match ? match[1] : '';
}

const scanTable = block('.scan-table');
assert(scanTable.includes('table-layout: fixed'), 'scan result table should use fixed layout to keep columns stable');

const pathCell = block('.path-cell');
assert(pathCell.includes('white-space: nowrap'), 'path cells should not wrap long paths into tall rows');
assert(pathCell.includes('text-overflow: ellipsis'), 'path cells should truncate long paths with ellipsis');

const categoryCell = block('.category-cell');
assert(categoryCell.includes('white-space: nowrap'), 'category cells should stay on one line');

const scanMeta = block('.scan-table .item-meta');
assert(scanMeta.includes('text-overflow: ellipsis'), 'scan table metadata should truncate instead of wrapping into tall rows');
