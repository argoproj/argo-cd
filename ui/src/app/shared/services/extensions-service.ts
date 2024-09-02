import * as React from 'react';
import * as minimatch from 'minimatch';

import {Application, ApplicationTree, State} from '../models';

type ExtensionsEventType = 'resource' | 'systemLevel' | 'appView' | 'statusPanel' | 'top-bar';
type ExtensionsType = ResourceTabExtension | SystemLevelExtension | AppViewExtension | StatusPanelExtension | TopBarActionMenuExt;

class ExtensionsEventTarget {
    private listeners: Map<ExtensionsEventType, Array<(extension: ExtensionsType) => void>> = new Map();

    addEventListener(eventName: ExtensionsEventType, listener: (extension: ExtensionsType) => void) {
        if (!this.listeners.has(eventName)) {
            this.listeners.set(eventName, []);
        }
        this.listeners.get(eventName)?.push(listener);
    }

    removeEventListener(eventName: ExtensionsEventType, listenerToRemove: (extension: ExtensionsType) => void) {
        const listeners = this.listeners.get(eventName);
        if (!listeners) return;

        const filteredListeners = listeners.filter(listener => listener !== listenerToRemove);
        this.listeners.set(eventName, filteredListeners);
    }

    emit(eventName: ExtensionsEventType, extension: ExtensionsType) {
        this.listeners.get(eventName)?.forEach(listener => listener(extension));
    }
}

const extensions = {
    eventTarget: new ExtensionsEventTarget(),
    resourceExtentions: new Array<ResourceTabExtension>(),
    systemLevelExtensions: new Array<SystemLevelExtension>(),
    appViewExtensions: new Array<AppViewExtension>(),
    statusPanelExtensions: new Array<StatusPanelExtension>(),
    topBarActionMenuExts: new Array<TopBarActionMenuExt>()
};

function registerResourceExtension(component: ExtensionComponent, group: string, kind: string, tabTitle: string, opts?: {icon: string}) {
    const ext = {component, group, kind, title: tabTitle, icon: opts?.icon};
    extensions.resourceExtentions.push(ext);
    extensions.eventTarget.emit('resource', ext);
}

function registerSystemLevelExtension(component: ExtensionComponent, title: string, path: string, icon: string) {
    const ext = {component, title, icon, path};
    extensions.systemLevelExtensions.push(ext);
    extensions.eventTarget.emit('systemLevel', ext);
}

function registerAppViewExtension(component: ExtensionComponent, title: string, icon: string) {
    const ext = {component, title, icon};
    extensions.appViewExtensions.push(ext);
    extensions.eventTarget.emit('appView', ext);
}

function registerStatusPanelExtension(component: StatusPanelExtensionComponent, title: string, id: string, flyout?: ExtensionComponent) {
    const ext = {component, flyout, title, id};
    extensions.statusPanelExtensions.push(ext);
    extensions.eventTarget.emit('statusPanel', ext);
}

function registerTopBarActionMenuExt(
    component: TopBarActionMenuExtComponent,
    title: string,
    id: string,
    flyout: ExtensionComponent,
    shouldDisplay: (app?: Application) => boolean = () => true,
    iconClassName?: string,
    isMiddle = false
) {
    const ext = {component, flyout, shouldDisplay, title, id, iconClassName, isMiddle};
    extensions.topBarActionMenuExts.push(ext);
    extensions.eventTarget.emit('top-bar', ext);
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

export interface TopBarActionMenuExt {
    component: TopBarActionMenuExtComponent;
    flyout: TopBarActionMenuExtFlyoutComponent;
    shouldDisplay: (app: Application) => boolean;
    title: string;
    id: string;
    iconClassName?: string;
    isMiddle?: boolean;
    isNarrow?: boolean;
}

export type ExtensionComponent = React.ComponentType<ExtensionComponentProps>;
export type SystemExtensionComponent = React.ComponentType;
export type AppViewExtensionComponent = React.ComponentType<AppViewComponentProps>;
export type StatusPanelExtensionComponent = React.ComponentType<StatusPanelComponentProps>;
export type StatusPanelExtensionFlyoutComponent = React.ComponentType<StatusPanelFlyoutProps>;
export type TopBarActionMenuExtComponent = React.ComponentType<TopBarActionMenuExtComponentProps>;
export type TopBarActionMenuExtFlyoutComponent = React.ComponentType<TopBarActionMenuExtFlyoutProps>;

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
    openFlyout: () => any;
}

export interface TopBarActionMenuExtComponentProps {
    application: Application;
    tree: ApplicationTree;
    openFlyout: () => any;
}

export interface StatusPanelFlyoutProps {
    application: Application;
    tree: ApplicationTree;
}

export interface TopBarActionMenuExtFlyoutProps {
    application: Application;
    tree: ApplicationTree;
}

export class ExtensionsService {
    public addEventListener(evtType: ExtensionsEventType, cb: (ext: ExtensionsType) => void) {
        extensions.eventTarget.addEventListener(evtType, cb);
    }

    public removeEventListener(evtType: ExtensionsEventType, cb: (ext: ExtensionsType) => void) {
        extensions.eventTarget.removeEventListener(evtType, cb);
    }

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
    public getActionMenuExtensions(): TopBarActionMenuExt[] {
        return extensions.topBarActionMenuExts.slice();
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
        registerTopBarActionMenuExt
    };
})(window);
