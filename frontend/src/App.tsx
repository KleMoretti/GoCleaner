import { useEffect, useMemo, useState } from 'react';
import './App.css';
import {
  Clean,
  GetEnvInfo,
  GetOperationLogs,
  GetRulesPreview,
  GetRulesWarnings,
  Ping,
  QuarantinePlugins,
  Scan,
} from '../wailsjs/go/app/App';
import {
  CategoryLabels,
  RiskColors,
  RiskLabels,
} from './models';
import {
  countFailures,
  reconcileItemsAfterClean,
  summarizeSelection,
} from './summary';
import type {
  CleanResult,
  CleanRule,
  OperationLog,
  QuarantineResult,
  RiskLevel,
  ScanItem,
  ScanResult,
} from './models';

type RiskFilter = 'all' | RiskLevel;

const riskOrder: RiskLevel[] = ['low', 'medium', 'high'];

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
  const [operationLogs, setOperationLogs] = useState<OperationLog[]>([]);
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [selectedRisk, setSelectedRisk] = useState<RiskFilter>('all');
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadInitialData();
  }, []);

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
      setCleanResult(null);
      setQuarantineResult(null);
      const result = await Scan();
      setScanResult(result as unknown as ScanResult);
      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(errorMessage(e));
    } finally {
      setScanning(false);
    }
  }

  function setItemSelected(id: string, selected: boolean) {
    setScanResult((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        items: current.items.map((item) => (
          item.id === id ? { ...item, selected } : item
        )),
      };
    });
  }

  const scanItems = scanResult?.items || [];

  const categories = useMemo(() => {
    const source = scanItems.length > 0 ? scanItems.map((item) => item.category) : rules.map((rule) => rule.category);
    return ['all', ...Array.from(new Set(source))];
  }, [rules, scanItems]);

  const filteredItems = useMemo(() => {
    return scanItems.filter((item) => {
      const categoryMatch = selectedCategory === 'all' || item.category === selectedCategory;
      const riskMatch = selectedRisk === 'all' || item.risk === selectedRisk;
      return categoryMatch && riskMatch;
    });
  }, [scanItems, selectedCategory, selectedRisk]);

  const selectionSummary = useMemo(() => summarizeSelection(scanItems), [scanItems]);
  const selectedItems = selectionSummary.items;
  const selectedCleanItems = selectionSummary.cleanableItems;
  const selectedPluginItems = selectionSummary.pluginItems;
  const selectedSize = selectionSummary.size;
  const selectedPluginSize = selectionSummary.pluginSize;
  const selectedRiskCounts = selectionSummary.riskCounts;
  const hasHighRiskSelection = selectedCleanItems.some((item) => item.risk === 'high');
  const failureSummary = useMemo(
    () => countFailures(scanResult, cleanResult),
    [scanResult, cleanResult],
  );

  function selectVisibleSafeItems() {
    const visibleSafeIds = new Set(
      filteredItems
        .filter((item) => item.risk !== 'high' && item.type !== 'plugin')
        .map((item) => item.id),
    );
    setScanResult((current) => {
      if (!current) {
        return current;
      }
      return {
        ...current,
        items: current.items.map((item) => (
          visibleSafeIds.has(item.id) ? { ...item, selected: true } : item
        )),
      };
    });
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
    ].join('\n');

    if (!window.confirm(`确认清理选中文件？\n${summary}`)) {
      return;
    }

    if (hasHighRiskSelection && !window.confirm('已选中高风险项目。系统目录或敏感路径可能需要管理员权限，且失败原因会记录到日志。确认继续清理？')) {
      return;
    }

    try {
      setCleaning(true);
      const result = await Clean(selectedCleanItems as any, hasHighRiskSelection);
      const clean = result as unknown as CleanResult;
      setCleanResult(clean);
      setQuarantineResult(null);

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

  async function quarantineSelectedPlugins() {
    if (selectedPluginItems.length === 0) {
      return;
    }

    const summary = [
      `选中插件：${selectedPluginItems.length} 项`,
      `插件目录占用：${formatBytes(selectedPluginSize)}`,
      '操作方式：移动到 data/quarantine/plugins，可从隔离记录恢复，不直接删除。',
    ].join('\n');

    if (!window.confirm(`确认隔离选中插件？\n${summary}`)) {
      return;
    }

    try {
      setCleaning(true);
      const result = await QuarantinePlugins(selectedPluginItems as any);
      const quarantine = result as unknown as QuarantineResult;
      setQuarantineResult(quarantine);
      setCleanResult(null);

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
          <span className="subtitle">Windows 空间清理工作台</span>
        </div>
        <span className={`status-badge ${error ? 'status-error' : 'status-ok'}`}>
          {backendStatus}
        </span>
        <button className="primary-action" onClick={runScan} disabled={loading || scanning || cleaning}>
          {scanning ? '扫描中...' : '开始扫描'}
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
                <button onClick={selectVisibleSafeItems} disabled={!scanResult || filteredItems.length === 0}>
                  选择当前可见安全项
                </button>
                <button onClick={clearSelection} disabled={selectedItems.length === 0}>
                  清空选择
                </button>
                <button className="warning-action" onClick={quarantineSelectedPlugins} disabled={selectedPluginItems.length === 0 || cleaning}>
                  {cleaning ? '处理中...' : `隔离插件 ${selectedPluginItems.length} 项`}
                </button>
                <button className="danger-action" onClick={cleanSelectedItems} disabled={selectedCleanItems.length === 0 || cleaning}>
                  {cleaning ? '清理中...' : `清理 ${selectedCleanItems.length} 项`}
                </button>
              </div>
            </section>

            {cleanResult && (
              <section className="result-panel" aria-live="polite">
                <strong>清理结果</strong>
                <span>{cleanResult.message}</span>
                <span>已删除 {cleanResult.deleted_files} 项 | 释放 {formatBytes(cleanResult.freed_size)} | 失败 {cleanResult.failed_files.length} 项</span>
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
              <section className="result-panel" aria-live="polite">
                <strong>隔离结果</strong>
                <span>{quarantineResult.message}</span>
                <span>已隔离 {quarantineResult.moved_items} 项 | 已恢复 {quarantineResult.restored_items} 项 | 失败 {quarantineResult.failed_items.length} 项</span>
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

            <section className="table-section">
              <div className="section-heading">
                <h2>扫描结果</h2>
                {scanResult && <span>当前显示 {filteredItems.length} 项 / 共 {scanItems.length} 项，耗时 {scanResult.duration_ms} ms</span>}
              </div>
              <div className="table-wrap">
                <table className="data-table">
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
                    {filteredItems.map((item) => (
                      <tr key={item.id} className={item.risk === 'high' ? 'high-risk-row' : ''}>
                        <td className="checkbox-col">
                          <input
                            type="checkbox"
                            checked={item.selected}
                            onChange={(event) => setItemSelected(item.id, event.target.checked)}
                          />
                        </td>
                        <td>
                          <div className="item-name">{item.name}</div>
                          {item.plugin && (
                            <div className="item-meta">
                              {item.plugin.browser} / {item.plugin.profile} / v{item.plugin.version || '-'}
                            </div>
                          )}
                        </td>
                        <td>{categoryLabel(item.category)}</td>
                        <td>
                          <span className="risk-tag" style={{ backgroundColor: RiskColors[item.risk] }}>
                            {riskLabel(item.risk)}
                          </span>
                        </td>
                        <td>{formatBytes(item.size)}</td>
                        <td>{formatDateFromSeconds(item.last_modified)}</td>
                        <td>
                          <div>{item.source}</div>
                          {item.plugin && (
                            <div className="item-meta">{item.plugin.extension_id}</div>
                          )}
                        </td>
                        <td className="path-cell">{item.path}</td>
                      </tr>
                    ))}
                    {!scanResult && (
                      <tr>
                        <td colSpan={8} className="empty-row">尚未扫描</td>
                      </tr>
                    )}
                    {scanResult && filteredItems.length === 0 && (
                      <tr>
                        <td colSpan={8} className="empty-row">当前筛选条件下没有匹配项</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>

            {scanResult && scanResult.errors.length > 0 && (
              <section className="notice-panel" role="alert">
                <strong>扫描失败项</strong>
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

      <footer className="app-footer">
        <span>GoCleaner v1.0-dev</span>
      </footer>
    </div>
  );
}

export default App;
