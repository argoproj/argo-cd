import {ApplicationTree, State} from '../models';
import * as React from 'react';

interface Context {
    state: any
    setState: (value: any) => void
}

type AppToolbar = {
    type: 'appToolbar';
    factory: (context: Context) => { title: string | React.ReactElement; action: () => any };
};

type AppPanel = {
    type: 'appPanel';
    factory: (context: Context) => { shown: boolean, onClose: () => void, component: React.Component };
};

type Extension = { [key: string]: AppToolbar | AppPanel };

const extensions: {
    // non-resource extensions (v2)
    items: {
        [name: string]: Extension;
    };
    // resource extensions (v1)
    resources: { [key: string]: ResourceExtension };
} = {
    items: {},
    resources: {}
};
const cache = new Map<string, Promise<ResourceExtension>>();

export interface ResourceExtension {
    component: React.ComponentType<ExtensionComponentProps>;
}

interface IndexEntry {
    name: string;
}

type Index = IndexEntry[];

export interface ExtensionComponentProps {
    resource: State;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public list(type: string, state: any, setState: (value: any) => void) {
        return Object.values(extensions.items)
            .map(x => Object.values(x))
            .reduce((previous, current) => previous.concat(current), [])
            .filter(x => x.type === type)
            .map(x => x.factory({state, setState}));
    }

    public async load(): Promise<IndexEntry> {
        return new Promise((resolve, reject) => {
            fetch('/extensions/index.json')
                .then(res => res.json())
                .then(json => json as Index)
                .then(index => {
                    index.forEach(extension => {
                        const script = document.createElement('script');
                        script.src = `extensions/${extension.name}.js`;
                        script.onload = () => resolve(extension);
                        script.onerror = e => {
                            reject(new Error(`failed to load ${extension.name} extension: ${e}`));
                        };
                        document.body.appendChild(script);
                    });
                }, reject);
        });
    }

    public async loadResourceExtension(group: string, kind: string): Promise<ResourceExtension> {
        const key = `${group}/${kind}`;
        const res =
            cache.get(key) ||
            new Promise<ResourceExtension>((resolve, reject) => {
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
