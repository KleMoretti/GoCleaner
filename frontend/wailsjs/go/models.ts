export namespace model {
	
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

}
