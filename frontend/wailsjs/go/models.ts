export namespace main {

	export class installState {
	    selected: string;
	    installed: string;
	    version: string;
	    gameFound: boolean;

	    static createFrom(source: any = {}) {
	        return new installState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selected = source["selected"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.gameFound = source["gameFound"];
	    }
	}

}
