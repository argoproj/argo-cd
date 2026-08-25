import React from 'react';
import {EditablePanel} from '../../../shared/components';
import {helpTip} from '../../../applications/components/utils';
import {Project, ProjectSourceIntegrity, SourceIntegrityGit} from '../../../shared/models';

require('./source-integrity-panel.scss');

type SourceIntegritySection<T> = {
    key: string;
    isConfigured: (si?: ProjectSourceIntegrity) => boolean;
    View: React.ComponentType<T>;
    getProps: (si?: ProjectSourceIntegrity) => T;
};

const getIncludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => !repo.startsWith('!'));
const getExcludedRepoUrlsOnly = (repos?: string[]) => repos?.filter(repo => repo.startsWith('!')).map(repo => repo.slice(1));

const GitSourceIntegrityView = ({git}: {git?: SourceIntegrityGit}) => {
    return (
        <>
            {/* <p className='project-details__list-title'>GIT</p> */}
            {git?.policies?.map((policy, policyIndex) => {
                const includedRepoUrls = getIncludedRepoUrlsOnly(policy?.repos?.map(repo => repo.url));
                const excludedRepoUrls = getExcludedRepoUrlsOnly(policy?.repos?.map(repo => repo.url));
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
};

const HelmMockView = () => {
    return (
        <>
            {/* <p className='project-details__list-title'>HELM</p> */}
            <div key={1} className='white-box source-integrity-panel__policy'>
                <div className='row white-box__details-row'>
                    <div className='columns small-2'>PROVENANCE</div>
                    <div className='columns small-2'>KEYS</div>
                    <div className='columns small-8'>FACE1234FFFFAAAA, 88882222FFFFAAAA</div>
                </div>
                <div className='row white-box__details-row'>
                    <div className='columns small-4'>REPO-URLS</div>
                    <div className='columns small-8'>
                        <div>*</div>
                    </div>
                </div>
                <div className='row white-box__details-row'>
                    <div className='columns small-4'>EXCLUDED REPO-URLS</div>
                    <div className='columns small-8'>
                        <div>https://github.com/argoproj/argo-cd.git</div>
                    </div>
                </div>
            </div>
        </>
    );
};

const SOURCE_INTEGRITY_SECTIONS: SourceIntegritySection<ProjectSourceIntegrity>[] = [
    {
        key: 'git',
        isConfigured: (si?: ProjectSourceIntegrity) => (si?.git?.policies?.length ?? 0) > 0,
        View: GitSourceIntegrityView,
        getProps: (si?: ProjectSourceIntegrity) => ({git: si?.git})
    },
    {
        key: 'helm',
        isConfigured: (si?: ProjectSourceIntegrity) => (si?.git?.policies?.length ?? 0) > 0, // show with git to mock the helm section
        View: HelmMockView,
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        getProps: (_si?: ProjectSourceIntegrity) => ({})
    }
];

const SourceIntegrityContent = ({sourceIntegrity}: {sourceIntegrity?: ProjectSourceIntegrity}) => {
    const configuredSections = SOURCE_INTEGRITY_SECTIONS.filter(section => section.isConfigured(sourceIntegrity));

    if (configuredSections.length === 0) {
        return (
            <p className={'source-integrity-panel--empty'}>
                Source Integrity is not configured. Use the <code> argocd proj source-integrity</code>{' '}
                <a href='https://argo-cd.readthedocs.io/en/stable/user-guide/commands/argocd_proj_source-integrity/' target='_blank' rel='noopener noreferrer'>
                    <i className='fa fa-external-link' /> CLI command
                </a>{' '}
                to configure it.
            </p>
        );
    }

    return configuredSections.map(section => <section.View key={section.key} {...section.getProps(sourceIntegrity)} />);
};

const SourceIntegrityPanel = ({proj}: {proj: Project}) => {
    const configuredSections = SOURCE_INTEGRITY_SECTIONS.filter(section => section.isConfigured(proj?.spec?.sourceIntegrity));
    return (
        <>
            {/* <EditablePanel
                values={proj}
                title={
                    <React.Fragment>SOURCE INTEGRITY {helpTip('Verification criteria that application sources must meet before they can be synced and applied.')}</React.Fragment>
                }
                view={<SourceIntegrityContent sourceIntegrity={proj.spec.sourceIntegrity} />}
                items={[]}
            /> */}

            {configuredSections.length === 0 && (
                <EditablePanel
                    values={{}}
                    view={
                        <p className='source-integrity-panel--empty'>
                            Source Integrity is not configured. Use the <code> argocd proj source-integrity</code>{' '}
                            <a href='https://argo-cd.readthedocs.io/en/stable/user-guide/commands/argocd_proj_source-integrity/' target='_blank' rel='noopener noreferrer'>
                                <i className='fa fa-external-link' /> CLI command
                            </a>{' '}
                            to configure it.
                        </p>
                    }
                    items={[]}
                />
            )}

            {configuredSections.map(section => (
                <EditablePanel
                    values={proj}
                    title={<React.Fragment>{section.key.toUpperCase()}</React.Fragment>}
                    view={<section.View {...section.getProps(proj?.spec?.sourceIntegrity)} />}
                    items={[]}
                />
            ))}
        </>
    );
};

export const SourceIntegrityTab = ({proj}: {proj: Project}) => {
    return (
        <div className='argo-container'>
            <SourceIntegrityPanel proj={proj} />
        </div>
    );
};
