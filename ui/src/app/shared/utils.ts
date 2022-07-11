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
