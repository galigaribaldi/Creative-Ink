export namespace config {
	
	export class Entry {
	    id: string;
	    type: string;
	    default: any;
	    scopes: string[];
	    description: string;
	    value: any;
	    scope: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.default = source["default"];
	        this.scopes = source["scopes"];
	        this.description = source["description"];
	        this.value = source["value"];
	        this.scope = source["scope"];
	    }
	}

}

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
	export class OutputChunk {
	    runId: number;
	    data: string;
	    offset: number;
	    next: number;
	    size: number;
	    done: boolean;
	    dropped: number;
	
	    static createFrom(source: any = {}) {
	        return new OutputChunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.data = source["data"];
	        this.offset = source["offset"];
	        this.next = source["next"];
	        this.size = source["size"];
	        this.done = source["done"];
	        this.dropped = source["dropped"];
	    }
	}
	export class ProjectWarning {
	    kind: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.message = source["message"];
	    }
	}
	export class ProjectHealth {
	    folder: string;
	    warnings: ProjectWarning[];
	    files: number;
	    texFiles: number;
	    largestFile: string;
	    largestBytes: number;
	    largestLines: number;
	    imageCalls: number;
	    imageBytes: number;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProjectHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.warnings = this.convertValues(source["warnings"], ProjectWarning);
	        this.files = source["files"];
	        this.texFiles = source["texFiles"];
	        this.largestFile = source["largestFile"];
	        this.largestBytes = source["largestBytes"];
	        this.largestLines = source["largestLines"];
	        this.imageCalls = source["imageCalls"];
	        this.imageBytes = source["imageBytes"];
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
	export class ProjectInfo {
	    name: string;
	    path: string;
	    mainFile: string;
	    // Go type: time
	    modified: any;
	
	    static createFrom(source: any = {}) {
	        return new ProjectInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.mainFile = source["mainFile"];
	        this.modified = this.convertValues(source["modified"], null);
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
	export class StartScreen {
	    repositoryPath: string;
	    projects: core.ProjectInfo[];
	    quickDocs: core.ProjectInfo[];
	    staleQuickDocs: core.ProjectInfo[];
	    staleAfterHours: number;
	    recent: string[];
	    pluginsPath: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new StartScreen(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repositoryPath = source["repositoryPath"];
	        this.projects = this.convertValues(source["projects"], core.ProjectInfo);
	        this.quickDocs = this.convertValues(source["quickDocs"], core.ProjectInfo);
	        this.staleQuickDocs = this.convertValues(source["staleQuickDocs"], core.ProjectInfo);
	        this.staleAfterHours = source["staleAfterHours"];
	        this.recent = source["recent"];
	        this.pluginsPath = source["pluginsPath"];
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
	    runId: number;
	
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
	        this.runId = source["runId"];
	    }
	}
	export class WorkspaceState {
	    needsSetup: boolean;
	    needsStart: boolean;
	    repositoryPath: string;
	    canceled: boolean;
	    folderPath: string;
	    filePath: string;
	    texFilePath: string;
	    mainFilePath: string;
	    autoCompile: boolean;
	    tree: core.FileNode;
	    truncated: boolean;
	    trustStatus: string;
	    trustFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.needsSetup = source["needsSetup"];
	        this.needsStart = source["needsStart"];
	        this.repositoryPath = source["repositoryPath"];
	        this.canceled = source["canceled"];
	        this.folderPath = source["folderPath"];
	        this.filePath = source["filePath"];
	        this.texFilePath = source["texFilePath"];
	        this.mainFilePath = source["mainFilePath"];
	        this.autoCompile = source["autoCompile"];
	        this.tree = this.convertValues(source["tree"], core.FileNode);
	        this.truncated = source["truncated"];
	        this.trustStatus = source["trustStatus"];
	        this.trustFiles = source["trustFiles"];
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

