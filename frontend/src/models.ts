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

export const RiskLabels: Record<RiskLevel, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
};

export const RiskColors: Record<RiskLevel, string> = {
  low: '#2f9d58',
  medium: '#b7791f',
  high: '#d64545',
};

export const CategoryLabels: Record<string, string> = {
  system: 'System',
  software: 'Software',
  privacy: 'Privacy',
  plugin: 'Plugin',
};
