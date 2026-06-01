import * as AppUtils from '../../applications/components/utils';
import {ContextApis} from '../../shared/context';
import * as models from '../../shared/models';

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
