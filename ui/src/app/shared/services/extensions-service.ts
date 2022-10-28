import * as React from 'react';
import * as minimatch from 'minimatch';

import {Application, ApplicationTree, State} from '../models';

const extensions = {
    resourceExtentions: new Array<ResourceTabExtension>()
};

function registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string, opts?: {icon: string}) {
    extensions.resourceExtentions.push({component, group, kind, title: tabTitle, icon: opts?.icon});
}

let legacyInitialized = false;

function initLegacyExtensions() {
    if (legacyInitialized) {
        return;
    }
    legacyInitialized = true;
    const resources = (window as any).extensions.resources;
    Object.keys(resources).forEach(key => {
        const [group, kind] = key.split('/');
        registerResourceExtension(resources[key].component, group, kind, 'More');
    });
}

export interface ResourceTabExtension {
    title: string;
    group: string;
    kind: string;
    component: ExtensionComponent;
    icon?: string;
}

export type ExtensionComponent = React.ComponentType<ExtensionComponentProps>;

export interface Extension {
    component: ExtensionComponent;
}

export interface ExtensionComponentProps {
    resource: State;
    tree: ApplicationTree;
    application: Application;
}

export class ExtensionsService {
    public getResourceTabs(group: string, kind: string): ResourceTabExtension[] {
        initLegacyExtensions();
        const items = extensions.resourceExtentions.filter(extension => minimatch(group, extension.group) && minimatch(kind, extension.kind)).slice();
        return items.sort((a, b) => a.title.localeCompare(b.title));
    }
}

((window: any) => {
    // deprecated: kept for backwards compatibility
    window.extensions = {resources: {}};
    window.extensionsAPI = {
        registerResourceExtension
    };
})(window);
