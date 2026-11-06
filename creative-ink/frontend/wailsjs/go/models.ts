export namespace main {
	
	export class SaveResult {
	    saved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = source["saved"];
	    }
	}

}

export namespace system {
	
	export class HealthStatus {
	    hasXeLaTeX: boolean;
	    hasLatexmk: boolean;
	    hasBiber: boolean;
	    totalRAMGB: number;
	    cpuCores: number;
	    freeDiskSpaceGB: number;
	    hardwareTier: string;
	    hardwareTierId: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new HealthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasXeLaTeX = source["hasXeLaTeX"];
	        this.hasLatexmk = source["hasLatexmk"];
	        this.hasBiber = source["hasBiber"];
	        this.totalRAMGB = source["totalRAMGB"];
	        this.cpuCores = source["cpuCores"];
	        this.freeDiskSpaceGB = source["freeDiskSpaceGB"];
	        this.hardwareTier = source["hardwareTier"];
	        this.hardwareTierId = source["hardwareTierId"];
	        this.warnings = source["warnings"];
	    }
	}

}

