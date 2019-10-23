export function hashCode(str: string) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        // tslint:disable-next-line:no-bitwise
        hash = ~~((hash << 5) - hash + str.charCodeAt(i));
    }
    return hash;
}
