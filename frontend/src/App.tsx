import { useState, useEffect } from 'react';
import './App.css';
import { Ping, GetRulesPreview, GetRulesWarnings, GetEnvInfo } from '../wailsjs/go/app/App';
import { CleanRule, RiskLabels, RiskColors, CategoryLabels } from './models';
import { model } from '../wailsjs/go/models';

function App() {
  const [backendStatus, setBackendStatus] = useState<string>('检查中...');
  const [rules, setRules] = useState<CleanRule[]>([]);
  const [ruleWarnings, setRuleWarnings] = useState<string[]>([]);
  const [envInfo, setEnvInfo] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string>('all');

  useEffect(() => {
    loadData();
  }, []);

  async function loadData() {
    try {
      setLoading(true);
      const status = await Ping();
      setBackendStatus(status);

      const rulesList = await GetRulesPreview();
      setRules((rulesList || []) as unknown as CleanRule[]);

      const warnings = await GetRulesWarnings();
      setRuleWarnings(warnings || []);

      const env = await GetEnvInfo();
      setEnvInfo(env || {});

      setError(null);
    } catch (e: any) {
      setError(e?.message || String(e));
      setBackendStatus('连接失败');
    } finally {
      setLoading(false);
    }
  }

  const categories = ['all', ...new Set(rules.map(r => r.category))];
  const filteredRules = selectedCategory === 'all'
    ? rules
    : rules.filter(r => r.category === selectedCategory);

  const stats = {
    low: rules.filter(r => r.risk === 'low').length,
    medium: rules.filter(r => r.risk === 'medium').length,
    high: rules.filter(r => r.risk === 'high').length,
    total: rules.length,
  };

  return (
    <div id="app">
      {/* Header */}
      <header className="app-header">
        <h1>🧹 GoCleaner</h1>
        <span className="subtitle">操作系统空间清理工具</span>
        <span className={`status-badge ${error ? 'status-error' : 'status-ok'}`}>
          {backendStatus}
        </span>
      </header>

      {/* Main Content */}
      <main className="app-main">
        {loading ? (
          <div className="loading-container">
            <div className="spinner" />
            <p>正在加载规则配置...</p>
          </div>
        ) : error ? (
          <div className="error-panel">
            <h3>⚠ 连接后端失败</h3>
            <p>{error}</p>
            <button onClick={loadData}>重试</button>
          </div>
        ) : (
          <>
            {/* Stats Bar */}
            <section className="stats-bar">
              <div className="stat-item">
                <span className="stat-value">{stats.total}</span>
                <span className="stat-label">规则总数</span>
              </div>
              <div className="stat-item stat-low">
                <span className="stat-value">{stats.low}</span>
                <span className="stat-label">低风险</span>
              </div>
              <div className="stat-item stat-medium">
                <span className="stat-value">{stats.medium}</span>
                <span className="stat-label">中风险</span>
              </div>
              <div className="stat-item stat-high">
                <span className="stat-value">{stats.high}</span>
                <span className="stat-label">高风险</span>
              </div>
            </section>

            {/* Env Info */}
            <section className="env-info">
              <h3>环境变量展开验证</h3>
              <div className="env-grid">
                {Object.entries(envInfo).map(([key, value]) => (
                  <div key={key} className="env-item">
                    <code>%{key}%</code>
                    <span className="env-arrow">→</span>
                    <code className="env-value">{value}</code>
                  </div>
                ))}
              </div>
            </section>

            {ruleWarnings.length > 0 && (
              <section className="rule-warnings">
                <h3>规则加载警告</h3>
                <ul>
                  {ruleWarnings.map((warning, idx) => (
                    <li key={idx}>{warning}</li>
                  ))}
                </ul>
              </section>
            )}

            {/* Category Filter */}
            <section className="category-filter">
              {categories.map(cat => (
                <button
                  key={cat}
                  className={`filter-btn ${selectedCategory === cat ? 'active' : ''}`}
                  onClick={() => setSelectedCategory(cat)}
                >
                  {cat === 'all' ? '全部' : CategoryLabels[cat] || cat}
                </button>
              ))}
            </section>

            {/* Rules Table */}
            <section className="rules-preview">
              <h3>规则预览（扫描入口占位）</h3>
              <table className="rules-table">
                <thead>
                  <tr>
                    <th>规则名称</th>
                    <th>分类</th>
                    <th>扫描路径数</th>
                    <th>风险等级</th>
                    <th>默认选中</th>
                    <th>最短天数</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredRules.map((rule, idx) => (
                    <tr key={idx}>
                      <td>{rule.name}</td>
                      <td>{CategoryLabels[rule.category] || rule.category}</td>
                      <td>{rule.paths.length}</td>
                      <td>
                        <span
                          className="risk-tag"
                          style={{ backgroundColor: RiskColors[rule.risk] }}
                        >
                          {RiskLabels[rule.risk] || rule.risk}
                        </span>
                      </td>
                      <td>{rule.default_on ? '✅ 是' : '❌ 否'}</td>
                      <td>{rule.min_age_days > 0 ? `${rule.min_age_days} 天` : '-'}</td>
                    </tr>
                  ))}
                  {filteredRules.length === 0 && (
                    <tr>
                      <td colSpan={6} className="empty-row">暂无匹配规则</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </section>
          </>
        )}
      </main>

      {/* Footer */}
      <footer className="app-footer">
        <span>GoCleaner v1.0-dev | 扫描预览模式 | 尚未实现真实清理功能</span>
      </footer>
    </div>
  );
}

export default App;
