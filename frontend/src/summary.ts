import type { CleanResult, RiskLevel, ScanItem, ScanResult } from './models';

export type RiskCounts = Record<RiskLevel, number>;
export type ResultOutcome = 'empty' | 'success' | 'partial' | 'failed';

export interface SelectionSummary {
  items: ScanItem[];
  cleanableItems: ScanItem[];
  pluginItems: ScanItem[];
  registryItems: ScanItem[];
  size: number;
  pluginSize: number;
  riskCounts: RiskCounts;
  hasMediumRisk: boolean;
  hasHighRisk: boolean;
  hasPlugins: boolean;
  hasRegistry: boolean;
}

export interface FailureSummary {
  scan: number;
  clean: number;
  total: number;
}

const emptyRiskCounts = (): RiskCounts => ({
  low: 0,
  medium: 0,
  high: 0,
});

export function summarizeSelection(items: ScanItem[]): SelectionSummary {
  const selected = items.filter((item) => item.selected);
  const cleanableItems = selected.filter((item) => item.type === 'file');
  const pluginItems = selected.filter((item) => item.type === 'plugin');
  const registryItems = selected.filter((item) => item.type === 'registry');
  const riskCounts = emptyRiskCounts();
  const size = selected.reduce((total, item) => {
    riskCounts[item.risk] += 1;
    if (item.type === 'plugin') {
      return total;
    }
    return total + item.size;
  }, 0);
  const pluginSize = pluginItems.reduce((total, item) => total + item.size, 0);

  return {
    items: selected,
    cleanableItems,
    pluginItems,
    registryItems,
    size,
    pluginSize,
    riskCounts,
    hasMediumRisk: riskCounts.medium > 0,
    hasHighRisk: riskCounts.high > 0,
    hasPlugins: pluginItems.length > 0,
    hasRegistry: registryItems.length > 0,
  };
}

export function countFailures(
  scanResult: ScanResult | null,
  cleanResult: CleanResult | null,
): FailureSummary {
  const scan = scanResult?.errors?.length || 0;
  const clean = cleanResult?.failed_files?.length || 0;
  return {
    scan,
    clean,
    total: scan + clean,
  };
}

export function reconcileItemsAfterClean(
  currentItems: ScanItem[],
  selectedItems: ScanItem[],
  cleanResult: CleanResult,
): ScanItem[] {
  const failedPaths = new Set(cleanResult.failed_files || []);
  const selectedPaths = new Set(selectedItems.map((item) => item.path));

  return currentItems
    .filter((item) => !(selectedPaths.has(item.path) && !failedPaths.has(item.path)))
    .map((item) => (
      failedPaths.has(item.path) ? { ...item, selected: false } : item
    ));
}

export function describeCleanOutcome(cleanResult: CleanResult | null): ResultOutcome | null {
  if (!cleanResult) {
    return null;
  }

  const successCount = cleanResult.deleted_files || 0;
  const failureCount = cleanResult.failed_files?.length || 0;
  return describeResultOutcome(successCount, failureCount);
}

export function describeResultOutcome(successCount: number, failureCount: number): ResultOutcome {
  if (successCount > 0 && failureCount > 0) {
    return 'partial';
  }
  if (successCount > 0) {
    return 'success';
  }
  if (failureCount > 0) {
    return 'failed';
  }
  return 'empty';
}

export function hasPermissionFailure(reasons: string[]): boolean {
  return reasons.some((reason) => {
    const value = reason.toLowerCase();
    return value.includes('权限不足')
      || value.includes('permission denied')
      || value.includes('access is denied');
  });
}
