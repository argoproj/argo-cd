import {AbstractApplication, Application, AuthSettings} from '../../../shared/models';
import {getApplicationSetOwnerRef} from '../utils';

export interface ApplicationParent {
    kind: 'Application' | 'ApplicationSet';
    name: string;
    namespace: string;
}

type TrackingSettings = Pick<AuthSettings, 'trackingMethod' | 'appLabelKey' | 'controllerNamespace' | 'installationID'>;

export function getApplicationParent(application: AbstractApplication, settings: TrackingSettings): ApplicationParent | undefined {
    const {metadata} = application;
    const kind = application.kind || 'Application';
    const namespace = metadata.namespace || settings.controllerNamespace;
    if (kind === 'Application') {
        const owner = getApplicationSetOwnerRef(application as Application);
        if (owner?.name && namespace) {
            return {kind: 'ApplicationSet', name: owner.name, namespace};
        }
    }

    let trackingIdentity: string | undefined;
    if (settings.trackingMethod === 'label') {
        trackingIdentity = metadata.labels?.[settings.appLabelKey || 'app.kubernetes.io/instance'];
    } else {
        const annotations = metadata.annotations || {};
        if (settings.installationID && annotations['argocd.argoproj.io/installation-id'] !== settings.installationID) {
            return undefined;
        }
        const trackingParts = annotations['argocd.argoproj.io/tracking-id']?.split(':');
        if (trackingParts?.length !== 3 || trackingParts[1] !== `argoproj.io/${kind}` || trackingParts[2] !== `${namespace}/${metadata.name}`) {
            return undefined;
        }
        trackingIdentity = trackingParts[0];
    }
    if (!trackingIdentity) {
        return undefined;
    }

    const namespaceSeparator = trackingIdentity.indexOf('_');
    const parentNamespace = namespaceSeparator < 0 ? settings.controllerNamespace : trackingIdentity.substring(0, namespaceSeparator);
    const name = namespaceSeparator < 0 ? trackingIdentity : trackingIdentity.substring(namespaceSeparator + 1);
    if (!parentNamespace || !name || (kind === 'Application' && parentNamespace === namespace && name === metadata.name)) {
        return undefined;
    }
    return {kind: 'Application', name, namespace: parentNamespace};
}

export function getApplicationParentPath(parent: ApplicationParent): string {
    const root = parent.kind === 'ApplicationSet' ? '/applicationsets' : '/applications';
    return `${root}/${encodeURIComponent(parent.namespace)}/${encodeURIComponent(parent.name)}`;
}
