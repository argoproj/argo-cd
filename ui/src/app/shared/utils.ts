export function hashCode(str: string) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        // tslint:disable-next-line:no-bitwise
        hash = ~~((hash << 5) - hash + str.charCodeAt(i));
    }
    return hash;
}

// concatMaps merges two maps. Later args take precedence where there's a key conflict.
export function concatMaps(...maps: (Map<string, string> | null)[]): Map<string, string> {
    const newMap = new Map<string, string>();
    for (const map of maps) {
        if (map) {
            for (const entry of Object.entries(map)) {
                newMap.set(entry[0], entry[1]);
            }
        }
    }
    return newMap;
}

export function isValidURL(url: string): boolean {
    try {
        const parsedUrl = new URL(url);
        return parsedUrl.protocol !== 'javascript:' && parsedUrl.protocol !== 'data:' && parsedUrl.protocol !== 'vbscript:';
    } catch (TypeError) {
        try {
            // Try parsing as a relative URL.
            const parsedUrl = new URL(url, window.location.origin);
            return parsedUrl.protocol !== 'javascript:' && parsedUrl.protocol !== 'data:' && parsedUrl.protocol !== 'vbscript:';
        } catch (TypeError) {
            return false;
        }
    }
}
