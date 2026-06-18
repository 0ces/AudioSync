export namespace engine {
	
	export class OutputDevice {
	    id: string;
	    name: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OutputDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.default = source["default"];
	    }
	}
	export class Source {
	    id: number;
	    name: string;
	    platform: string;
	    sampleRate: number;
	    peakL: number;
	    peakR: number;
	    volume: number;
	    muted: boolean;
	    solo: boolean;
	    health: string;
	    driftPpm: number;
	    conn: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.platform = source["platform"];
	        this.sampleRate = source["sampleRate"];
	        this.peakL = source["peakL"];
	        this.peakR = source["peakR"];
	        this.volume = source["volume"];
	        this.muted = source["muted"];
	        this.solo = source["solo"];
	        this.health = source["health"];
	        this.driftPpm = source["driftPpm"];
	        this.conn = source["conn"];
	    }
	}
	export class Snapshot {
	    role: string;
	    running: boolean;
	    listenAddr: string;
	    outputDevice: string;
	    outputDeviceId: string;
	    masterVolume: number;
	    prefillMs: number;
	    bufferMs: number;
	    sources: Source[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.running = source["running"];
	        this.listenAddr = source["listenAddr"];
	        this.outputDevice = source["outputDevice"];
	        this.outputDeviceId = source["outputDeviceId"];
	        this.masterVolume = source["masterVolume"];
	        this.prefillMs = source["prefillMs"];
	        this.bufferMs = source["bufferMs"];
	        this.sources = this.convertValues(source["sources"], Source);
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

