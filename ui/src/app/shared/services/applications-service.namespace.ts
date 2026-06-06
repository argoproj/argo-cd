/** Application API uses appNamespace; ApplicationSet API uses appsetNamespace (appSetNamespace for watch stream). */
export function namespaceQueryKey(objectListKind: string, forWatch = false): string {
    return objectListKind === 'application' ? 'appNamespace' : forWatch ? 'appSetNamespace' : 'appsetNamespace';
}

export function namespaceQuery(objectListKind: string, namespace: string, forWatch = false): {[key: string]: string} {
    if (!namespace) {
        return {};
    }
    return {[namespaceQueryKey(objectListKind, forWatch)]: namespace};
}
