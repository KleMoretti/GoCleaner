export type RiskLevel = 'low' | 'medium' | 'high';
export type ItemType = 'file' | 'directory' | 'registry' | 'plugin';

export interface CleanRule {
  name: string;
  category: string;
  paths: string[];
  patterns: string[];
  exclude: string[];
  risk: RiskLevel;
  min_age_days: number;
  default_on: boolean;
}

export interface ScanItem {
  id: string;
  path: string;
  name: string;
  type: ItemType;
  category: string;
  size: number;
  risk: RiskLevel;
  source: string;
  last_modified: number;
  selected: boolean;
  plugin?: PluginInfo;
  registry?: RegistryInfo;
}

export interface PluginInfo {
  browser: string;
  profile: string;
  extension_id: string;
  version: string;
  description: string;
  manifest_path: string;
}

export interface RegistryInfo {
  hive: string;
  key_path: string;
  value_name: string;
  value_type: string;
  raw_data: string;
  expanded_path: string;
  target_path: string;
  backup_path: string;
}

export interface ScanError {
  path: string;
  reason: string;
}

export interface ScanResult {
  items: ScanItem[];
  total_files: number;
  total_size: number;
  errors: ScanError[];
  duration_ms: number;
}

export type ScanPhase =
  | 'loading_rules'
  | 'scanning_files'
  | 'scanning_plugins'
  | 'scanning_registry'
  | 'done';

export interface ScanProgress {
  phase: ScanPhase;
  current_label: string;
  completed_steps: number;
  total_steps: number;
  found_items: number;
  failed_items: number;
  percent: number;
}

export interface CleanResult {
  deleted_files: number;
  freed_size: number;
  failed_files: string[];
  failed_reasons: Record<string, string>;
  message: string;
}

export interface QuarantineRecord {
  record_id: string;
  original_path: string;
  quarantine_path: string;
  name: string;
  item_type: string;
  browser: string;
  created_at: string;
  size: number;
  restored_at?: string;
}

export interface QuarantineResult {
  moved_items: number;
  restored_items: number;
  failed_items: string[];
  failed_reasons: Record<string, string>;
  message: string;
}

export interface RegistryActionResult {
  deleted_values: number;
  backup_path: string;
  failed_items: string[];
  failed_reasons: Record<string, string>;
  message: string;
}

export interface ShredRequest {
  path: string;
  passes: number;
}

export interface ShredResult {
  shredded_files: number;
  freed_size: number;
  failed_files: string[];
  failed_reasons: Record<string, string>;
  message: string;
}

export interface OperationLog {
  timestamp: string;
  operation: string;
  scanned_files: number;
  deleted_files: number;
  freed_size: number;
  failed_paths: string[];
  failed_reasons: string[];
  duration: number;
}

export const RiskLabels: Record<RiskLevel, string> = {
  low: '低风险',
  medium: '中风险',
  high: '高风险',
};

export const RiskColors: Record<RiskLevel, string> = {
  low: '#2f9d58',
  medium: '#b7791f',
  high: '#d64545',
};

export const CategoryLabels: Record<string, string> = {
  system: '系统清理',
  software: '软件缓存',
  privacy: '隐私痕迹',
  plugin: '插件扫描',
  registry: '注册表',
};
