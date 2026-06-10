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

}

