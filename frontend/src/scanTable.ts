import type { RiskLevel, ScanItem } from './models';

export type RiskFilter = 'all' | RiskLevel;

export interface ScanRow {
  item: ScanItem;
  index: number;
  key: string;
}

export interface PaginatedScanRows {
  rows: ScanRow[];
  page: number;
  pageSize: number;
  totalPages: number;
  totalRows: number;
  from: number;
  to: number;
}

export function makeScanRowKey(item: ScanItem, index: number): string {
  const stablePart = item.id || item.path || item.name || 'scan-row';
  return `${stablePart}::${index}`;
}

export function makeScanRows(items: ScanItem[]): ScanRow[] {
  return items.map((item, index) => ({
    item,
    index,
    key: makeScanRowKey(item, index),
  }));
}

export function filterScanRows(
  items: ScanItem[],
  selectedCategory: string,
  selectedRisk: RiskFilter,
): ScanRow[] {
  return makeScanRows(items).filter(({ item }) => {
    const categoryMatch = selectedCategory === 'all' || item.category === selectedCategory;
    const riskMatch = selectedRisk === 'all' || item.risk === selectedRisk;
    return categoryMatch && riskMatch;
  });
}

export function paginateScanRows(
  rows: ScanRow[],
  requestedPage: number,
  requestedPageSize: number,
): PaginatedScanRows {
  const pageSize = Number.isFinite(requestedPageSize) && requestedPageSize > 0
    ? Math.floor(requestedPageSize)
    : 100;
  const totalRows = rows.length;
  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const page = clampPage(requestedPage, totalPages);
  const start = (page - 1) * pageSize;
  const pageRows = rows.slice(start, start + pageSize);

  return {
    rows: pageRows,
    page,
    pageSize,
    totalPages,
    totalRows,
    from: totalRows === 0 ? 0 : start + 1,
    to: start + pageRows.length,
  };
}

export function clampPage(requestedPage: number, totalPages: number): number {
  const maxPage = Math.max(1, totalPages);
  if (!Number.isFinite(requestedPage)) {
    return 1;
  }
  return Math.max(1, Math.min(maxPage, Math.floor(requestedPage)));
}

export function updateItemSelectionAtIndex(
  items: ScanItem[],
  index: number,
  selected: boolean,
): ScanItem[] {
  if (!Number.isInteger(index) || index < 0 || index >= items.length) {
    return items;
  }
  return items.map((item, itemIndex) => (
    itemIndex === index ? { ...item, selected } : item
  ));
}

export function updateRowsSelection(
  items: ScanItem[],
  rows: ScanRow[],
  selected: boolean,
  includeHighRisk = false,
): ScanItem[] {
  const rowIndexes = new Set(rows.map((row) => row.index));
  return items.map((item, index) => {
    if (!rowIndexes.has(index)) {
      return item;
    }
    if (selected && !includeHighRisk && item.risk === 'high') {
      return item;
    }
    return { ...item, selected };
  });
}

export function countSelectableRows(rows: ScanRow[], includeHighRisk = false): number {
  return rows.filter(({ item }) => includeHighRisk || item.risk !== 'high').length;
}
