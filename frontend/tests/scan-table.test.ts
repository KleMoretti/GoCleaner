import {
  filterScanRows,
  makeScanRowKey,
  paginateScanRows,
  updateItemSelectionAtIndex,
  updateRowsSelection,
} from '../src/scanTable';
import type { ScanItem } from '../src/models';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(message);
  }
}

function item(
  id: string,
  category: string,
  risk: ScanItem['risk'],
  selected = false,
  type: ScanItem['type'] = 'file',
): ScanItem {
  return {
    id,
    path: `C:/fixture/${category}/${id}.tmp`,
    name: `${id}.tmp`,
    type,
    category,
    size: 10,
    risk,
    source: 'test rule',
    last_modified: 0,
    selected,
  };
}

const duplicateItems = [
  item('same-id', 'system', 'low', true),
  item('same-id', 'system', 'low', true),
];

const duplicateKeys = duplicateItems.map((entry, index) => makeScanRowKey(entry, index));
assert(duplicateKeys[0] !== duplicateKeys[1], 'row keys must remain unique when backend ids are duplicated');

const updatedDuplicate = updateItemSelectionAtIndex(duplicateItems, 1, false);
assert(updatedDuplicate[0].selected === true, 'updating a duplicate id row must not affect earlier rows');
assert(updatedDuplicate[1].selected === false, 'target duplicate id row should be updated by row index');
assert(duplicateItems[1].selected === true, 'row selection update should not mutate the original array');

const mixedItems = [
  item('system-low', 'system', 'low'),
  item('system-high', 'system', 'high'),
  item('plugin-medium', 'plugin', 'medium', false, 'plugin'),
  item('software-low', 'software', 'low'),
];

const systemRows = filterScanRows(mixedItems, 'system', 'all');
const selectedSystemRows = updateRowsSelection(mixedItems, systemRows, true);
assert(selectedSystemRows[0].selected === true, 'filtered select all should select low-risk rows');
assert(selectedSystemRows[1].selected === false, 'filtered select all should leave high-risk rows unselected');
assert(selectedSystemRows[2].selected === false, 'filtered select all should not affect other categories');

const pluginRows = filterScanRows(mixedItems, 'plugin', 'all');
const selectedPluginRows = updateRowsSelection(mixedItems, pluginRows, true);
assert(selectedPluginRows[2].selected === true, 'filtered select all should include plugin scan rows');

const paged = paginateScanRows(filterScanRows(mixedItems, 'all', 'all'), 2, 2);
assert(paged.page === 2, 'pagination should keep an in-range requested page');
assert(paged.totalPages === 2, 'pagination should compute total pages from filtered rows');
assert(paged.rows.length === 2, 'pagination should return only the requested page rows');
assert(paged.rows[0].index === 2, 'pagination should preserve original item indexes');

const clamped = paginateScanRows(filterScanRows(mixedItems, 'all', 'all'), 99, 3);
assert(clamped.page === 2, 'pagination should clamp page numbers above the total page count');
assert(clamped.rows.length === 1, 'clamped pagination should return the final page rows');
