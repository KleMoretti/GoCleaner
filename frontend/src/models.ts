// TypeScript models matching the Go backend data structures.

export interface CleanRule {
  name: string;
  category: string;
  paths: string[];
  patterns: string[];
  exclude: string[];
  risk: 'low' | 'medium' | 'high';
  min_age_days: number;
  default_on: boolean;
}

export interface ScanItem {
  id: string;
  path: string;
  name: string;
  type: 'file' | 'directory' | 'registry' | 'plugin';
  category: string;
  size: number;
  risk: 'low' | 'medium' | 'high';
  source: string;
  last_modified: number;
  selected: boolean;
}

export interface CleanResult {
  deleted_files: number;
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

export const RiskLabels: Record<string, string> = {
  low: '低风险',
  medium: '中风险',
  high: '高风险',
};

export const RiskColors: Record<string, string> = {
  low: '#52c41a',
  medium: '#faad14',
  high: '#f5222d',
};

export const CategoryLabels: Record<string, string> = {
  system: '系统垃圾',
  software: '软件缓存',
  privacy: '隐私痕迹',
  plugin: '浏览器插件',
};
