import {DataLoader} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';

interface Props {
    app: models.Application;
    info: models.RevisionHistory;
}

// Mirror of util/resource.HasAnnotationOption — comma-separated Key=Value pairs.
// NOTE: only the per-app annotation is checked. The controller's global
// `--server-side-diff` flag is not exposed via the settings API, so an app with
// global SSD on but no annotation will render client-side here.
const hasCompareOption = (app: models.Application, option: string): boolean => {
    const raw = app.metadata.annotations?.['argocd.argoproj.io/compare-options'];
    if (!raw) {
        return false;
    }
    return raw
        .split(',')
        .map(s => s.trim())
        .includes(option);
};

const isServerSideDiffEnabled = (app: models.Application): boolean => {
    if (hasCompareOption(app, 'ServerSideDiff=false')) {
        return false;
    }
    return hasCompareOption(app, 'ServerSideDiff=true');
};

const parseManifest = (raw: string): any | null => {
    if (!raw || !raw.trim()) {
        return null;
    }
    try {
        const obj = jsYaml.load(raw);
        if (!obj || typeof obj !== 'object' || !(obj as any).kind) {
            return null;
        }
        return obj;
    } catch {
        return null;
    }
};

const resourceKey = (group: string, kind: string, namespace: string, name: string) => `${group}/${kind}/${namespace}/${name}`;

const keyOfManifest = (obj: any, defaultNs: string) => {
    const apiVersion: string = obj.apiVersion || '';
    const group = apiVersion.includes('/') ? apiVersion.split('/')[0] : '';
    const kind: string = obj.kind;
    const name: string = obj.metadata?.name || '';
    const namespace: string = obj.metadata?.namespace || defaultNs || '';
    return {key: resourceKey(group, kind, namespace, name), group, kind, namespace, name};
};

// Client-side: build ResourceDiff records keyed by GVK+ns+name where
// liveState comes from the real live cluster (managedResources) and
// targetState comes from the rollback revision's manifests.
const buildClientSideDiffs = (live: models.ResourceDiff[], targetManifests: string[], defaultNs: string): models.ResourceDiff[] => {
    const liveByKey = new Map<string, models.ResourceDiff>();
    for (const r of live || []) {
        liveByKey.set(resourceKey(r.group || '', r.kind, r.namespace || '', r.name), r);
    }
    const targetByKey = new Map<string, {obj: any; group: string; kind: string; namespace: string; name: string}>();
    for (const raw of targetManifests || []) {
        const obj = parseManifest(raw);
        if (!obj) {
            continue;
        }
        const k = keyOfManifest(obj, defaultNs);
        targetByKey.set(k.key, {obj, group: k.group, kind: k.kind, namespace: k.namespace, name: k.name});
    }

    const keys = new Set<string>([...liveByKey.keys(), ...targetByKey.keys()]);
    const diffs: models.ResourceDiff[] = [];
    keys.forEach(key => {
        const l = liveByKey.get(key);
        const t = targetByKey.get(key);
        const meta = t || (l && {group: l.group || '', kind: l.kind, namespace: l.namespace || '', name: l.name});
        if (!meta) {
            return;
        }
        diffs.push({
            group: meta.group,
            kind: meta.kind,
            namespace: meta.namespace,
            name: meta.name,
            hook: l?.hook || false,
            targetState: (t?.obj || null) as any,
            liveState: (l?.liveState || null) as any,
            normalizedLiveState: (l?.normalizedLiveState || l?.liveState || null) as any,
            predictedLiveState: (t?.obj || null) as any
        });
    });
    return diffs;
};

export const RollbackPreviewDiff = ({app, info}: Props) => {
    const targetRevisions = info.revisions && info.revisions.length > 0 ? info.revisions : undefined;
    const targetSourcePositions = targetRevisions ? targetRevisions.map((_, i) => i + 1) : undefined;

    type LoadResult = {diffs?: models.ResourceDiff[]; error?: string};

    return (
        <DataLoader
            key={`rollback-preview-${info.id}`}
            input={{
                appName: app.metadata.name,
                appNs: app.metadata.namespace,
                project: app.spec.project,
                targetRevision: info.revision,
                targetRevisions,
                targetSourcePositions,
                defaultNs: app.spec.destination?.namespace || '',
                ssd: isServerSideDiffEnabled(app)
            }}
            load={async (input): Promise<LoadResult> => {
                try {
                    const [liveResources, target] = await Promise.all([
                        services.applications.managedResources(input.appName, input.appNs),
                        services.applications.getManifest(input.appName, input.appNs, input.targetRevision, input.targetRevisions, input.targetSourcePositions)
                    ]);
                    const targetManifests = target.manifests || [];

                    if (input.ssd) {
                        const res = await services.applications.serverSideDiff(input.appName, input.appNs, input.project, liveResources, targetManifests);
                        return {diffs: res.items};
                    }
                    return {diffs: buildClientSideDiffs(liveResources, targetManifests, input.defaultNs)};
                } catch (e: any) {
                    return {error: e?.message || String(e)};
                }
            }}>
            {(result: LoadResult) => {
                if (result.error) {
                    return <div style={{padding: '1em', color: 'red'}}>Could not render diff: {result.error}</div>;
                }
                if (!result.diffs || result.diffs.length === 0) {
                    return <div style={{padding: '1em'}}>No changes between live state and this history entry.</div>;
                }
                return <ApplicationResourcesDiff states={result.diffs} />;
            }}
        </DataLoader>
    );
};
