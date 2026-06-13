import {
  countFailures,
  reconcileItemsAfterClean,
  summarizeSelection,
} from '../src/summary';
import type { CleanResult, ScanItem, ScanResult } from '../src/models';

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(message);
  }
}

function item(id: string, selected: boolean, risk: ScanItem['risk'], size: number, path = id): ScanItem {
  return {
    id,
    path,
    name: `${id}.tmp`,
    type: 'file',
    category: 'system',
    size,
    risk,
    source: 'test rule',
    last_modified: 0,
    selected,
  };
}

function pluginItem(id: string, selected: boolean, size: number): ScanItem {
  return {
    id,
    path: `C:/Users/test/AppData/Local/Browser/User Data/Default/Extensions/${id}`,
    name: 'Test Plugin',
    type: 'plugin',
    category: 'plugin',
    size,
    risk: 'medium',
    source: 'Chrome 插件',
    last_modified: 0,
    selected,
    plugin: {
      browser: 'Chrome',
      profile: 'Default',
      extension_id: id,
      version: '1.0.0',
      description: 'fixture',
      manifest_path: `C:/manifest/${id}/manifest.json`,
    },
  };
}

const scanItems = [
  item('low', true, 'low', 10, 'C:/tmp/low.tmp'),
  item('medium', true, 'medium', 20, 'C:/tmp/medium.tmp'),
  item('high', false, 'high', 30, 'C:/tmp/high.tmp'),
  item('ignored', false, 'low', 40, 'C:/tmp/ignored.tmp'),
];

const selection = summarizeSelection(scanItems);
assert(selection.items.length === 2, 'selected item count should include only checked items');
assert(selection.size === 30, 'selected size should sum checked items only');
assert(selection.riskCounts.low === 1, 'low risk selected count should be 1');
assert(selection.riskCounts.medium === 1, 'medium risk selected count should be 1');
assert(selection.riskCounts.high === 0, 'high risk selected count should be 0');
assert(selection.hasHighRisk === false, 'high risk flag should be false without selected high-risk items');

const pluginSelection = summarizeSelection([
  item('file-selected', true, 'low', 10, 'C:/tmp/file.tmp'),
  pluginItem('plugin-selected', true, 500),
]);
assert(pluginSelection.items.length === 2, 'selected plugin should still be part of selected items');
assert(pluginSelection.size === 10, 'selected plugin size should not count as releasable clean size');

const scanResult: ScanResult = {
  items: scanItems,
  total_files: 4,
  total_size: 100,
  errors: [
    { path: 'C:/Windows/Logs', reason: 'permission denied' },
    { path: 'C:/Windows/Temp', reason: 'file locked' },
  ],
  duration_ms: 12,
};
const cleanResult: CleanResult = {
  deleted_files: 1,
  freed_size: 10,
  failed_files: ['C:/tmp/medium.tmp'],
  failed_reasons: {
    'C:/tmp/medium.tmp': 'file locked',
  },
  message: 'done',
};

const failures = countFailures(scanResult, cleanResult);
assert(failures.scan === 2, 'scan failure count should come from scan errors');
assert(failures.clean === 1, 'clean failure count should come from clean failed files');
assert(failures.total === 3, 'total failure count should include scan and clean failures');

const reconciled = reconcileItemsAfterClean(scanItems, selection.items, cleanResult);
assert(reconciled.length === 3, 'successfully deleted selected item should be removed');
assert(!reconciled.some((entry) => entry.path === 'C:/tmp/low.tmp'), 'deleted low-risk item should be gone');
const failed = reconciled.find((entry) => entry.path === 'C:/tmp/medium.tmp');
assert(!!failed, 'failed item should remain visible');
assert(failed?.selected === false, 'failed item should be unselected after clean');
