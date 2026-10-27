export namespace main {
	
	export class DeleteResult {
	    deleted: string[];
	    failed: string[];
	    freed: number;
	
	    static createFrom(source: any = {}) {
	        return new DeleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deleted = source["deleted"];
	        this.failed = source["failed"];
	        this.freed = source["freed"];
	    }
	}
	export class ExplorerStatus {
	    ncdu: string;
	    scanned: boolean;
	    // Go type: time
	    scannedAt: any;
	    seconds: number;
	    total: number;
	    items: number;
	    freed: number;
	
	    static createFrom(source: any = {}) {
	        return new ExplorerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ncdu = source["ncdu"];
	        this.scanned = source["scanned"];
	        this.scannedAt = this.convertValues(source["scannedAt"], null);
	        this.seconds = source["seconds"];
	        this.total = source["total"];
	        this.items = source["items"];
	        this.freed = source["freed"];
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
	export class Report {
	    distro: wsl.Distro;
	    usage?: wsl.Usage;
	    reclaimable: number;
	    trimmed?: number;
	    snapshots: wsl.Snapshot[];
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.distro = this.convertValues(source["distro"], wsl.Distro);
	        this.usage = this.convertValues(source["usage"], wsl.Usage);
	        this.reclaimable = source["reclaimable"];
	        this.trimmed = source["trimmed"];
	        this.snapshots = this.convertValues(source["snapshots"], wsl.Snapshot);
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

export namespace wsl {
	
	export class Cleanup {
	    id: string;
	    key: string;
	    path: string;
	    size: number;
	    category: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new Cleanup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.key = source["key"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.category = source["category"];
	        this.command = source["command"];
	    }
	}
	export class Distro {
	    id: string;
	    name: string;
	    version: number;
	    state: string;
	    default: boolean;
	    basePath: string;
	    vhdPath: string;
	    fileSize: number;
	    sizeOnDisk: number;
	    sparse: boolean;
	    defaultUid: number;
	    managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Distro(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.state = source["state"];
	        this.default = source["default"];
	        this.basePath = source["basePath"];
	        this.vhdPath = source["vhdPath"];
	        this.fileSize = source["fileSize"];
	        this.sizeOnDisk = source["sizeOnDisk"];
	        this.sparse = source["sparse"];
	        this.defaultUid = source["defaultUid"];
	        this.managed = source["managed"];
	    }
	}
	export class FsNode {
	    name: string;
	    path: string;
	    size: number;
	    items: number;
	    kind: string;
	    count?: number;
	    category: string;
	    protected: boolean;
	    readError: boolean;
	    children?: FsNode[];
	
	    static createFrom(source: any = {}) {
	        return new FsNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.items = source["items"];
	        this.kind = source["kind"];
	        this.count = source["count"];
	        this.category = source["category"];
	        this.protected = source["protected"];
	        this.readError = source["readError"];
	        this.children = this.convertValues(source["children"], FsNode);
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
	export class FsView {
	    node: FsNode;
	    categories: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new FsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], FsNode);
	        this.categories = source["categories"];
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
	export class Platform {
	    wslVersion: string;
	    nativeCompact: boolean;
	    nativeSparse: boolean;
	    optimizeVhd: boolean;
	    isAdmin: boolean;
	    wslConfigPath: string;
	    wslConfigSparse: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Platform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.wslVersion = source["wslVersion"];
	        this.nativeCompact = source["nativeCompact"];
	        this.nativeSparse = source["nativeSparse"];
	        this.optimizeVhd = source["optimizeVhd"];
	        this.isAdmin = source["isAdmin"];
	        this.wslConfigPath = source["wslConfigPath"];
	        this.wslConfigSparse = source["wslConfigSparse"];
	    }
	}
	export class Snapshot {
	    step: string;
	    label: string;
	    // Go type: time
	    at: any;
	    fileSize: number;
	    sizeOnDisk: number;
	    used: number;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.step = source["step"];
	        this.label = source["label"];
	        this.at = this.convertValues(source["at"], null);
	        this.fileSize = source["fileSize"];
	        this.sizeOnDisk = source["sizeOnDisk"];
	        this.used = source["used"];
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
	export class Usage {
	    total: number;
	    used: number;
	    avail: number;
	
	    static createFrom(source: any = {}) {
	        return new Usage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.used = source["used"];
	        this.avail = source["avail"];
	    }
	}

}

