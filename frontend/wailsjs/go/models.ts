export namespace fileops {
	
	export class DiskUsage {
	    total: number;
	    free: number;
	    used: number;
	    usedPct: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.free = source["free"];
	        this.used = source["used"];
	        this.usedPct = source["usedPct"];
	    }
	}

}

export namespace index {
	
	export class Favorite {
	    id: number;
	    path: string;
	    label: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new Favorite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.label = source["label"];
	        this.sortOrder = source["sortOrder"];
	    }
	}
	export class FileEntry {
	    id: number;
	    path: string;
	    name: string;
	    parentPath: string;
	    extension: string;
	    size: number;
	    isDir: boolean;
	    modTime: number;
	    createTime: number;
	    permissions: number;
	    hidden: boolean;
	    indexedAt: number;
	    isSymlink: boolean;
	    symlinkTarget: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.parentPath = source["parentPath"];
	        this.extension = source["extension"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.modTime = source["modTime"];
	        this.createTime = source["createTime"];
	        this.permissions = source["permissions"];
	        this.hidden = source["hidden"];
	        this.indexedAt = source["indexedAt"];
	        this.isSymlink = source["isSymlink"];
	        this.symlinkTarget = source["symlinkTarget"];
	    }
	}
	export class ScanProgress {
	    scanned: number;
	    indexed: number;
	    current: string;
	    running: boolean;
	    startedAt: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanned = source["scanned"];
	        this.indexed = source["indexed"];
	        this.current = source["current"];
	        this.running = source["running"];
	        this.startedAt = source["startedAt"];
	        this.error = source["error"];
	    }
	}

}

export namespace main {
	
	export class ConnectionInfo {
	    id: string;
	    name: string;
	    protocol: string;
	    host: string;
	    port: number;
	    user: string;
	    basePath: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.basePath = source["basePath"];
	        this.active = source["active"];
	    }
	}
	export class DiffLine {
	    lineNum: number;
	    type: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lineNum = source["lineNum"];
	        this.type = source["type"];
	        this.text = source["text"];
	    }
	}
	export class FileInfoDetail {
	    path: string;
	    name: string;
	    size: number;
	    isDir: boolean;
	    permissions: number;
	    owner: string;
	    group: string;
	    created: number;
	    modified: number;
	    accessed: number;
	    isSymlink: boolean;
	    symlinkTarget: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfoDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.permissions = source["permissions"];
	        this.owner = source["owner"];
	        this.group = source["group"];
	        this.created = source["created"];
	        this.modified = source["modified"];
	        this.accessed = source["accessed"];
	        this.isSymlink = source["isSymlink"];
	        this.symlinkTarget = source["symlinkTarget"];
	    }
	}
	export class GrepResult {
	    path: string;
	    name: string;
	    line: number;
	    snippet: string;
	
	    static createFrom(source: any = {}) {
	        return new GrepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.line = source["line"];
	        this.snippet = source["snippet"];
	    }
	}
	export class RenameOp {
	    path: string;
	    newName: string;
	
	    static createFrom(source: any = {}) {
	        return new RenameOp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.newName = source["newName"];
	    }
	}
	export class SyncDiffEntry {
	    path: string;
	    name: string;
	    status: string;
	    isDir: boolean;
	    leftSize: number;
	    rightSize: number;
	    leftMod: number;
	    rightMod: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncDiffEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.isDir = source["isDir"];
	        this.leftSize = source["leftSize"];
	        this.rightSize = source["rightSize"];
	        this.leftMod = source["leftMod"];
	        this.rightMod = source["rightMod"];
	    }
	}

}

export namespace preview {
	
	export class FilePreview {
	    path: string;
	    name: string;
	    type: string;
	    mimeType: string;
	    content: string;
	    size: number;
	    lines: number;
	    truncated: boolean;
	    language: string;
	    metadata?: Record<string, string>;
	    entries?: string[];
	
	    static createFrom(source: any = {}) {
	        return new FilePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.mimeType = source["mimeType"];
	        this.content = source["content"];
	        this.size = source["size"];
	        this.lines = source["lines"];
	        this.truncated = source["truncated"];
	        this.language = source["language"];
	        this.metadata = source["metadata"];
	        this.entries = source["entries"];
	    }
	}

}

export namespace search {
	
	export class Result {
	    id: number;
	    path: string;
	    name: string;
	    extension: string;
	    size: number;
	    isDir: boolean;
	    modTime: number;
	    rank: number;
	    snippet: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.extension = source["extension"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.modTime = source["modTime"];
	        this.rank = source["rank"];
	        this.snippet = source["snippet"];
	    }
	}

}

