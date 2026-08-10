import * as React from 'react';
import * as AppUtils from '../../applications/components/utils';
import {ContextApis} from '../../shared/context';
import * as models from '../../shared/models';
import {HealthStatuses} from '../../shared/models';

/** Resources without health status are treated as Healthy (matches application list behavior). */
export function resourceHealthStatus(resource: models.Resource): models.HealthStatusCode {
    return resource.health?.status || HealthStatuses.Healthy;
}

export function resourceHealthState(resource: models.Resource): models.HealthStatus {
    if (resource.health?.status) {
        return resource.health;
    }
    return {status: HealthStatuses.Healthy, message: ''};
}

/** URL for an Application that owns a managed resource (always /applications, not /applicationsets). */
export function getManagingApplicationUrl(appName: string, appNamespace: string): string {
    return AppUtils.getAppUrl({
        kind: 'Application',
        metadata: {name: appName, namespace: appNamespace}
    } as models.Application);
}

/** `node` query value for resource details (matches application details list view). */
export function resourceNodeUrl(resource: models.Resource): string {
    return `${AppUtils.nodeKey(resource)}/0`;
}

/** `detailsApp` query value: owning application namespace/name. */
export function resourceDetailsAppParam(resource: models.Resource): string {
    return `${resource.appNamespace}/${resource.appName}`;
}

export function openResourceDetails(ctx: ContextApis, resource: models.Resource) {
    ctx.navigation.goto(
        '.',
        {
            node: resourceNodeUrl(resource),
            detailsApp: resourceDetailsAppParam(resource)
        },
        {replace: true}
    );
}

/**
 * Navigate to the owning Application and open the resource's details panel there.
 * Matches the DETAILS button in the read-only resource panel (opens details in the app context).
 */
export function openResourceDetailsInApplication(ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) {
    ctx.navigation.goto(getManagingApplicationUrl(resource.appName, resource.appNamespace), {node: resourceNodeUrl(resource)}, e ? {event: e} : undefined);
}

/** `highlight` query value: highlight a resource in list/tree view without opening the details panel. */
export function resourceHighlightUrl(resource: models.Resource): string {
    return resourceNodeUrl(resource);
}

/** Navigate to the owning Application and highlight the resource in list/tree view. */
export function navigateToManagingApplication(ctx: ContextApis, resource: models.Resource, e?: React.MouseEvent) {
    ctx.navigation.goto(getManagingApplicationUrl(resource.appName, resource.appNamespace), {highlight: resourceHighlightUrl(resource)}, e ? {event: e} : undefined);
}

/** A row/cell link: a real `href` for middle-click / open-in-new-tab plus an SPA `onClick`. */
export interface ResourceLink {
    href: string;
    onClick: (e: React.MouseEvent<HTMLAnchorElement>) => void;
}

/** True for clicks the browser should handle natively on the anchor (new tab / new window). */
function isModifiedClick(e: React.MouseEvent): boolean {
    return e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0;
}

/**
 * Link that opens the resource's details panel on the current Resources page (query params only,
 * same route). The `href` reproduces the current URL so middle-click opens the panel in a new tab.
 */
export function getResourceRowLink(ctx: ContextApis, resource: models.Resource): ResourceLink {
    const params = {node: resourceNodeUrl(resource), detailsApp: resourceDetailsAppParam(resource)};
    const search = new URLSearchParams(window.location.search);
    search.set('node', params.node);
    search.set('detailsApp', params.detailsApp);
    return {
        href: `${window.location.pathname}?${search.toString()}`,
        onClick: (e: React.MouseEvent<HTMLAnchorElement>) => {
            if (isModifiedClick(e)) {
                return;
            }
            e.preventDefault();
            ctx.navigation.goto('.', params, {replace: true});
        }
    };
}

/** Link to the owning Application, highlighting the resource in list/tree view (real route change). */
export function getResourceAppLink(ctx: ContextApis, resource: models.Resource): ResourceLink {
    const url = getManagingApplicationUrl(resource.appName, resource.appNamespace);
    const highlight = resourceHighlightUrl(resource);
    return {
        href: `${ctx.baseHref}${url}?highlight=${encodeURIComponent(highlight)}`,
        onClick: (e: React.MouseEvent<HTMLAnchorElement>) => {
            if (isModifiedClick(e)) {
                return;
            }
            e.preventDefault();
            ctx.navigation.goto(`/${url}`, {highlight});
        }
    };
}
