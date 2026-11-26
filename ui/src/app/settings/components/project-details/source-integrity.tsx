import * as React from 'react';
import {EditablePanel} from '../../../shared/components';
import {helpTip} from '../../../applications/components/utils';
import {Project, ProjectSignatureKey, ProjectSourceIntegrity, SourceIntegrityGit} from '../../../shared/models';
import {AutocompleteField, FormApi, FormField} from 'argo-ui';
import {useQuery} from '@tanstack/react-query';
import {gpgkeyQueries} from '../../../shared/queries';

require('./source-integrity.scss');

type SourceIntegritySection<T> = {
    key: string;
    isConfigured: (si?: ProjectSourceIntegrity) => boolean;
    View: React.ComponentType<T>;
    getProps: (si?: ProjectSourceIntegrity) => T;
};

const SOURCE_INTEGRITY_SECTIONS: SourceIntegritySection<ProjectSourceIntegrity>[] = [
    {
        key: 'git',
        isConfigured: (si?: ProjectSourceIntegrity) => si?.git != null,
        View: GitSourceIntegritySection,
        getProps: (si?: ProjectSourceIntegrity) => ({git: si?.git})
    }
    // Add other sections here as new source integrity types are added
];

export const SourceIntegrityTab = ({proj, saveProject}: {proj: Project; saveProject: (proj: Project) => Promise<void>}) => (
    <>
        <SourceIntegrityInfoBanner />
        <div className='argo-container source-integrity-tab-container'>
            <SourceIntegrityPanel proj={proj} />
            <LegacyGPGSignatureKeysPanel proj={proj} saveProject={saveProject} />
        </div>
    </>
);

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

const SourceIntegrityPanel = ({proj}: {proj: Project}) => (
    <EditablePanel
        values={proj}
        title={<React.Fragment>SOURCE INTEGRITY {helpTip('Verification criteria that application sources must meet before they can be synced and applied.')}</React.Fragment>}
        view={<SourceIntegrityContent sourceIntegrity={proj.spec.sourceIntegrity} signatureKeys={proj.spec.signatureKeys} />}
        items={[]}
    />
);

const SourceIntegrityContent = ({sourceIntegrity, signatureKeys}: {sourceIntegrity?: ProjectSourceIntegrity; signatureKeys?: ProjectSignatureKey[]}) => {
    const configuredSections = SOURCE_INTEGRITY_SECTIONS.filter(section => section.isConfigured(sourceIntegrity));

    // EffectiveSourceIntegrity - legacy keys are ignored whenever Git is set.
    const showLegacyKeysNotice = (signatureKeys?.length ?? 0) > 0 && sourceIntegrity?.git == null;

    if (configuredSections.length === 0) {
        if (showLegacyKeysNotice) {
            return <p>Source Integrity is not configured. Legacy GPG signature keys below are still enforced.</p>;
        }
        return <p>Source Integrity is not configured.</p>;
    }

    return (
        <>
            {showLegacyKeysNotice && <p>Source Integrity Git is not configured. Legacy GPG signature keys below are still enforced.</p>}
            {configuredSections.map(section => (
                <section.View key={section.key} {...section.getProps(sourceIntegrity)} />
            ))}
        </>
    );
};

const SIGNATURE_KEYS_DEPRECATION_MESSAGE = 'This feature is deprecated. Migrate to Source Integrity instead.';

const LegacyGPGSignatureKeysPanel = ({proj, saveProject}: {proj: Project; saveProject: (proj: Project) => Promise<void>}) => {
    const signatureKeysMessage = getSignatureKeysMessage(proj);

    return (
        <EditablePanel
            save={saveProject}
            values={proj}
            title={<React.Fragment>[DEPRECATED] GPG SIGNATURE KEYS {helpTip('GnuPG key IDs that Git commits must be signed with before they can be synced.')}</React.Fragment>}
            view={
                <React.Fragment>
                    <p>{SIGNATURE_KEYS_DEPRECATION_MESSAGE}</p>
                    {signatureKeysMessage && <p>{signatureKeysMessage}</p>}
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
            edit={formApi => <SignatureKeysEdit formApi={formApi} signatureKeysMessage={signatureKeysMessage} />}
            items={[]}
        />
    );
};

const SignatureKeysEdit = ({formApi, signatureKeysMessage}: {formApi: FormApi; signatureKeysMessage: React.ReactNode}) => {
    const {data: keys = []} = useQuery(gpgkeyQueries.list());
    return (
        <React.Fragment>
            <p>{SIGNATURE_KEYS_DEPRECATION_MESSAGE}</p>
            {signatureKeysMessage && <p>{signatureKeysMessage}</p>}
            {(formApi.values.spec.signatureKeys || []).map((_: Project, i: number) => (
                <div className='row white-box__details-row' key={i}>
                    <div className='columns small-12'>
                        <FormField formApi={formApi} field={`spec.signatureKeys[${i}].keyID`} component={AutocompleteField} componentProps={{items: keys.map(key => key.keyID)}} />
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
    );
};

const SIGNATURE_KEYS_UPGRADE_GUIDE_URL = 'https://argo-cd.readthedocs.io/en/stable/user-guide/source-integrity-git-gpg/#upgrade-to-source-integrity-verification';

const UpgradeGuideLink = () => (
    <a target='_blank' rel='noopener noreferrer' href={SIGNATURE_KEYS_UPGRADE_GUIDE_URL}>
        Upgrade to Source Integrity
    </a>
);

const getSignatureKeysMessage = (proj: Project): React.ReactNode => {
    if ((proj.spec.signatureKeys?.length ?? 0) === 0) {
        return null;
    }

    if (proj.spec.sourceIntegrity?.git != null) {
        return (
            <>
                These keys are <strong>not enforced</strong> because Source Integrity Git is configured. See <UpgradeGuideLink />.
            </>
        );
    }

    return (
        <>
            These keys are still enforced for all Git repositories, same as a Source Integrity Git policy in head mode. See <UpgradeGuideLink />.
        </>
    );
};

function GitSourceIntegritySection({git}: {git?: SourceIntegrityGit}) {
    return (
        <>
            <p className='project-details__list-title'>GIT</p>
            {(git?.policies?.length ?? 0) === 0 && <div className='white-box source-integrity-panel__policy'>Source Integrity Git has no policies.</div>}
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
                                    <div className='columns small-8'>{(policy.gpg?.keys ?? []).length > 0 ? (policy.gpg?.keys ?? []).join(', ') : <div>None</div>}</div>
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
}

const getIncludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => !repo.startsWith('!'));
const getExcludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => repo.startsWith('!')).map(repo => repo.slice(1));
const removeEl = (items: any[], index: number) => items.slice(0, index).concat(items.slice(index + 1));
