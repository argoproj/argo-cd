import * as React from 'react';
import * as minimatch from 'minimatch';

import {Application, ApplicationTree, State} from '../models';

const extensions = {
    resourceExtensions: new Array<ResourceTabExtension>(),
    systemLevelExtensions: new Array<SystemLevelExtension>(),
    appViewExtensions: new Array<AppViewExtension>(),
    statusPanelExtensions: new Array<StatusPanelExtension>(),
    AppActionMenuExtensions: new Array<AppActionMenuExtension>()
};

function registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string, opts?: {icon: string}) {
    extensions.resourceExtensions.push({component, group, kind, title: tabTitle, icon: opts?.icon});
}

function registerSystemLevelExtension(component: SystemExtensionComponent, title: string, path: string, icon: string) {
    extensions.systemLevelExtensions.push({component, title, icon, path});
}

function registerAppViewExtension(component: AppViewExtensionComponent, title: string, icon: string) {
    extensions.appViewExtensions.push({component, title, icon});
}

function registerStatusPanelExtension(component: StatusPanelExtensionComponent, title: string, id: string, flyout?: StatusPanelExtensionFlyoutComponent) {
    extensions.statusPanelExtensions.push({component, flyout, title, id});
}

function registerAppActionMenuPanelExtension(component: StatusPanelExtensionComponent, title: string, id: string, flyout?: AppActionMenuExtensionExtensionFlyoutComponent) {
    extensions.AppActionMenuExtensions.push({component, flyout, title, id});
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

export interface StatusPanelExtension {
    component: StatusPanelExtensionComponent;
    flyout?: StatusPanelExtensionFlyoutComponent;
    title: string;
    id: string;
}

export interface AppActionMenuExtension {
    component: AppActionMenuExtensionComponent;
    flyout: AppActionMenuExtensionExtensionFlyoutComponent;
    title: string;
    id: string;
}

export type ExtensionComponent = React.ComponentType<ExtensionComponentProps>;
export type SystemExtensionComponent = React.ComponentType;
export type AppViewExtensionComponent = React.ComponentType<AppViewComponentProps>;
export type StatusPanelExtensionComponent = React.ComponentType<StatusPanelComponentProps>;
export type StatusPanelExtensionFlyoutComponent = React.ComponentType<StatusPanelFlyoutProps>;
export type AppActionMenuExtensionComponent = React.ComponentType<AppActionMenuComponentProps>;
export type AppActionMenuExtensionExtensionFlyoutComponent = React.ComponentType<AppActionMenuFlyoutProps>;


export interface Extension {
    component: ExtensionComponent;
}

export interface ExtensionComponentProps {
    resource: State;
    tree: ApplicationTree;
    application: Application;
}

export interface AppViewComponentProps {
    application: Application;
    tree: ApplicationTree;
}

export interface StatusPanelComponentProps {
    application: Application;
}

export interface StatusPanelFlyoutProps {
    application: Application;
    openFlyout?: () => any;}

export interface AppActionMenuComponentProps {
    application?: Application;
    openFlyout?: () => any;
}

export interface AppActionMenuFlyoutProps {
    application: Application;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public getResourceTabs(group: string, kind: string): ResourceTabExtension[] {
        initLegacyExtensions();
        const items = extensions.resourceExtensions.filter(extension => minimatch(group, extension.group) && minimatch(kind, extension.kind)).slice();
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

    public getAppActionMenuExtensions(): AppActionMenuExtension[] {
        return extensions.AppActionMenuExtensions.slice();
    }
}

((window: any) => {
    // deprecated: kept for backwards compatibility
    window.extensions = {resources: {}};
    window.extensionsAPI = {
        registerResourceExtension,
        registerSystemLevelExtension,
        registerAppViewExtension,
        registerStatusPanelExtension,
        registerAppActionMenuPanelExtension,
    };
})(window);