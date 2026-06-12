import { useEffect, useMemo, useState } from 'react';
import './App.css';
import {
  Clean,
  GetEnvInfo,
  GetOperationLogs,
  GetRulesPreview,
  GetRulesWarnings,
  Ping,
  Scan,
} from '../wailsjs/go/app/App';
import {
  CategoryLabels,
  RiskColors,
  RiskLabels,
} from './models';
import type {
  CleanResult,
  CleanRule,
  OperationLog,
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

function App() {
  const [backendStatus, setBackendStatus] = useState('Checking...');
  const [rules, setRules] = useState<CleanRule[]>([]);
  const [ruleWarnings, setRuleWarnings] = useState<string[]>([]);
  const [envInfo, setEnvInfo] = useState<Record<string, string>>({});
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [cleanResult, setCleanResult] = useState<CleanResult | null>(null);
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
      const status = await Ping();
      setBackendStatus(status);

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
      setError(e?.message || String(e));
      setBackendStatus('Backend unavailable');
    } finally {
      setLoading(false);
    }
  }

  async function runScan() {
    try {
      setScanning(true);
      setCleanResult(null);
      const result = await Scan();
      setScanResult(result as unknown as ScanResult);
      await loadOperationLogs();
      setError(null);
    } catch (e: any) {
      setError(e?.message || String(e));
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

  const selectedItems = useMemo(() => scanItems.filter((item) => item.selected), [scanItems]);
  const selectedSize = selectedItems.reduce((total, item) => total + item.size, 0);
  const selectedRiskCounts = riskOrder.reduce<Record<RiskLevel, number>>((counts, risk) => {
    counts[risk] = selectedItems.filter((item) => item.risk === risk).length;
    return counts;
  }, { low: 0, medium: 0, high: 0 });
  const hasHighRiskSelection = selectedRiskCounts.high > 0;

  function selectVisibleSafeItems() {
    const visibleSafeIds = new Set(
      filteredItems
        .filter((item) => item.risk !== 'high')
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
    if (selectedItems.length === 0) {
      return;
    }

    const summary = [
      `${selectedItems.length} item(s)`,
      formatBytes(selectedSize),
      `low ${selectedRiskCounts.low}`,
      `medium ${selectedRiskCounts.medium}`,
      `high ${selectedRiskCounts.high}`,
    ].join(' | ');

    if (!window.confirm(`Clean selected files?\n${summary}`)) {
      return;
    }

    if (hasHighRiskSelection && !window.confirm('High-risk items are selected. Continue with high-risk cleaning?')) {
      return;
    }

    try {
      setCleaning(true);
      const result = await Clean(selectedItems as any, hasHighRiskSelection);
      const clean = result as unknown as CleanResult;
      setCleanResult(clean);

      const failedPaths = new Set(clean.failed_files || []);
      const selectedPaths = new Set(selectedItems.map((item) => item.path));
      setScanResult((current) => {
        if (!current) {
          return current;
        }
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
      setError(e?.message || String(e));
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
          <span className="subtitle">Windows cleanup workbench</span>
        </div>
        <span className={`status-badge ${error ? 'status-error' : 'status-ok'}`}>
          {backendStatus}
        </span>
        <button className="primary-action" onClick={runScan} disabled={loading || scanning || cleaning}>
          {scanning ? 'Scanning...' : 'Scan'}
        </button>
      </header>

      <main className="app-main">
        {loading ? (
          <section className="state-panel">
            <div className="spinner" />
            <span>Loading configuration</span>
          </section>
        ) : (
          <>
            {error && (
              <section className="error-panel">
                <strong>Error</strong>
                <span>{error}</span>
                <button onClick={loadInitialData}>Retry</button>
              </section>
            )}

            <section className="stats-bar">
              <div className="stat-item">
                <span className="stat-value">{scanResult ? scanResult.total_files : ruleStats.total}</span>
                <span className="stat-label">{scanResult ? 'Scanned files' : 'Rules'}</span>
              </div>
              <div className="stat-item">
                <span className="stat-value">{scanResult ? formatBytes(scanResult.total_size) : ruleStats.low}</span>
                <span className="stat-label">{scanResult ? 'Detected size' : 'Low risk rules'}</span>
              </div>
              <div className="stat-item">
                <span className="stat-value">{selectedItems.length}</span>
                <span className="stat-label">Selected</span>
              </div>
              <div className={`stat-item ${hasHighRiskSelection ? 'stat-high' : ''}`}>
                <span className="stat-value">{selectedRiskCounts.high}</span>
                <span className="stat-label">High risk selected</span>
              </div>
            </section>

            {ruleWarnings.length > 0 && (
              <section className="notice-panel">
                <strong>Rule warnings</strong>
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
                    {category === 'all' ? 'All categories' : categoryLabel(category)}
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
                    {risk === 'all' ? 'All risks' : riskLabel(risk)}
                  </button>
                ))}
              </div>
              <div className="toolbar-actions">
                <button onClick={selectVisibleSafeItems} disabled={!scanResult || filteredItems.length === 0}>
                  Select visible safe
                </button>
                <button onClick={clearSelection} disabled={selectedItems.length === 0}>
                  Clear
                </button>
                <button className="danger-action" onClick={cleanSelectedItems} disabled={selectedItems.length === 0 || cleaning}>
                  {cleaning ? 'Cleaning...' : `Clean ${selectedItems.length}`}
                </button>
              </div>
            </section>

            {cleanResult && (
              <section className="result-panel">
                <strong>Clean result</strong>
                <span>{cleanResult.message}</span>
                <span>{cleanResult.deleted_files} deleted | {formatBytes(cleanResult.freed_size)} freed | {cleanResult.failed_files.length} failed</span>
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

            <section className="table-section">
              <div className="section-heading">
                <h2>Scan Results</h2>
                {scanResult && <span>{filteredItems.length} visible / {scanItems.length} total</span>}
              </div>
              <div className="table-wrap">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th className="checkbox-col">Pick</th>
                      <th>Name</th>
                      <th>Category</th>
                      <th>Risk</th>
                      <th>Size</th>
                      <th>Modified</th>
                      <th>Source</th>
                      <th>Path</th>
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
                        <td>{item.name}</td>
                        <td>{categoryLabel(item.category)}</td>
                        <td>
                          <span className="risk-tag" style={{ backgroundColor: RiskColors[item.risk] }}>
                            {riskLabel(item.risk)}
                          </span>
                        </td>
                        <td>{formatBytes(item.size)}</td>
                        <td>{formatDateFromSeconds(item.last_modified)}</td>
                        <td>{item.source}</td>
                        <td className="path-cell">{item.path}</td>
                      </tr>
                    ))}
                    {!scanResult && (
                      <tr>
                        <td colSpan={8} className="empty-row">No scan results</td>
                      </tr>
                    )}
                    {scanResult && filteredItems.length === 0 && (
                      <tr>
                        <td colSpan={8} className="empty-row">No matching items</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>

            {scanResult && scanResult.errors.length > 0 && (
              <section className="notice-panel">
                <strong>Scan failures</strong>
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
                <h2>Operation Log</h2>
                <span>{operationLogs.length} recent entries</span>
              </div>
              <div className="table-wrap">
                <table className="data-table compact-table">
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Operation</th>
                      <th>Scanned</th>
                      <th>Deleted</th>
                      <th>Freed</th>
                      <th>Failures</th>
                      <th>Duration</th>
                    </tr>
                  </thead>
                  <tbody>
                    {operationLogs.map((entry, idx) => (
                      <tr key={`${entry.timestamp}-${idx}`}>
                        <td>{formatTimestamp(entry.timestamp)}</td>
                        <td>{entry.operation}</td>
                        <td>{entry.scanned_files}</td>
                        <td>{entry.deleted_files}</td>
                        <td>{formatBytes(entry.freed_size)}</td>
                        <td>{entry.failed_paths?.length || 0}</td>
                        <td>{entry.duration} ms</td>
                      </tr>
                    ))}
                    {operationLogs.length === 0 && (
                      <tr>
                        <td colSpan={7} className="empty-row">No operation logs</td>
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
