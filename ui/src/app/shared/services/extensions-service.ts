import * as React from 'react';
import {ApplicationTree, State} from '../models';

const extensions: {resources: {[key: string]: Extension}} = {resources: {}};
const cache = new Map<string, Promise<Extension>>();

export interface Extension {
    component: React.ComponentType<ExtensionComponentProps>;
}

export interface ExtensionComponentProps {
    resource: State;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public async loadResourceExtension(group: string, kind: string): Promise<Extension> {
        const key = `${group}/${kind}`;
        const res =
            cache.get(key) ||
            new Promise<Extension>((resolve, reject) => {
                const script = document.createElement('script');
                script.src = `extensions/resources/${group}/${kind}/ui/extensions.js`;
                script.onload = () => {
                    const ext = extensions.resources[key];
                    if (!ext) {
                        reject(`Failed to load extension for ${group}/${kind}`);
                    } else {
                        resolve(ext);
                    }
                };
                script.onerror = reject;
                document.body.appendChild(script);
            });
        cache.set(key, res);
        return res;
    }
}

(window as any).extensions = extensions;
