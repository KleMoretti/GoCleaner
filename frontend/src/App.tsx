import { useEffect, useMemo, useState } from 'react';
import './App.css';
import {
  Clean,
  DeleteRegistryItems,
  GetEnvInfo,
  GetOperationLogs,
  GetRulesPreview,
  GetRulesWarnings,
  Ping,
  QuarantinePlugins,
  Scan,
  ScanInvalidStartupRegistry,
  SelectShredFile,
  ShredFile,
} from '../wailsjs/go/app/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import {
  CategoryLabels,
  RiskColors,
  RiskLabels,
} from './models';
import {
  countFailures,
  describeCleanOutcome,
  describeResultOutcome,
  hasPermissionFailure,
  reconcileItemsAfterClean,
  summarizeSelection,
} from './summary';
import {
  countSelectableRows,
  filterScanRows,
  paginateScanRows,
  updateItemSelectionAtIndex,
  updateRowsSelection,
} from './scanTable';
import type {
  CleanResult,
  CleanRule,
  OperationLog,
  QuarantineResult,
  RegistryActionResult,
  RiskLevel,
  ScanItem,
  ScanProgress,
  ScanResult,
  ShredResult,
} from './models';
import type { RiskFilter } from './scanTable';

type ConfirmVariant = 'default' | 'warning' | 'danger';

interface ConfirmDialogState {
  title: string;
  body: string[];
  confirmLabel: string;
  cancelLabel: string;
  variant: ConfirmVariant;
  resolve: (confirmed: boolean) => void;
}

const riskOrder: RiskLevel[] = ['low', 'medium', 'high'];
const pageSizeOptions = [50, 100, 200];

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0 B';
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatDateFromSeconds(seconds: number): string {
  if (!seconds) {
    return '-';
  }
  const date = new Date(seconds * 1000);
  if (Number.isNaN(date.getTime())) {
    return '-';
  }
  return date.toLocaleString();
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || '-';
  }
  return date.toLocaleString();
}

function categoryLabel(category: string): string {
  return CategoryLabels[category] || category || '-';
}

function riskLabel(risk: string): string {
  return RiskLabels[risk as RiskLevel] || risk || '-';
}

function operationLabel(operation: string): string {
  const labels: Record<string, string> = {
    scan: '扫描',
    clean: '清理',
    shred: '粉碎',
    registry_backup: '注册表备份',
    registry_delete: '注册表删除',
    quarantine: '隔离',
    restore: '恢复',
  };
  return labels[operation] || operation || '-';
}

function scanPhaseLabel(phase: ScanProgress['phase']): string {
  const labels: Record<ScanProgress['phase'], string> = {
    loading_rules: '加载规则',
    scanning_files: '扫描文件',
    scanning_plugins: '扫描插件',
    scanning_registry: '扫描注册表',
    done: '完成',
  };
  return labels[phase] || phase;
}

function outcomeLabel(outcome: ReturnType<typeof describeCleanOutcome>): string {
  switch (outcome) {
    case 'success':
      return '全部成功';
    case 'partial':
      return '部分成功';
    case 'failed':
      return '全部失败';
    case 'empty':
      return '未处理任何项目';
    default:
      return '';
  }
}

function resultPanelClass(outcome: ReturnType<typeof describeCleanOutcome>): string {
  return `result-panel ${outcome ? `result-${outcome}` : ''}`;
}

function WarningList({ warnings }: { warnings?: string[] }) {
  const visibleWarnings = (warnings || []).filter(Boolean);
  if (visibleWarnings.length === 0) {
    return null;
  }
  return (
    <ul className="result-warning-list">
      {visibleWarnings.map((warning) => (
        <li key={warning}>{warning}</li>
      ))}
    </ul>
  );
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, Math.round(value)));
}

function errorMessage(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  if (message.includes("reading 'app'") || message.includes('window.go')) {
    return '未检测到 Wails 后端绑定，请在 Wails 桌面运行环境中打开应用。';
  }
  return message;
}

function App() {
  const [backendStatus, setBackendStatus] = useState('检查中...');
  const [rules, setRules] = useState<CleanRule[]>([]);
  const [ruleWarnings, setRuleWarnings] = useState<string[]>([]);
  const [envInfo, setEnvInfo] = useState<Record<string, string>>({});
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [cleanResult, setCleanResult] = useState<CleanResult | null>(null);
  const [quarantineResult, setQuarantineResult] = useState<QuarantineResult | null>(null);
  const [registryResult, setRegistryResult] = useState<RegistryActionResult | null>(null);
  const [shredResult, setShredResult] = useState<ShredResult | null>(null);
  const [scanProgress, setScanProgress] = useState<ScanProgress | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);
  const [operationLogs, setOperationLogs] = useState<OperationLog[]>([]);
  const [shredPath, setShredPath] = useState('');
  const [shredPasses, setShredPasses] = useState(1);
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [selectedRisk, setSelectedRisk] = useState<RiskFilter>('all');
  const [resultsPage, setResultsPage] = useState(1);
  const [resultsPageSize, setResultsPageSize] = useState(100);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadInitialData();
  }, []);

  useEffect(() => {
    setResultsPage(1);
  }, [selectedCategory, selectedRisk, resultsPageSize, scanResult?.items.length]);

  useEffect(() => {
    const runtime = (window as unknown as { runtime?: { EventsOn?: unknown } }).runtime;
    if (!runtime?.EventsOn) {
      return;
    }
    const off = EventsOn('gocleaner:scan-progress', (progress: ScanProgress) => {
      setScanProgress(progress);
    });
    return () => {
      if (typeof off === 'function') {
        off();
      }
    };
  }, []);

  useEffect(() => {
    if (!confirmDialog) {
      return;
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        closeConfirmDialog(false);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [confirmDialog]);

  async function loadOperationLogs() {
    const logs = await GetOperationLogs(30);
    setOperationLogs((logs || []) as unknown as OperationLog[]);
  }

  async function loadInitialData() {
    try {
      setLoading(true);
      await Ping();
      setBackendStatus('后端已连接');

      const [rulesList, warnings, env] = await Promise.all([
        GetRulesPreview(),
        GetRulesWarnings(),
        GetEnvInfo(),
      ]);

      setRules((rulesList || []) as unknown as CleanRule[]);
      setRuleWarnings(warnings || []);
      setEnvInfo(env || {});
      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
      setBackendStatus('后端不可用');
    } finally {
      setLoading(false);
    }
  }

  async function runScan() {
    try {
      setScanning(true);
      setScanProgress({
        phase: 'loading_rules',
        current_label: '准备扫描',
        completed_steps: 0,
        total_steps: rules.length + 1,
        found_items: 0,
        failed_items: 0,
        percent: 0,
      });
      setCleanResult(null);
      setQuarantineResult(null);
      setRegistryResult(null);
      setShredResult(null);
      const result = await Scan();
      setScanResult(result as unknown as ScanResult);
      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
      setScanProgress(null);
    } finally {
      setScanning(false);
    }
  }

  function setItemSelected(index: number, selected: boolean) {
    setScanResult((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        items: updateItemSelectionAtIndex(current.items, index, selected),
      };
    });
  }

  const scanItems = scanResult?.items || [];

  const categories = useMemo(() => {
    const source = scanItems.length > 0 ? scanItems.map((item) => item.category) : rules.map((rule) => rule.category);
    return ['all', ...Array.from(new Set(source))];
  }, [rules, scanItems]);

  const filteredRows = useMemo(
    () => filterScanRows(scanItems, selectedCategory, selectedRisk),
    [scanItems, selectedCategory, selectedRisk],
  );
  const pagedRows = useMemo(
    () => paginateScanRows(filteredRows, resultsPage, resultsPageSize),
    [filteredRows, resultsPage, resultsPageSize],
  );
  const selectableFilteredCount = useMemo(
    () => countSelectableRows(filteredRows),
    [filteredRows],
  );
  const selectedFilteredCount = useMemo(
    () => filteredRows.filter((row) => row.item.risk !== 'high' && row.item.selected).length,
    [filteredRows],
  );
  const selectedFilteredAnyCount = useMemo(
    () => filteredRows.filter((row) => row.item.selected).length,
    [filteredRows],
  );

  const selectionSummary = useMemo(() => summarizeSelection(scanItems), [scanItems]);
  const selectedItems = selectionSummary.items;
  const selectedCleanItems = selectionSummary.cleanableItems;
  const selectedPluginItems = selectionSummary.pluginItems;
  const selectedRegistryItems = selectionSummary.registryItems;
  const selectedSize = selectionSummary.size;
  const selectedPluginSize = selectionSummary.pluginSize;
  const selectedRiskCounts = selectionSummary.riskCounts;
  const hasHighRiskSelection = selectedCleanItems.some((item) => item.risk === 'high');
  const failureSummary = useMemo(
    () => countFailures(scanResult, cleanResult),
    [scanResult, cleanResult],
  );
  const cleanOutcome = useMemo(() => describeCleanOutcome(cleanResult), [cleanResult]);
  const quarantineOutcome = useMemo(() => (
    quarantineResult
      ? describeResultOutcome(
        quarantineResult.moved_items + quarantineResult.restored_items,
        quarantineResult.failed_items.length,
      )
      : null
  ), [quarantineResult]);
  const registryOutcome = useMemo(() => (
    registryResult
      ? describeResultOutcome(registryResult.deleted_values, registryResult.failed_items.length)
      : null
  ), [registryResult]);
  const shredOutcome = useMemo(() => (
    shredResult
      ? describeResultOutcome(shredResult.shredded_files, shredResult.failed_files.length)
      : null
  ), [shredResult]);
  const scanHasPermissionFailure = hasPermissionFailure((scanResult?.errors || []).map((scanError) => scanError.reason));
  const cleanHasPermissionFailure = hasPermissionFailure(Object.values(cleanResult?.failed_reasons || {}));
  const quarantineHasPermissionFailure = hasPermissionFailure(Object.values(quarantineResult?.failed_reasons || {}));
  const registryHasPermissionFailure = hasPermissionFailure(Object.values(registryResult?.failed_reasons || {}));
  const shredHasPermissionFailure = hasPermissionFailure(Object.values(shredResult?.failed_reasons || {}));
  const progressPercent = clampPercent(scanProgress?.percent || 0);

  function setFilteredSelection(selected: boolean) {
    setScanResult((current) => {
      if (!current) {
        return current;
      }
      const currentRows = filterScanRows(current.items, selectedCategory, selectedRisk);
      return {
        ...current,
        items: updateRowsSelection(current.items, currentRows, selected),
      };
    });
  }

  function selectVisibleSafeItems() {
    setFilteredSelection(true);
  }

  function clearFilteredSelection() {
    setFilteredSelection(false);
  }

  function clearSelection() {
    setScanResult((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        items: current.items.map((item) => ({ ...item, selected: false })),
      };
    });
  }

  async function cleanSelectedItems() {
    if (selectedCleanItems.length === 0) {
      return;
    }

    const summary = [
      `选中项：${selectedCleanItems.length} 项`,
      `预计释放：${formatBytes(selectedSize)}`,
      `风险构成：低风险 ${selectedCleanItems.filter((item) => item.risk === 'low').length}，中风险 ${selectedCleanItems.filter((item) => item.risk === 'medium').length}，高风险 ${selectedCleanItems.filter((item) => item.risk === 'high').length}`,
    ];

    const confirmed = await requestConfirmation({
      title: '确认清理选中文件',
      body: summary,
      confirmLabel: '确认清理',
      cancelLabel: '取消',
      variant: hasHighRiskSelection ? 'danger' : 'default',
    });
    if (!confirmed) {
      return;
    }

    if (hasHighRiskSelection) {
      const highRiskConfirmed = await requestConfirmation({
        title: '高风险项目二次确认',
        body: [
          '已选中高风险项目。系统目录或敏感路径可能需要管理员权限。',
          '清理失败时会保留失败项目，并把具体原因写入结果面板和操作日志。',
        ],
        confirmLabel: '继续清理',
        cancelLabel: '返回检查',
        variant: 'danger',
      });
      if (!highRiskConfirmed) {
        return;
      }
    }

    try {
      setCleaning(true);
      const result = await Clean(selectedCleanItems as any, hasHighRiskSelection);
      const clean = result as unknown as CleanResult;
      setCleanResult(clean);
      setQuarantineResult(null);
      setRegistryResult(null);
      setShredResult(null);

      setScanResult((current) => {
        if (!current) {
          return current;
        }
        const items = reconcileItemsAfterClean(current.items, selectedCleanItems, clean);
        return {
          ...current,
          items,
          total_files: items.filter((item) => item.type === 'file').length,
          total_size: items.reduce((total, item) => total + item.size, 0),
        };
      });

      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    } finally {
      setCleaning(false);
    }
  }

  function requestConfirmation(options: Omit<ConfirmDialogState, 'resolve'>): Promise<boolean> {
    return new Promise((resolve) => {
      setConfirmDialog({ ...options, resolve });
    });
  }

  function closeConfirmDialog(confirmed: boolean) {
    setConfirmDialog((current) => {
      if (current) {
        current.resolve(confirmed);
      }
      return null;
    });
  }

  async function quarantineSelectedPlugins() {
    if (selectedPluginItems.length === 0) {
      return;
    }

    const summary = [
      `选中插件：${selectedPluginItems.length} 项`,
      `插件目录占用：${formatBytes(selectedPluginSize)}`,
      '操作方式：移动到 data/quarantine/plugins，可从隔离记录恢复，不直接删除。',
    ];

    const confirmed = await requestConfirmation({
      title: '确认隔离选中插件',
      body: summary,
      confirmLabel: '确认隔离',
      cancelLabel: '取消',
      variant: 'warning',
    });
    if (!confirmed) {
      return;
    }

    try {
      setCleaning(true);
      const result = await QuarantinePlugins(selectedPluginItems as any);
      const quarantine = result as unknown as QuarantineResult;
      setQuarantineResult(quarantine);
      setCleanResult(null);
      setRegistryResult(null);
      setShredResult(null);

      setScanResult((current) => {
        if (!current) {
          return current;
        }
        const failedPaths = new Set(quarantine.failed_items || []);
        const selectedPluginPaths = new Set(selectedPluginItems.map((item) => item.path));
        const items = current.items
          .filter((item) => !(selectedPluginPaths.has(item.path) && !failedPaths.has(item.path)))
          .map((item) => (
            failedPaths.has(item.path) ? { ...item, selected: false } : item
          ));
        return {
          ...current,
          items,
          total_files: items.filter((item) => item.type === 'file').length,
          total_size: items.reduce((total, item) => total + item.size, 0),
        };
      });

      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    } finally {
      setCleaning(false);
    }
  }

  async function runRegistryScan() {
    try {
      setScanning(true);
      setScanProgress({
        phase: 'scanning_registry',
        current_label: '准备扫描 HKCU Run',
        completed_steps: 0,
        total_steps: 1,
        found_items: 0,
        failed_items: 0,
        percent: 0,
      });
      setCleanResult(null);
      setQuarantineResult(null);
      setRegistryResult(null);
      setShredResult(null);
      const result = await ScanInvalidStartupRegistry();
      const registryScan = result as unknown as ScanResult;

      setScanResult((current) => {
        if (!current) {
          return registryScan;
        }
        const items = [
          ...current.items.filter((item) => item.type !== 'registry'),
          ...(registryScan.items || []),
        ];
        return {
          ...current,
          items,
          errors: [...(current.errors || []), ...(registryScan.errors || [])],
          total_files: items.filter((item) => item.type === 'file').length,
          total_size: items.reduce((total, item) => total + item.size, 0),
          duration_ms: current.duration_ms + registryScan.duration_ms,
        };
      });

      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
      setScanProgress(null);
    } finally {
      setScanning(false);
    }
  }

  async function deleteSelectedRegistryItems() {
    if (selectedRegistryItems.length === 0) {
      return;
    }

    const summary = [
      `选中注册表项：${selectedRegistryItems.length} 项`,
      '范围：仅 HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run 中的无效启动项。',
      '删除前会导出 .reg 备份到 data/registry_backup/。',
    ];

    const confirmed = await requestConfirmation({
      title: '确认备份并删除注册表项',
      body: ['注册表删除可能影响应用自启动。', ...summary],
      confirmLabel: '确认下一步',
      cancelLabel: '取消',
      variant: 'danger',
    });
    if (!confirmed) {
      return;
    }

    const highRiskConfirmed = await requestConfirmation({
      title: '注册表高风险二次确认',
      body: [
        '这是高风险操作。程序会先导出备份，备份失败时拒绝删除。',
        '只删除当前选中的注册表值，不扫描或修复全注册表。',
      ],
      confirmLabel: '备份后删除',
      cancelLabel: '返回检查',
      variant: 'danger',
    });
    if (!highRiskConfirmed) {
      return;
    }

    try {
      setCleaning(true);
      const result = await DeleteRegistryItems(selectedRegistryItems as any, true);
      const registry = result as unknown as RegistryActionResult;
      setRegistryResult(registry);
      setCleanResult(null);
      setQuarantineResult(null);
      setShredResult(null);

      setScanResult((current) => {
        if (!current) {
          return current;
        }
        const failedPaths = new Set(registry.failed_items || []);
        const selectedPaths = new Set(selectedRegistryItems.map((item) => item.path));
        const items = current.items
          .filter((item) => !(selectedPaths.has(item.path) && !failedPaths.has(item.path)))
          .map((item) => (
            failedPaths.has(item.path) ? { ...item, selected: false } : item
          ));
        return {
          ...current,
          items,
          total_files: items.filter((item) => item.type === 'file').length,
          total_size: items.reduce((total, item) => total + item.size, 0),
        };
      });

      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    } finally {
      setCleaning(false);
    }
  }

  async function chooseShredFile() {
    try {
      const path = await SelectShredFile();
      if (path) {
        setShredPath(path);
      }
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    }
  }

  async function shredSelectedFile() {
    if (!shredPath.trim()) {
      return;
    }

    const warning = [
      `文件：${shredPath}`,
      `覆写次数：${shredPasses}`,
      '限制：SSD、NTFS 日志、系统缓存、云同步目录等场景下无法保证专业取证级不可恢复。',
    ];

    const confirmed = await requestConfirmation({
      title: '确认粉碎该文件',
      body: warning,
      confirmLabel: '确认下一步',
      cancelLabel: '取消',
      variant: 'danger',
    });
    if (!confirmed) {
      return;
    }

    const highRiskConfirmed = await requestConfirmation({
      title: '文件粉碎二次确认',
      body: [
        '粉碎后无法通过本程序恢复。',
        '确认继续执行覆写、随机重命名并删除该文件。',
      ],
      confirmLabel: '继续粉碎',
      cancelLabel: '返回检查',
      variant: 'danger',
    });
    if (!highRiskConfirmed) {
      return;
    }

    try {
      setCleaning(true);
      const result = await ShredFile({ path: shredPath, passes: shredPasses } as any, true);
      const shred = result as unknown as ShredResult;
      setShredResult(shred);
      setCleanResult(null);
      setQuarantineResult(null);
      setRegistryResult(null);
      if (shred.shredded_files > 0) {
        setShredPath('');
      }
      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    } finally {
      setCleaning(false);
    }
  }

  const ruleStats = {
    total: rules.length,
    low: rules.filter((rule) => rule.risk === 'low').length,
    medium: rules.filter((rule) => rule.risk === 'medium').length,
    high: rules.filter((rule) => rule.risk === 'high').length,
  };

  return (
    <div id="app">
      <header className="app-header">
        <div className="brand-block">
          <h1>GoCleaner</h1>
          <span className="subtitle">Windows 清理工具</span>
        </div>
        <span className={`status-badge ${error ? 'status-error' : 'status-ok'}`}>
          {backendStatus}
        </span>
        <button className="primary-action" onClick={runScan} disabled={loading || scanning || cleaning}>
          {scanning ? '扫描中...' : '扫描'}
        </button>
      </header>

      <main className="app-main">
        {loading ? (
          <section className="state-panel">
            <div className="spinner" />
            <span>正在加载配置</span>
          </section>
        ) : (
          <>
            {error && (
              <section className="error-panel" role="alert">
                <strong>错误</strong>
                <span>{error}</span>
                <button onClick={loadInitialData}>重试</button>
              </section>
            )}

            {scanProgress && (
              <section className="progress-panel" aria-live="polite">
                <div className="progress-copy">
                  <strong>{scanPhaseLabel(scanProgress.phase)}</strong>
                  <span>{scanProgress.current_label || '等待后端返回进度'}</span>
                </div>
                <div className="progress-meta">
                  <span>{progressPercent}%</span>
                  <span>发现 {scanProgress.found_items} 项 / 失败 {scanProgress.failed_items} 项</span>
                </div>
                <div className="progress-track" aria-label="扫描进度">
                  <div className="progress-fill" style={{ width: `${progressPercent}%` }} />
                </div>
              </section>
            )}

            <section className="stats-bar">
              <div className="stat-item">
                <span className="stat-value">{scanResult ? scanResult.total_files : ruleStats.total}</span>
                <span className="stat-label">{scanResult ? '扫描文件数' : '规则数量'}</span>
              </div>
              <div className="stat-item">
                <span className="stat-value">{scanResult ? formatBytes(scanResult.total_size) : `${ruleStats.low}/${ruleStats.medium}/${ruleStats.high}`}</span>
                <span className="stat-label">{scanResult ? '扫描总大小' : '低/中/高风险规则'}</span>
              </div>
              <div className="stat-item">
                <span className="stat-value">{selectedCleanItems.length}</span>
                <span className="stat-label">已勾选项目</span>
              </div>
              <div className="stat-item">
                <span className="stat-value">{formatBytes(selectedSize)}</span>
                <span className="stat-label">已勾选大小</span>
              </div>
              <div className={`stat-item ${selectedPluginItems.length > 0 ? 'stat-warning' : ''}`}>
                <span className="stat-value">{selectedPluginItems.length}</span>
                <span className="stat-label">已选插件（{formatBytes(selectedPluginSize)}）</span>
              </div>
              <div className={`stat-item ${selectedRegistryItems.length > 0 ? 'stat-high' : ''}`}>
                <span className="stat-value">{selectedRegistryItems.length}</span>
                <span className="stat-label">已选注册表项</span>
              </div>
              <div className={`stat-item ${failureSummary.total > 0 ? 'stat-warning' : ''}`}>
                <span className="stat-value">{failureSummary.total}</span>
                <span className="stat-label">失败项（扫描 {failureSummary.scan} / 清理 {failureSummary.clean}）</span>
              </div>
              <div className={`stat-item ${hasHighRiskSelection ? 'stat-high' : ''}`}>
                <span className="stat-value">{selectedRiskCounts.high}</span>
                <span className="stat-label">已选高风险</span>
              </div>
            </section>

            {ruleWarnings.length > 0 && (
              <section className="notice-panel">
                <strong>规则警告</strong>
                <ul>
                  {ruleWarnings.map((warning, idx) => (
                    <li key={idx}>{warning}</li>
                  ))}
                </ul>
              </section>
            )}

            <section className="env-strip">
              {Object.entries(envInfo).map(([key, value]) => (
                <div key={key} className="env-item">
                  <code>%{key}%</code>
                  <span>{value}</span>
                </div>
              ))}
            </section>

            <section className="toolbar">
              <div className="filter-group">
                {categories.map((category) => (
                  <button
                    key={category}
                    className={`filter-btn ${selectedCategory === category ? 'active' : ''}`}
                    onClick={() => setSelectedCategory(category)}
                  >
                    {category === 'all' ? '全部分类' : categoryLabel(category)}
                  </button>
                ))}
              </div>
              <div className="filter-group">
                {(['all', ...riskOrder] as RiskFilter[]).map((risk) => (
                  <button
                    key={risk}
                    className={`filter-btn ${selectedRisk === risk ? 'active' : ''}`}
                    onClick={() => setSelectedRisk(risk)}
                  >
                    {risk === 'all' ? '全部风险' : riskLabel(risk)}
                  </button>
                ))}
              </div>
              <div className="toolbar-actions">
                <button onClick={runRegistryScan} disabled={loading || scanning || cleaning}>
                  {scanning ? '扫描中...' : '扫描注册表'}
                </button>
                <button
                  onClick={selectVisibleSafeItems}
                  disabled={!scanResult || selectableFilteredCount === 0 || selectableFilteredCount === selectedFilteredCount}
                  title="批量选择会跳过高风险项目"
                >
                  全选当前筛选
                </button>
                <button onClick={clearFilteredSelection} disabled={!scanResult || selectedFilteredAnyCount === 0}>
                  取消当前筛选
                </button>
                <button onClick={clearSelection} disabled={selectedItems.length === 0}>
                  清空选择
                </button>
                <button className="warning-action" onClick={quarantineSelectedPlugins} disabled={selectedPluginItems.length === 0 || cleaning}>
                  {cleaning ? '处理中...' : `隔离插件 ${selectedPluginItems.length}`}
                </button>
                <button className="danger-action" onClick={deleteSelectedRegistryItems} disabled={selectedRegistryItems.length === 0 || cleaning}>
                  {cleaning ? '处理中...' : `删除注册表项 ${selectedRegistryItems.length}`}
                </button>
                <button className="danger-action" onClick={cleanSelectedItems} disabled={selectedCleanItems.length === 0 || cleaning}>
                  {cleaning ? '清理中...' : `清理 ${selectedCleanItems.length}`}
                </button>
              </div>
            </section>

            <section className="shred-panel">
              <div className="section-heading inline-heading">
                <h2>文件粉碎</h2>
                <span>仅手动选择单个普通文件</span>
              </div>
              <div className="shred-controls">
                <button onClick={chooseShredFile} disabled={cleaning}>选择文件粉碎</button>
                <input value={shredPath} readOnly placeholder="尚未选择文件" aria-label="待粉碎文件路径" />
                <label>
                  覆写次数
                  <select value={shredPasses} onChange={(event) => setShredPasses(Number(event.target.value))}>
                    <option value={1}>1</option>
                    <option value={3}>3</option>
                    <option value={7}>7</option>
                  </select>
                </label>
                <button className="danger-action" onClick={shredSelectedFile} disabled={!shredPath || cleaning}>
                  {cleaning ? '粉碎中...' : '确认粉碎'}
                </button>
              </div>
              <p className="risk-copy">
                文件粉碎是高风险操作。SSD、NTFS 日志、系统缓存、云同步目录等场景下无法保证专业取证级不可恢复。
              </p>
            </section>

            {cleanResult && (
              <section className={resultPanelClass(cleanOutcome)} aria-live="polite">
                <strong>清理结果</strong>
                <span className="result-state">{outcomeLabel(cleanOutcome)}</span>
                <span>{cleanResult.message}</span>
                <WarningList warnings={cleanResult.warnings} />
                <span>已删除 {cleanResult.deleted_files} 项 | 释放 {formatBytes(cleanResult.freed_size)} | 失败 {cleanResult.failed_files.length} 项</span>
                {cleanHasPermissionFailure && (
                  <span className="recovery-hint">存在权限不足项：可跳过该项，或确认文件不重要后以管理员身份运行；程序不会自动提权。</span>
                )}
                {cleanResult.failed_files.length > 0 && (
                  <ul>
                    {cleanResult.failed_files.map((path) => (
                      <li key={path}>
                        <code>{path}</code>
                        <span>{cleanResult.failed_reasons[path]}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}

            {quarantineResult && (
              <section className={resultPanelClass(quarantineOutcome)} aria-live="polite">
                <strong>隔离结果</strong>
                <span className="result-state">{outcomeLabel(quarantineOutcome)}</span>
                <span>{quarantineResult.message}</span>
                <WarningList warnings={quarantineResult.warnings} />
                <span>已隔离 {quarantineResult.moved_items} 项 | 已恢复 {quarantineResult.restored_items} 项 | 失败 {quarantineResult.failed_items.length} 项</span>
                {quarantineHasPermissionFailure && (
                  <span className="recovery-hint">存在权限不足项：请关闭占用该插件目录的浏览器，必要时以管理员身份运行后重试。</span>
                )}
                {quarantineResult.failed_items.length > 0 && (
                  <ul>
                    {quarantineResult.failed_items.map((path) => (
                      <li key={path}>
                        <code>{path}</code>
                        <span>{quarantineResult.failed_reasons[path]}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}

            {registryResult && (
              <section className={resultPanelClass(registryOutcome)} aria-live="polite">
                <strong>注册表结果</strong>
                <span className="result-state">{outcomeLabel(registryOutcome)}</span>
                <span>{registryResult.message}</span>
                <WarningList warnings={registryResult.warnings} />
                <span>已删除 {registryResult.deleted_values} 项 | 备份 {registryResult.backup_path || '-'} | 失败 {registryResult.failed_items.length} 项</span>
                {registryHasPermissionFailure && (
                  <span className="recovery-hint">存在权限不足项：当前版本只处理 HKCU 安全范围，不会尝试全注册表修复或静默跳过。</span>
                )}
                {registryResult.failed_items.length > 0 && (
                  <ul>
                    {registryResult.failed_items.map((path) => (
                      <li key={path}>
                        <code>{path}</code>
                        <span>{registryResult.failed_reasons[path]}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}

            {shredResult && (
              <section className={resultPanelClass(shredOutcome)} aria-live="polite">
                <strong>粉碎结果</strong>
                <span className="result-state">{outcomeLabel(shredOutcome)}</span>
                <span>{shredResult.message}</span>
                <WarningList warnings={shredResult.warnings} />
                <span>已粉碎 {shredResult.shredded_files} 项 | 释放 {formatBytes(shredResult.freed_size)} | 失败 {shredResult.failed_files.length} 项</span>
                {shredHasPermissionFailure && (
                  <span className="recovery-hint">存在权限不足项：请确认文件未被系统或同步程序占用，必要时选择测试文件演示粉碎流程。</span>
                )}
                {shredResult.failed_files.length > 0 && (
                  <ul>
                    {shredResult.failed_files.map((path) => (
                      <li key={path}>
                        <code>{path}</code>
                        <span>{shredResult.failed_reasons[path]}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}

            <section className="table-section">
              <div className="section-heading">
                <h2>扫描结果</h2>
                {scanResult && (
                  <div className="table-pagination" aria-label="扫描结果分页">
                    <span>
                      显示 {pagedRows.from}-{pagedRows.to} / 筛选 {filteredRows.length} / 总 {scanItems.length} 项，耗时 {scanResult.duration_ms} ms
                    </span>
                    <label>
                      每页
                      <select value={resultsPageSize} onChange={(event) => setResultsPageSize(Number(event.target.value))}>
                        {pageSizeOptions.map((size) => (
                          <option key={size} value={size}>{size}</option>
                        ))}
                      </select>
                    </label>
                    <button onClick={() => setResultsPage((page) => Math.max(1, page - 1))} disabled={pagedRows.page <= 1}>
                      上一页
                    </button>
                    <span>{pagedRows.page} / {pagedRows.totalPages}</span>
                    <button onClick={() => setResultsPage((page) => Math.min(pagedRows.totalPages, page + 1))} disabled={pagedRows.page >= pagedRows.totalPages}>
                      下一页
                    </button>
                  </div>
                )}
              </div>
              <div className="table-wrap">
                <table className="data-table scan-table">
                  <colgroup>
                    <col className="scan-col-check" />
                    <col className="scan-col-name" />
                    <col className="scan-col-category" />
                    <col className="scan-col-risk" />
                    <col className="scan-col-size" />
                    <col className="scan-col-modified" />
                    <col className="scan-col-source" />
                    <col className="scan-col-path" />
                  </colgroup>
                  <thead>
                    <tr>
                      <th className="checkbox-col">勾选</th>
                      <th>名称</th>
                      <th>分类</th>
                      <th>风险</th>
                      <th>大小</th>
                      <th>修改时间</th>
                      <th>来源规则</th>
                      <th>路径</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pagedRows.rows.map(({ item, index, key }) => (
                      <tr key={key} className={item.risk === 'high' ? 'high-risk-row' : ''}>
                        <td className="checkbox-col">
                          <input
                            type="checkbox"
                            checked={item.selected}
                            onChange={(event) => setItemSelected(index, event.target.checked)}
                          />
                        </td>
                        <td className="name-cell">
                          <div className="item-name" title={item.name}>{item.name}</div>
                          {item.plugin && (
                            <div className="item-meta" title={`${item.plugin.browser} / ${item.plugin.profile} / v${item.plugin.version || '-'}`}>
                              {item.plugin.browser} / {item.plugin.profile} / v{item.plugin.version || '-'}
                            </div>
                          )}
                          {item.registry && (
                            <div className="item-meta" title={`${item.registry.hive} / ${item.registry.value_type}`}>
                              {item.registry.hive} / {item.registry.value_type}
                            </div>
                          )}
                        </td>
                        <td className="category-cell">{categoryLabel(item.category)}</td>
                        <td className="risk-cell">
                          <span className="risk-tag" style={{ backgroundColor: RiskColors[item.risk] }}>
                            {riskLabel(item.risk)}
                          </span>
                        </td>
                        <td className="size-cell">{formatBytes(item.size)}</td>
                        <td className="date-cell">{formatDateFromSeconds(item.last_modified)}</td>
                        <td className="source-cell">
                          <div title={item.source}>{item.source}</div>
                          {item.plugin && (
                            <div className="item-meta" title={item.plugin.extension_id}>{item.plugin.extension_id}</div>
                          )}
                          {item.registry && (
                            <div className="item-meta" title={item.registry.target_path}>{item.registry.target_path}</div>
                          )}
                        </td>
                        <td className="path-cell" title={item.path}>{item.path}</td>
                      </tr>
                    ))}
                    {!scanResult && (
                      <tr>
                        <td colSpan={8} className="empty-row">尚未扫描</td>
                      </tr>
                    )}
                    {scanResult && filteredRows.length === 0 && (
                      <tr>
                        <td colSpan={8} className="empty-row">
                          {scanItems.length === 0 ? '扫描完成，未发现可清理项目' : '当前筛选条件下没有匹配项'}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>

            {scanResult && scanResult.errors.length > 0 && (
              <section className="notice-panel" role="alert">
                <strong>扫描失败项</strong>
                {scanHasPermissionFailure && (
                  <span className="recovery-hint">存在权限不足路径：这通常来自系统目录或被策略保护的目录，可保留为失败记录，必要时以管理员身份运行后重新扫描。</span>
                )}
                <ul>
                  {scanResult.errors.map((scanError) => (
                    <li key={`${scanError.path}-${scanError.reason}`}>
                      <code>{scanError.path}</code>
                      <span>{scanError.reason}</span>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section className="table-section">
              <div className="section-heading">
                <h2>操作日志</h2>
                <span>最近 {operationLogs.length} 条记录</span>
              </div>
              <div className="table-wrap">
                <table className="data-table compact-table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>操作</th>
                      <th>扫描数</th>
                      <th>处理数</th>
                      <th>释放空间</th>
                      <th>失败详情</th>
                      <th>耗时</th>
                    </tr>
                  </thead>
                  <tbody>
                    {operationLogs.map((entry, idx) => (
                      <tr key={`${entry.timestamp}-${idx}`}>
                        <td>{formatTimestamp(entry.timestamp)}</td>
                        <td>{operationLabel(entry.operation)}</td>
                        <td>{entry.scanned_files}</td>
                        <td>{entry.deleted_files}</td>
                        <td>{formatBytes(entry.freed_size)}</td>
                        <td className="log-failure-cell">
                          {(entry.failed_paths?.length || 0) === 0 ? (
                            '0'
                          ) : (
                            <details>
                              <summary>{entry.failed_paths.length} 项</summary>
                              <ul>
                                {entry.failed_paths.map((path, failedIdx) => (
                                  <li key={`${entry.timestamp}-${path}-${failedIdx}`}>
                                    <code>{path}</code>
                                    <span>{entry.failed_reasons?.[failedIdx] || '-'}</span>
                                  </li>
                                ))}
                              </ul>
                            </details>
                          )}
                        </td>
                        <td>{entry.duration} ms</td>
                      </tr>
                    ))}
                    {operationLogs.length === 0 && (
                      <tr>
                        <td colSpan={7} className="empty-row">暂无操作日志</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>
          </>
        )}
      </main>

      {confirmDialog && (
        <div className="modal-backdrop" role="presentation">
          <section
            className={`confirm-dialog confirm-${confirmDialog.variant}`}
            role="dialog"
            aria-modal="true"
            aria-labelledby="confirm-dialog-title"
          >
            <div className="confirm-header">
              <h2 id="confirm-dialog-title">{confirmDialog.title}</h2>
            </div>
            <div className="confirm-body">
              {confirmDialog.body.map((line, idx) => (
                <p key={`${confirmDialog.title}-${idx}`}>{line}</p>
              ))}
            </div>
            <div className="confirm-actions">
              <button onClick={() => closeConfirmDialog(false)} autoFocus>
                {confirmDialog.cancelLabel}
              </button>
              <button
                className={confirmDialog.variant === 'danger' ? 'danger-action' : confirmDialog.variant === 'warning' ? 'warning-action' : 'primary-action'}
                onClick={() => closeConfirmDialog(true)}
              >
                {confirmDialog.confirmLabel}
              </button>
            </div>
          </section>
        </div>
      )}

      <footer className="app-footer">
        <span>GoCleaner v1.0-dev</span>
      </footer>
    </div>
  );
}

export default App;
