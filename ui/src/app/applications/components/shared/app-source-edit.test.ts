import * as models from '../../../shared/models';
import {
    APP_SOURCE_TYPES,
    inferSourceRepositoryType,
    isRefOnlySource,
    normalizeRefOnlySource,
    normalizeSourceForRepositoryType,
    normalizeSourceTypeFields,
    sourceDiscoveryKey,
    sourceKeyForTypeOverride,
    SourceRepositoryType
} from './app-source-edit';

describe('isRefOnlySource', () => {
    test.each([
        ['Git source with ref only', {repoURL: 'https://git.example.com/values.git', ref: 'values'}, true],
        ['Git source with ref and path', {repoURL: 'https://git.example.com/values.git', ref: 'values', path: 'manifests'}, false],
        ['Helm source with ref', {repoURL: 'https://charts.example.com', ref: 'values', chart: 'application'}, false],
        ['OCI source with ref', {repoURL: 'oci://registry.example.com/application', ref: 'values'}, false]
    ])('%s', (_name, source, expected) => {
        expect(isRefOnlySource(source as models.ApplicationSource)).toBe(expected);
    });
});

describe('inferSourceRepositoryType', () => {
    test.each([
        ['Git', {repoURL: 'https://git.example.com/app.git', path: 'manifests'}, 'git'],
        ['Helm', {repoURL: 'https://charts.example.com', chart: 'application'}, 'helm'],
        ['OCI', {repoURL: 'oci://registry.example.com/application', path: '.'}, 'oci']
    ])('infers %s sources', (_name, source, expected) => {
        expect(inferSourceRepositoryType(source as models.ApplicationSource)).toBe(expected);
    });
});

describe('normalizeSourceForRepositoryType', () => {
    test('normalizes a ref-only Git source selected as a Helm repository', () => {
        const source = {
            repoURL: 'https://charts.example.com',
            targetRevision: 'main',
            ref: 'values',
            kustomize: {namePrefix: 'stale-'}
        } as models.ApplicationSource;

        const normalized = normalizeSourceForRepositoryType(source, 'helm');

        expect(normalized).toEqual({repoURL: 'https://charts.example.com', targetRevision: '', chart: ''});
        expect(source).toHaveProperty('ref', 'values');
        expect(source).toHaveProperty('kustomize');
    });

    test.each<{
        name: string;
        source: models.ApplicationSource;
        type: SourceRepositoryType;
        expected: models.ApplicationSource;
    }>([
        {
            name: 'Git path to Helm chart',
            source: {repoURL: 'https://charts.example.com', path: 'application', targetRevision: 'main', directory: {recurse: true}} as models.ApplicationSource,
            type: 'helm',
            expected: {repoURL: 'https://charts.example.com', chart: 'application', targetRevision: ''} as models.ApplicationSource
        },
        {
            name: 'Helm chart to Git path',
            source: {
                repoURL: 'https://git.example.com/app.git',
                chart: 'application',
                targetRevision: '1.0.0',
                helm: {valueFiles: ['values.yaml']}
            } as models.ApplicationSource,
            type: 'git',
            expected: {
                repoURL: 'https://git.example.com/app.git',
                path: 'application',
                targetRevision: 'HEAD',
                helm: {valueFiles: ['values.yaml']}
            } as models.ApplicationSource
        },
        {
            name: 'Helm chart to OCI path',
            source: {repoURL: 'oci://registry.example.com/app', chart: 'application', targetRevision: '1.0.0'} as models.ApplicationSource,
            type: 'oci',
            expected: {repoURL: 'oci://registry.example.com/app', path: 'application', targetRevision: 'HEAD'} as models.ApplicationSource
        },
        {
            name: 'Git path with ref to OCI path',
            source: {repoURL: 'oci://registry.example.com/app', path: 'manifests', targetRevision: 'main', ref: 'values'} as models.ApplicationSource,
            type: 'oci',
            expected: {repoURL: 'oci://registry.example.com/app', path: 'manifests', targetRevision: 'main'} as models.ApplicationSource
        },
        {
            name: 'OCI path to Git path',
            source: {repoURL: 'https://git.example.com/app.git', path: 'manifests', targetRevision: '1.2.3'} as models.ApplicationSource,
            type: 'git',
            expected: {repoURL: 'https://git.example.com/app.git', path: 'manifests', targetRevision: '1.2.3'} as models.ApplicationSource
        }
    ])('$name', ({source, type, expected}) => {
        expect(normalizeSourceForRepositoryType(source, type)).toEqual(expected);
    });

    test('is idempotent for an already normalized source', () => {
        const source = {repoURL: 'https://charts.example.com', chart: 'application', targetRevision: '1.0.0', helm: {valueFiles: []}} as models.ApplicationSource;

        expect(normalizeSourceForRepositoryType(source, 'helm')).toBe(source);
    });
});

describe('source identity keys', () => {
    const pathSource = {repoURL: 'https://git.example.com/app.git', path: 'manifests', targetRevision: 'main'} as models.ApplicationSource;

    test('separates explicit generator choices by repository and path/chart shape', () => {
        expect(sourceKeyForTypeOverride({...pathSource, repoURL: 'https://git.example.com/other.git'})).not.toBe(sourceKeyForTypeOverride(pathSource));
        expect(sourceKeyForTypeOverride({repoURL: pathSource.repoURL, chart: pathSource.path, targetRevision: 'main'} as models.ApplicationSource)).not.toBe(
            sourceKeyForTypeOverride(pathSource)
        );
    });

    test('keeps a generator choice across revisions but remounts discovery', () => {
        const nextRevision = {...pathSource, targetRevision: 'release'};

        expect(sourceKeyForTypeOverride(nextRevision)).toBe(sourceKeyForTypeOverride(pathSource));
        expect(sourceDiscoveryKey(nextRevision)).not.toBe(sourceDiscoveryKey(pathSource));
    });

    test('remounts discovery when the application name or project changes', () => {
        expect(sourceDiscoveryKey(pathSource, 'app-a', 'default')).not.toBe(sourceDiscoveryKey(pathSource, 'app-b', 'default'));
        expect(sourceDiscoveryKey(pathSource, 'app-a', 'default')).not.toBe(sourceDiscoveryKey(pathSource, 'app-a', 'other-project'));
    });
});

describe('source generator normalization', () => {
    const sourceWithEveryGenerator = {
        repoURL: 'https://git.example.com/app.git',
        path: 'manifests',
        targetRevision: 'main',
        helm: {valueFiles: []},
        kustomize: {namePrefix: 'test-'},
        directory: {recurse: true},
        plugin: {name: 'example'}
    } as models.ApplicationSource;

    test.each(APP_SOURCE_TYPES)('keeps only the $type generator block', ({type, field}) => {
        const normalized = normalizeSourceTypeFields(sourceWithEveryGenerator, type) as unknown as Record<string, unknown>;

        for (const sourceType of APP_SOURCE_TYPES) {
            expect(Object.prototype.hasOwnProperty.call(normalized, sourceType.field)).toBe(sourceType.field === field);
        }
    });

    test('clears every generator block when a source becomes ref-only', () => {
        const source = {...sourceWithEveryGenerator, path: '', ref: 'values'} as models.ApplicationSource;
        delete source.path;

        const normalized = normalizeRefOnlySource(source) as unknown as Record<string, unknown>;

        for (const sourceType of APP_SOURCE_TYPES) {
            expect(normalized).not.toHaveProperty(sourceType.field);
        }
        expect(normalized).toMatchObject({repoURL: source.repoURL, targetRevision: 'main', ref: 'values'});
    });
});
