import * as React from 'react';
import * as minimatch from 'minimatch';

import {Application, ApplicationSet, ApplicationTree, State} from '../models';

const extensions = {
    resourceExtentions: new Array<ResourceTabExtension>(),
    // appSetResourceExtentions: new Array<AppSetResourceTabExtension>(),
    systemLevelExtensions: new Array<SystemLevelExtension>(),
    appViewExtensions: new Array<AppViewExtension>(),
    // appSetViewExtensions: new Array<AppSetViewExtension>(),
    statusPanelExtensions: new Array<StatusPanelExtension>()
};

function registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string, opts?: {icon: string}) {
    extensions.resourceExtentions.push({component, group, kind, title: tabTitle, icon: opts?.icon});
}

function registerSystemLevelExtension(component: ExtensionComponent, title: string, path: string, icon: string) {
    extensions.systemLevelExtensions.push({component, title, icon, path});
}

function registerAppViewExtension(component: ExtensionComponent, title: string, icon: string) {
    extensions.appViewExtensions.push({component, title, icon});
}

// function registerAppSetViewExtension(component: AppSetExtensionComponent, title: string, icon: string) {
//     extensions.appSetViewExtensions.push({component, title, icon});
// }

function registerStatusPanelExtension(component: StatusPanelExtensionComponent, title: string, id: string, flyout?: ExtensionComponent) {
    extensions.statusPanelExtensions.push({component, flyout, title, id});
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

// export interface AppSetResourceTabExtension {
//     title: string;
//     group: string;
//     kind: string;
//     component: AppSetExtensionComponent;
//     icon?: string;
// }

export interface SystemLevelExtension {
    title: string;
    component: SystemExtensionComponent;
    icon?: string;
    path?: string;
}

export interface AppViewExtension {
    component: AppViewExtensionComponent;
    title: string;
    icon?: string;
}

// export interface AppSetViewExtension {
//     component: AppSetViewExtensionComponent;
//     title: string;
//     icon?: string;
// }

export interface StatusPanelExtension {
    component: StatusPanelExtensionComponent;
    flyout?: StatusPanelExtensionFlyoutComponent;
    title: string;
    id: string;
}

export type ExtensionComponent = React.ComponentType<AbstractExtensionComponentProps>;
// export type AppSetExtensionComponent = React.ComponentType<AppSetExtensionComponentProps>;
export type SystemExtensionComponent = React.ComponentType;
export type AppViewExtensionComponent = React.ComponentType<AbstractViewComponentProps>;
// export type AppSetViewExtensionComponent = React.ComponentType<AppSetViewComponentProps>;
export type StatusPanelExtensionComponent = React.ComponentType<StatusPanelComponentProps>;
export type StatusPanelExtensionFlyoutComponent = React.ComponentType<StatusPanelFlyoutProps>;

export interface Extension {
    component: ExtensionComponent;
}

// export interface AppSetExtension {
//     component: AppSetExtensionComponent;
// }

export interface AbstractExtensionComponentProps {
    resource: State;
    tree: ApplicationTree;
    application: ApplicationSet | Application;
}

export interface ExtensionComponentProps extends AbstractExtensionComponentProps {
    application: Application;
}

export interface AppSetExtensionComponentProps extends AbstractExtensionComponentProps {
    application: ApplicationSet;
}

export interface ViewComponentProps extends AbstractViewComponentProps {
    application: Application;
}

export interface AbstractViewComponentProps {
    application: ApplicationSet | Application;
    tree: ApplicationTree;
}

export interface AppSetViewComponentProps {
    application: ApplicationSet;
}

export interface StatusPanelComponentProps {
    application: Application;
    openFlyout: () => any;
}

export interface StatusPanelFlyoutProps {
    application: ApplicationSet | Application;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public getResourceTabs(group: string, kind: string): ResourceTabExtension[] {
        initLegacyExtensions();
        const items = extensions.resourceExtentions.filter(extension => minimatch(group, extension.group) && minimatch(kind, extension.kind)).slice();
        return items.sort((a, b) => a.title.localeCompare(b.title));
    }

    // public getAppSetResourceTabs(group: string, kind: string): AppSetResourceTabExtension[] {
    //     initLegacyExtensions();
    //     const items = extensions.appSetResourceExtentions.filter(extension => minimatch(group, extension.group) && minimatch(kind, extension.kind)).slice();
    //     return items.sort((a, b) => a.title.localeCompare(b.title));
    // }

    public getSystemExtensions(): SystemLevelExtension[] {
        return extensions.systemLevelExtensions.slice();
    }

    public getAppViewExtensions(): AppViewExtension[] {
        return extensions.appViewExtensions.slice();
    }

    // public getAppSetViewExtensions(): AppSetViewExtension[] {
    //     return extensions.appSetViewExtensions.slice();
    // }

    public getStatusPanelExtensions(): StatusPanelExtension[] {
        return extensions.statusPanelExtensions.slice();
    }
}

((window: any) => {
    // deprecated: kept for backwards compatibility
    window.extensions = {resources: {}};
    window.extensionsAPI = {
        registerResourceExtension,
        registerSystemLevelExtension,
        registerAppViewExtension
    };
})(window);

export class AppSetExtensionsService {
    public getResourceTabs(group: string, kind: string): ResourceTabExtension[] {
        initLegacyExtensions();
        const items = extensions.resourceExtentions.filter(extension => minimatch(group, extension.group) && minimatch(kind, extension.kind)).slice();
        return items.sort((a, b) => a.title.localeCompare(b.title));
    }

    public getSystemExtensions(): SystemLevelExtension[] {
        return extensions.systemLevelExtensions.slice();
    }

    public getAppViewExtensions(): AppViewExtension[] {
        return extensions.appViewExtensions.slice();
    }

    public getStatusPanelExtensions(): StatusPanelExtension[] {
        return extensions.statusPanelExtensions.slice();
    }
}

((window: any) => {
    // deprecated: kept for backwards compatibility
    window.extensions = {resources: {}};
    window.extensionsAPI = {
        registerResourceExtension,
        registerSystemLevelExtension,
        registerAppViewExtension,
        registerStatusPanelExtension
    };
})(window);
