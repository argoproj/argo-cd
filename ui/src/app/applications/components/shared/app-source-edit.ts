import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';

export type SourceRepositoryType = 'git' | 'helm' | 'oci';

/** Shared with application-create-panel and application-parameters/source-panel. */
export const APP_SOURCE_TYPES = new Array<{field: string; type: models.AppSourceType}>(
    {type: 'Helm', field: 'helm'},
    {type: 'Kustomize', field: 'kustomize'},
    {type: 'Directory', field: 'directory'},
    {type: 'Plugin', field: 'plugin'}
);

/**
 * A ref-only source is checked out solely so another source can reference files from it.
 * It does not generate manifests and therefore has no application source type to configure.
 */
export function isRefOnlySource(source: models.ApplicationSource | undefined): boolean {
    return !!source?.ref && !source.path && !source.chart && !source.repoURL?.startsWith('oci://');
}

export function inferSourceRepositoryType(source: models.ApplicationSource | undefined): SourceRepositoryType {
    if (source?.repoURL?.startsWith('oci://')) {
        return 'oci';
    }
    if (source && Object.prototype.hasOwnProperty.call(source, 'chart')) {
        return 'helm';
    }
    return 'git';
}

/** Identifies the repository location for which the user explicitly selected a generator type. */
export function sourceKeyForTypeOverride(source: models.ApplicationSource | undefined): string {
    const usesChart = !!source && Object.prototype.hasOwnProperty.call(source, 'chart');
    return `${source?.repoURL || ''}\n${usesChart ? 'chart' : 'path'}\n${(usesChart ? source?.chart : source?.path) || ''}`;
}

/** Remounts source discovery whenever any field that identifies its request changes. */
export function sourceDiscoveryKey(source: models.ApplicationSource | undefined, appName = '', project = ''): string {
    return `${sourceKeyForTypeOverride(source)}\n${source?.targetRevision || ''}\n${appName}\n${project}`;
}

/**
 * Returns a source whose path/chart/ref fields match the selected repository type.
 * The original object is returned when it is already normalized.
 */
export function normalizeSourceForRepositoryType(source: models.ApplicationSource, type: SourceRepositoryType): models.ApplicationSource {
    const normalized = {...source};
    const normalizedRecord = normalized as unknown as Record<string, unknown>;
    let changed = false;

    const remove = (field: string) => {
        if (Object.prototype.hasOwnProperty.call(normalizedRecord, field)) {
            delete normalizedRecord[field];
            changed = true;
        }
    };
    const set = (field: string, value: unknown) => {
        if (normalizedRecord[field] !== value || !Object.prototype.hasOwnProperty.call(normalizedRecord, field)) {
            normalizedRecord[field] = value;
            changed = true;
        }
    };

    if (type !== 'git') {
        remove('ref');
    }

    if (type === 'helm') {
        const hasChart = Object.prototype.hasOwnProperty.call(normalizedRecord, 'chart');
        const hasPath = Object.prototype.hasOwnProperty.call(normalizedRecord, 'path');
        if (!hasChart) {
            set('chart', hasPath ? normalized.path : '');
            set('targetRevision', '');
        }
        remove('path');

        // A chart repository cannot render Kustomize, Directory, or Plugin sources.
        remove('kustomize');
        remove('directory');
        remove('plugin');
    } else if (Object.prototype.hasOwnProperty.call(normalizedRecord, 'chart')) {
        if (!Object.prototype.hasOwnProperty.call(normalizedRecord, 'path')) {
            set('path', normalized.chart);
            set('targetRevision', 'HEAD');
        }
        remove('chart');
    }

    return changed ? normalized : source;
}

/** Clears generator settings that cannot be used by a ref-only source. */
export function normalizeRefOnlySource(source: models.ApplicationSource): models.ApplicationSource {
    if (!isRefOnlySource(source)) {
        return source;
    }
    return clearSourceTypeFields(source);
}

/** Keeps only the generator block selected by the user. */
export function normalizeSourceTypeFields(source: models.ApplicationSource, type: models.AppSourceType): models.ApplicationSource {
    return clearSourceTypeFields(source, type);
}

function clearSourceTypeFields(source: models.ApplicationSource, keep?: models.AppSourceType): models.ApplicationSource {
    const normalized = {...source};
    const normalizedRecord = normalized as unknown as Record<string, unknown>;
    let changed = false;
    for (const item of APP_SOURCE_TYPES) {
        if (item.type !== keep && Object.prototype.hasOwnProperty.call(normalizedRecord, item.field)) {
            delete normalizedRecord[item.field];
            changed = true;
        }
    }
    return changed ? normalized : source;
}

/**
 * Clears sibling source-type blocks (helm/kustomize/directory/plugin) when the user picks a type.
 * @param sourceIndex — when set, edits `spec.sources[index]`; otherwise `spec.source`.
 */
export function normalizeTypeFieldsForSource(formApi: FormApi, type: models.AppSourceType, sourceIndex?: number): void {
    const appToNormalize = formApi.getFormState().values as models.Application;
    let normalizedSource: models.ApplicationSource;
    if (sourceIndex === undefined) {
        const source = appToNormalize.spec.source;
        if (!source) {
            return;
        }
        normalizedSource = normalizeSourceTypeFields(source, type);
        if (normalizedSource === source) {
            return;
        }
        formApi.setAllValues({...appToNormalize, spec: {...appToNormalize.spec, source: normalizedSource}});
    } else {
        const src = appToNormalize.spec.sources?.[sourceIndex];
        if (!src) {
            return;
        }
        normalizedSource = normalizeSourceTypeFields(src, type);
        if (normalizedSource === src) {
            return;
        }
        const sources = [...appToNormalize.spec.sources];
        sources[sourceIndex] = normalizedSource;
        formApi.setAllValues({...appToNormalize, spec: {...appToNormalize.spec, sources}});
    }
}
