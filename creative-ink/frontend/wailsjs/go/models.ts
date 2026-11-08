export namespace core {
	
	export class FileNode {
	    name: string;
	    path: string;
	    isDir: boolean;
	    editable: boolean;
	    children?: FileNode[];
	
	    static createFrom(source: any = {}) {
	        return new FileNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.editable = source["editable"];
	        this.children = this.convertValues(source["children"], FileNode);
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

export namespace main {
	
	export class OpenedFile {
	    path: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	    }
	}
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
	export class SyncTexBackwardResult {
	    found: boolean;
	    file: string;
	    line: number;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncTexBackwardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.reason = source["reason"];
	    }
	}
	export class SyncTexForwardResult {
	    found: boolean;
	    page: number;
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncTexForwardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.page = source["page"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.reason = source["reason"];
	    }
	}
	export class TerminalResult {
	    command: string;
	    folder: string;
	    output: string;
	    exitCode: number;
	    timedOut: boolean;
	    canceled: boolean;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TerminalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.folder = source["folder"];
	        this.output = source["output"];
	        this.exitCode = source["exitCode"];
	        this.timedOut = source["timedOut"];
	        this.canceled = source["canceled"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class WorkspaceState {
	    needsSetup: boolean;
	    canceled: boolean;
	    folderPath: string;
	    filePath: string;
	    mainFilePath: string;
	    autoCompile: boolean;
	    tree: core.FileNode;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.needsSetup = source["needsSetup"];
	        this.canceled = source["canceled"];
	        this.folderPath = source["folderPath"];
	        this.filePath = source["filePath"];
	        this.mainFilePath = source["mainFilePath"];
	        this.autoCompile = source["autoCompile"];
	        this.tree = this.convertValues(source["tree"], core.FileNode);
	        this.truncated = source["truncated"];
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

export namespace system {
	
	export class RepositoryOption {
	    scope: string;
	    path: string;
	    available: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new RepositoryOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scope = source["scope"];
	        this.path = source["path"];
	        this.available = source["available"];
	        this.reason = source["reason"];
	    }
	}
	export class RepositoryOptions {
	    os: string;
	    osName: string;
	    user: RepositoryOption;
	    shared: RepositoryOption;
	    warnings: string[];
	    currentScope: string;
	    currentPath: string;
	
	    static createFrom(source: any = {}) {
	        return new RepositoryOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.osName = source["osName"];
	        this.user = this.convertValues(source["user"], RepositoryOption);
	        this.shared = this.convertValues(source["shared"], RepositoryOption);
	        this.warnings = source["warnings"];
	        this.currentScope = source["currentScope"];
	        this.currentPath = source["currentPath"];
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
	export class HealthStatus {
	    hasXeLaTeX: boolean;
	    hasLatexmk: boolean;
	    hasBiber: boolean;
	    totalRAMGB: number;
	    cpuCores: number;
	    freeDiskSpaceGB: number;
	    hardwareTier: string;
	    hardwareTierId: string;
	    repository: RepositoryOptions;
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
	        this.repository = this.convertValues(source["repository"], RepositoryOptions);
	        this.warnings = source["warnings"];
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

