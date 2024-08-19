import * as React from 'react';
import * as minimatch from 'minimatch';

import {Application, ApplicationTree, State} from '../models';

const extensions = {
    resourceExtensions: new Array<ResourceTabExtension>(),
    systemLevelExtensions: new Array<SystemLevelExtension>(),
    appViewExtensions: new Array<AppViewExtension>(),
    statusPanelExtensions: new Array<StatusPanelExtension>(),
    ActionMenuExtensions: new Array<ActionMenuExtension>()
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

function registerAppActionMenuPanelExtension(component: StatusPanelExtensionComponent, title: string, id: string,env: string, flyout?: ActionMenuExtFlyoutComponent) {
    extensions.ActionMenuExtensions.push({component, flyout, title, id, env});
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

export interface ActionMenuExtension {
    component: ActionMenuExtensionComponent;
    flyout: ActionMenuExtFlyoutComponent;
    title: string;
    id: string;
    env?: string;

}

export type ExtensionComponent = React.ComponentType<ExtensionComponentProps>;
export type SystemExtensionComponent = React.ComponentType;
export type AppViewExtensionComponent = React.ComponentType<AppViewComponentProps>;
export type StatusPanelExtensionComponent = React.ComponentType<BaseExtensionComponentProps>;
export type StatusPanelExtensionFlyoutComponent = React.ComponentType<BaseFlyoutProps>;
export type ActionMenuExtensionComponent = React.ComponentType<BaseExtensionComponentProps>;
export type ActionMenuExtFlyoutComponent = React.ComponentType<BaseFlyoutProps>;

export interface Extension {
    component: ExtensionComponent;
}

export interface ExtensionComponentProps {
    resource?: State;
    tree?: ApplicationTree;
    application?: Application;
}

export interface AppViewComponentProps {
    application: Application;
    tree: ApplicationTree;
}

export interface BaseExtensionComponentProps {
    application?: Application;
    tree?: ApplicationTree;
    openFlyout?: () => any;
    setExtComponentData?: (refresh: any) => void;
    extComponentData?: any
}


 export interface BaseFlyoutProps {
    application: Application;
    tree: ApplicationTree;
    setExtComponentData?: (refresh: any) => void;
    extComponentData?: any}

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

    public getActionMenuExtensions(): ActionMenuExtension[] {
        return extensions.ActionMenuExtensions.slice();
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