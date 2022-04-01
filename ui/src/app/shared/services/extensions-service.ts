import {Application, ApplicationTree, State} from '../models';
import * as React from 'react';
import {ReactElement} from 'react';

interface IndexEntry {
    name: string;
}

type Index = {
    items: IndexEntry[];
};

type AppToolbarButton = (props: {
    application: Application;
    openPanel: () => void;
}) => {
    iconClassName: string;
    title: ReactElement;
    action: () => void;
};

type AppStatusPanelItem = (props: {application: Application; openPanel: () => void}) => ReactElement;

type AppPanel = (props: {application: Application}) => ReactElement;

type ResourcePanel = (props: {}) => {
    iconClassName: string;
    title: string;
    component: ReactElement;
};

export type Extension = {
    AppPanel?: AppPanel;
    AppToolbarButton?: AppToolbarButton;
    ResourcePanel?: ResourcePanel;
    AppStatusPanelItem?: AppStatusPanelItem;
};

const extensions: {
    // non-resource extensions (v2)
    items: {
        [name: string]: Extension;
    };
    // resource extensions (v1)
    resources: {[key: string]: ResourceExtension};
} = {
    items: {},
    resources: {}
};
const cache = new Map<string, Promise<any>>();

export interface ResourceExtension {
    component: React.ComponentType<ResourceExtensionComponentProps>;
}

export interface ResourceExtensionComponentProps {
    application: Application;
    resource: State;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public list() {
        return extensions.items;
    }

    public load(): Promise<any[]> {
        const key = 'index';
        const res =
            cache.get(key) ||
            fetch('/extensions/index.json')
                .then(r => r.json())
                .then(j => j as Index)
                .then(index =>
                    Promise.all(
                        index.items.map(
                            item =>
                                new Promise<IndexEntry>((resolve, reject) => {
                                    const script = document.createElement('script');
                                    script.src = `extensions/${item.name}.js`;
                                    script.onload = () => resolve(item);
                                    script.onerror = e => reject(new Error(`failed to load ${item.name} extension: ${e}`));
                                    document.body.appendChild(script);
                                })
                        )
                    )
                );
        cache.set(key, res);
        return res;
    }

    public async loadResourceExtension(group: string, kind: string): Promise<ResourceExtension> {
        const key = `resources/${group}/${kind}`;
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
