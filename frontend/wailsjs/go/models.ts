export namespace model {

	export class CleanResult {
	    deleted_files: number;
	    freed_size: number;
	    failed_files: string[];
	    failed_reasons: Record<string, string>;
	    message: string;
	    warnings?: string[];

	    static createFrom(source: any = {}) {
	        return new CleanResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deleted_files = source["deleted_files"];
	        this.freed_size = source["freed_size"];
	        this.failed_files = source["failed_files"];
	        this.failed_reasons = source["failed_reasons"];
	        this.message = source["message"];
	        this.warnings = source["warnings"];
	    }
	}
	export class CleanRule {
	    name: string;
	    category: string;
	    paths: string[];
	    patterns: string[];
	    exclude: string[];
	    risk: string;
	    min_age_days: number;
	    default_on: boolean;

	    static createFrom(source: any = {}) {
	        return new CleanRule(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.category = source["category"];
	        this.paths = source["paths"];
	        this.patterns = source["patterns"];
	        this.exclude = source["exclude"];
	        this.risk = source["risk"];
	        this.min_age_days = source["min_age_days"];
	        this.default_on = source["default_on"];
	    }
	}
	export class OperationLog {
	    timestamp: string;
	    operation: string;
	    scanned_files: number;
	    deleted_files: number;
	    freed_size: number;
	    failed_paths: string[];
	    failed_reasons: string[];
	    duration: number;

	    static createFrom(source: any = {}) {
	        return new OperationLog(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.operation = source["operation"];
	        this.scanned_files = source["scanned_files"];
	        this.deleted_files = source["deleted_files"];
	        this.freed_size = source["freed_size"];
	        this.failed_paths = source["failed_paths"];
	        this.failed_reasons = source["failed_reasons"];
	        this.duration = source["duration"];
	    }
	}
	export class PluginInfo {
	    browser: string;
	    profile: string;
	    extension_id: string;
	    version: string;
	    description: string;
	    manifest_path: string;

	    static createFrom(source: any = {}) {
	        return new PluginInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.browser = source["browser"];
	        this.profile = source["profile"];
	        this.extension_id = source["extension_id"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.manifest_path = source["manifest_path"];
	    }
	}
	export class RegistryActionResult {
	    deleted_values: number;
	    backup_path: string;
	    failed_items: string[];
	    failed_reasons: Record<string, string>;
	    message: string;
	    warnings?: string[];

	    static createFrom(source: any = {}) {
	        return new RegistryActionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deleted_values = source["deleted_values"];
	        this.backup_path = source["backup_path"];
	        this.failed_items = source["failed_items"];
	        this.failed_reasons = source["failed_reasons"];
	        this.message = source["message"];
	        this.warnings = source["warnings"];
	    }
	}
	export class RegistryInfo {
	    hive: string;
	    key_path: string;
	    value_name: string;
	    value_type: string;
	    raw_data: string;
	    expanded_path: string;
	    target_path: string;
	    backup_path: string;

	    static createFrom(source: any = {}) {
	        return new RegistryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hive = source["hive"];
	        this.key_path = source["key_path"];
	        this.value_name = source["value_name"];
	        this.value_type = source["value_type"];
	        this.raw_data = source["raw_data"];
	        this.expanded_path = source["expanded_path"];
	        this.target_path = source["target_path"];
	        this.backup_path = source["backup_path"];
	    }
	}
	export class QuarantineRecord {
	    record_id: string;
	    original_path: string;
	    quarantine_path: string;
	    name: string;
	    item_type: string;
	    browser: string;
	    created_at: string;
	    size: number;
	    restored_at?: string;

	    static createFrom(source: any = {}) {
	        return new QuarantineRecord(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.record_id = source["record_id"];
	        this.original_path = source["original_path"];
	        this.quarantine_path = source["quarantine_path"];
	        this.name = source["name"];
	        this.item_type = source["item_type"];
	        this.browser = source["browser"];
	        this.created_at = source["created_at"];
	        this.size = source["size"];
	        this.restored_at = source["restored_at"];
	    }
	}
	export class QuarantineResult {
	    moved_items: number;
	    restored_items: number;
	    failed_items: string[];
	    failed_reasons: Record<string, string>;
	    message: string;
	    warnings?: string[];

	    static createFrom(source: any = {}) {
	        return new QuarantineResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.moved_items = source["moved_items"];
	        this.restored_items = source["restored_items"];
	        this.failed_items = source["failed_items"];
	        this.failed_reasons = source["failed_reasons"];
	        this.message = source["message"];
	        this.warnings = source["warnings"];
	    }
	}
	export class ScanError {
	    path: string;
	    reason: string;

	    static createFrom(source: any = {}) {
	        return new ScanError(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	    }
	}
	export class ScanItem {
	    id: string;
	    path: string;
	    name: string;
	    type: string;
	    category: string;
	    size: number;
	    risk: string;
	    source: string;
	    last_modified: number;
	    selected: boolean;
	    plugin?: PluginInfo;
	    registry?: RegistryInfo;

	    static createFrom(source: any = {}) {
	        return new ScanItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.category = source["category"];
	        this.size = source["size"];
	        this.risk = source["risk"];
	        this.source = source["source"];
	        this.last_modified = source["last_modified"];
	        this.selected = source["selected"];
	        this.plugin = this.convertValues(source["plugin"], PluginInfo);
	        this.registry = this.convertValues(source["registry"], RegistryInfo);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScanResult {
	    items: ScanItem[];
	    total_files: number;
	    total_size: number;
	    errors: ScanError[];
	    duration_ms: number;

	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], ScanItem);
	        this.total_files = source["total_files"];
	        this.total_size = source["total_size"];
	        this.errors = this.convertValues(source["errors"], ScanError);
	        this.duration_ms = source["duration_ms"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ShredRequest {
	    path: string;
	    passes: number;

	    static createFrom(source: any = {}) {
	        return new ShredRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.passes = source["passes"];
	    }
	}
	export class ShredResult {
	    shredded_files: number;
	    freed_size: number;
	    failed_files: string[];
	    failed_reasons: Record<string, string>;
	    message: string;
	    warnings?: string[];

	    static createFrom(source: any = {}) {
	        return new ShredResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shredded_files = source["shredded_files"];
	        this.freed_size = source["freed_size"];
	        this.failed_files = source["failed_files"];
	        this.failed_reasons = source["failed_reasons"];
	        this.message = source["message"];
	        this.warnings = source["warnings"];
	    }
	}

}
