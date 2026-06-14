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

interface ParsedResource {
    group: string;
    kind: string;
    namespace: string;
    name: string;
    obj: any;
}

const parseManifests = (manifests: string[], defaultNamespace: string): Map<string, ParsedResource> => {
    const out = new Map<string, ParsedResource>();
    for (const raw of manifests || []) {
        if (!raw || !raw.trim()) {
            continue;
        }
        let obj: any;
        try {
            obj = jsYaml.load(raw);
        } catch {
            continue;
        }
        if (!obj || typeof obj !== 'object' || !obj.kind) {
            continue;
        }
        const apiVersion: string = obj.apiVersion || '';
        const group = apiVersion.includes('/') ? apiVersion.split('/')[0] : '';
        const kind: string = obj.kind;
        const name: string = obj.metadata?.name || '';
        const namespace: string = obj.metadata?.namespace || defaultNamespace || '';
        const key = `${group}/${kind}/${namespace}/${name}`;
        out.set(key, {group, kind, namespace, name, obj});
    }
    return out;
};

const buildDiffs = (current: Map<string, ParsedResource>, target: Map<string, ParsedResource>): models.ResourceDiff[] => {
    const keys = new Set<string>([...current.keys(), ...target.keys()]);
    const diffs: models.ResourceDiff[] = [];
    keys.forEach(key => {
        const c = current.get(key);
        const t = target.get(key);
        const meta = c || t;
        if (!meta) {
            return;
        }
        // ApplicationResourcesDiff reads normalizedLiveState (a side) and predictedLiveState (b side).
        // a = "current" (what's deployed now), b = "target" (what would render at the rollback revision).
        diffs.push({
            group: meta.group,
            kind: meta.kind,
            namespace: meta.namespace,
            name: meta.name,
            hook: false,
            targetState: (t?.obj || null) as any,
            liveState: (c?.obj || null) as any,
            normalizedLiveState: (c?.obj || null) as any,
            predictedLiveState: (t?.obj || null) as any
        });
    });
    return diffs;
};

const getCurrentRevisionInfo = (app: models.Application): {revision: string; revisions?: string[]} => {
    const sync = app.status?.sync;
    if (sync?.revisions && sync.revisions.length > 0) {
        return {revision: sync.revisions[0], revisions: sync.revisions};
    }
    if (sync?.revision) {
        return {revision: sync.revision};
    }
    // Fallback to latest history entry
    const history = app.status?.history || [];
    if (history.length > 0) {
        const last = history[history.length - 1];
        if (last.revisions && last.revisions.length > 0) {
            return {revision: last.revisions[0], revisions: last.revisions};
        }
        return {revision: last.revision};
    }
    return {revision: ''};
};

export const RollbackPreviewDiff = ({app, info}: Props) => {
    const current = getCurrentRevisionInfo(app);
    const targetRevisions = info.revisions && info.revisions.length > 0 ? info.revisions : undefined;
    const targetSourcePositions = targetRevisions ? targetRevisions.map((_, i) => i + 1) : undefined;

    const sameRevision = current.revision === info.revision && JSON.stringify(current.revisions || []) === JSON.stringify(info.revisions || []);

    if (!current.revision) {
        return <div style={{padding: '1em'}}>No currently-synced revision found — nothing to compare against.</div>;
    }

    if (sameRevision) {
        return <div style={{padding: '1em'}}>This is the currently-synced revision. No changes.</div>;
    }

    type LoadResult = {diffs?: models.ResourceDiff[]; error?: string};

    return (
        <DataLoader
            key={`rollback-preview-${info.id}`}
            input={{
                appName: app.metadata.name,
                appNs: app.metadata.namespace,
                currentRevision: current.revision,
                currentRevisions: current.revisions,
                targetRevision: info.revision,
                targetRevisions,
                targetSourcePositions,
                defaultNs: app.spec.destination?.namespace || ''
            }}
            load={async (input): Promise<LoadResult> => {
                // Catch errors here rather than letting them propagate to the DataLoader's error
                // handler, which depends on a React context (appContext) that may not be present
                // in every render path.
                try {
                    const currentSourcePositions = input.currentRevisions ? input.currentRevisions.map((_: string, i: number) => i + 1) : undefined;
                    const [currentResp, targetResp] = await Promise.all([
                        services.applications.getManifest(input.appName, input.appNs, input.currentRevision, input.currentRevisions, currentSourcePositions),
                        services.applications.getManifest(input.appName, input.appNs, input.targetRevision, input.targetRevisions, input.targetSourcePositions)
                    ]);
                    const currentMap = parseManifests(currentResp.manifests || [], input.defaultNs);
                    const targetMap = parseManifests(targetResp.manifests || [], input.defaultNs);
                    return {diffs: buildDiffs(currentMap, targetMap)};
                } catch (e: any) {
                    return {error: e?.message || String(e)};
                }
            }}>
            {(result: LoadResult) => {
                if (result.error) {
                    return <div style={{padding: '1em', color: 'red'}}>Could not render diff: {result.error}</div>;
                }
                if (!result.diffs || result.diffs.length === 0) {
                    return <div style={{padding: '1em'}}>No manifest changes between the current revision and this history entry.</div>;
                }
                return <ApplicationResourcesDiff states={result.diffs} />;
            }}
        </DataLoader>
    );
};
