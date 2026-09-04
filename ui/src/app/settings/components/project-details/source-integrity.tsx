import * as React from 'react';
import {DataLoader, EditablePanel} from '../../../shared/components';
import {helpTip} from '../../../applications/components/utils';
import {GnuPGPublicKey, Project, ProjectSignatureKey, ProjectSourceIntegrity, SourceIntegrityGit} from '../../../shared/models';
import {AutocompleteField, FormField} from 'argo-ui';

require('./source-integrity.scss');

type SourceIntegritySection<T> = {
    key: string;
    isConfigured: (si?: ProjectSourceIntegrity) => boolean;
    View: React.ComponentType<T>;
    getProps: (si?: ProjectSourceIntegrity) => T;
};

const removeEl = (items: any[], index: number) => items.slice(0, index).concat(items.slice(index + 1));
const getIncludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => !repo.startsWith('!'));
const getExcludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => repo.startsWith('!')).map(repo => repo.slice(1));

const GitSourceIntegritySection = ({git}: {git?: SourceIntegrityGit}) => (
    <>
        <p className='project-details__list-title'>GIT</p>
        {git?.policies?.map((policy, policyIndex) => {
            const includedRepoUrls = getIncludedRepoUrlsOnly(policy.repos?.map(repo => repo.url));
            const excludedRepoUrls = getExcludedRepoUrlsOnly(policy.repos?.map(repo => repo.url));
            return (
                <div key={policyIndex} className='white-box source-integrity-panel__policy'>
                    {policy.gpg && (
                        <>
                            <div className='row white-box__details-row'>
                                <div className='columns small-2 columns--no-border'>GPG</div>
                                <div className='columns small-2'>MODE</div>
                                <div className='columns small-8'>{policy.gpg?.mode}</div>
                            </div>
                            <div className='row white-box__details-row'>
                                <div className='columns small-2'></div>
                                <div className='columns small-2'>KEYS</div>
                                <div className='columns small-8'>{(policy.gpg?.keys ?? []).join(', ')}</div>
                            </div>
                        </>
                    )}
                    <div className='row white-box__details-row'>
                        <div className='columns small-4'>REPO-URLS</div>
                        <div className='columns small-8'>
                            {(includedRepoUrls?.length ?? 0) === 0 ? <div>None</div> : includedRepoUrls?.map((repoUrl, repoIndex) => <div key={repoIndex}>{repoUrl}</div>)}
                        </div>
                    </div>
                    {(excludedRepoUrls?.length ?? 0) > 0 && (
                        <div className='row white-box__details-row'>
                            <div className='columns small-4'>EXCLUDED REPO-URLS</div>
                            <div className='columns small-8'>
                                {excludedRepoUrls?.map((repoUrl, repoIndex) => (
                                    <div key={repoIndex}>{repoUrl}</div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            );
        })}
    </>
);

const SOURCE_INTEGRITY_SECTIONS: SourceIntegritySection<ProjectSourceIntegrity>[] = [
    {
        key: 'git',
        isConfigured: (si?: ProjectSourceIntegrity) => (si?.git?.policies?.length ?? 0) > 0,
        View: GitSourceIntegritySection,
        getProps: (si?: ProjectSourceIntegrity) => ({git: si?.git})
    }
    // Add other sections here as new source integrity types are added
];

const SourceIntegrityContent = ({sourceIntegrity}: {sourceIntegrity?: ProjectSourceIntegrity}) => {
    const configuredSections = SOURCE_INTEGRITY_SECTIONS.filter(section => section.isConfigured(sourceIntegrity));

    if (configuredSections.length === 0) {
        return <p>Source Integrity is not configured.</p>;
    }

    return configuredSections.map(section => <section.View key={section.key} {...section.getProps(sourceIntegrity)} />);
};

const SourceIntegrityPanel = ({proj}: {proj: Project}) => (
    <EditablePanel
        values={proj}
        title={<React.Fragment>SOURCE INTEGRITY {helpTip('Verification criteria that application sources must meet before they can be synced and applied.')}</React.Fragment>}
        view={<SourceIntegrityContent sourceIntegrity={proj.spec.sourceIntegrity} />}
        items={[]}
    />
);

const LegacyGPGSignatureKeysPanel = ({
    proj,
    saveProject,
    loadSignatureKeys
}: {
    proj: Project;
    saveProject: (proj: Project) => Promise<void>;
    loadSignatureKeys: () => Promise<GnuPGPublicKey[]>;
}) => {
    const deprecatedFeatureMessage = 'This feature is deprecated, migrate to Source Integrity instead.';

    return (
        <EditablePanel
            save={saveProject}
            values={proj}
            title={
                <React.Fragment>[DEPRECATED] GPG SIGNATURE KEYS {helpTip('IDs of GnuPG keys that commits must be signed with in order to be allowed to sync to')}</React.Fragment>
            }
            view={
                <React.Fragment>
                    <p>{deprecatedFeatureMessage}</p>
                    {(proj.spec.signatureKeys?.length ?? 0) > 0 ? (
                        proj.spec.signatureKeys.map((key, i) => (
                            <div className='row white-box__details-row' key={i}>
                                <div className='columns small-12'>{key.keyID}</div>
                            </div>
                        ))
                    ) : (
                        <p>Project has no signature keys</p>
                    )}
                </React.Fragment>
            }
            edit={formApi => (
                <DataLoader load={loadSignatureKeys}>
                    {(keys: GnuPGPublicKey[]) => (
                        <React.Fragment>
                            <p>{deprecatedFeatureMessage}</p>
                            {(formApi.values.spec.signatureKeys || []).map((_: ProjectSignatureKey, i: number) => (
                                <div className='row white-box__details-row' key={i}>
                                    <div className='columns small-12'>
                                        <FormField
                                            formApi={formApi}
                                            field={`spec.signatureKeys[${i}].keyID`}
                                            component={AutocompleteField}
                                            componentProps={{items: keys.map(key => key.keyID)}}
                                        />
                                    </div>
                                    <i className='fa fa-times' onClick={() => formApi.setValue('spec.signatureKeys', removeEl(formApi.values.spec.signatureKeys, i))} />
                                </div>
                            ))}
                            <button
                                className='argo-button argo-button--short'
                                onClick={() =>
                                    formApi.setValue(
                                        'spec.signatureKeys',
                                        (formApi.values.spec.signatureKeys || []).concat({
                                            keyID: ''
                                        })
                                    )
                                }>
                                ADD KEY
                            </button>
                        </React.Fragment>
                    )}
                </DataLoader>
            )}
            items={[]}
        />
    );
};

const SourceIntegrityInfoBanner = () => (
    <div className='source-integrity__info-banner'>
        <i className='fa fa-info-circle' />
        <span>
            Source Integrity is configured via CLI or manifests. See{' '}
            <a href='https://argo-cd.readthedocs.io/en/stable/user-guide/source-integrity/' target='_blank' rel='noopener noreferrer'>
                <i className='fa fa-external-link-alt' /> documentation
            </a>{' '}
            or{' '}
            <a href='https://argo-cd.readthedocs.io/en/stable/user-guide/commands/argocd_proj_source-integrity/' target='_blank' rel='noopener noreferrer'>
                <i className='fa fa-external-link-alt' /> CLI reference
            </a>
            .
        </span>
    </div>
);

export const SourceIntegrityTab = ({
    proj,
    loadSignatureKeys,
    saveProject
}: {
    proj: Project;
    loadSignatureKeys: () => Promise<GnuPGPublicKey[]>;
    saveProject: (proj: Project) => Promise<void>;
}) => (
    <>
        <SourceIntegrityInfoBanner />
        <div className='argo-container source-integrity-tab-container'>
            <SourceIntegrityPanel proj={proj} />
            <LegacyGPGSignatureKeysPanel proj={proj} saveProject={saveProject} loadSignatureKeys={loadSignatureKeys} />
        </div>
    </>
);
