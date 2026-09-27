// Invalid regex returns a never-match so the empty state surfaces the broken pattern to the user.
export function createMatcher(search: string, useRegex: boolean): (name: string, namespace: string) => boolean {
    if (search === '') {
        return () => true;
    }
    if (useRegex) {
        let re: RegExp;
        try {
            re = new RegExp(search);
        } catch {
            return () => false;
        }
        return (name, namespace) => re.test(name) || re.test(namespace);
    }
    return (name, namespace) => name.includes(search) || namespace.includes(search);
}
