import * as moment from 'moment';
import * as React from 'react';
import * as classNames from 'classnames';
import * as models from '../../../shared/models';
import './application-deployment-history.scss';
import '../../../shared/components/editable-panel/editable-panel.scss';
import {DataLoader} from 'argo-ui';
import {Repo, Revision} from '../../../shared/components';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {RevisionMetadataRows} from './revision-metadata-rows';

type props = {
    app: models.Application;
    info: models.RevisionHistory;
    index: number;
};

export const ApplicationDeploymentHistoryDetails = ({app, info, index}: props) => {
    const deployments = (app.status.history || []).slice().reverse();
    const recentDeployments = deployments.map((info, i) => {
        const nextDeployedAt = i === 0 ? null : deployments[i - 1].deployedAt;
        const runEnd = nextDeployedAt ? moment(nextDeployedAt) : moment();
        return {...info, nextDeployedAt, durationMs: runEnd.diff(moment(info.deployedAt)) / 1000};
    });

    const [showParameterDetails, setShowParameterDetails] = React.useState(Boolean);
    const [showSourceDetails, setShowSourceDetails] = React.useState([]);
    const updateMap = (i: number) => {
        if (i === null || i === undefined) {
            return;
        }
        if (showSourceDetails.includes(i)) {
            setShowSourceDetails(showSourceDetails.filter(item => item !== i));
        } else {
            setShowSourceDetails([...showSourceDetails, i]);
        }
    };

    const getCollapsedSection = (i: number, repoURL: string): React.ReactFragment => {
        return (
            <div
                id={i ? `'hide-parameters-'${i}` : 'hide-parameters'}
                key={i ? `'hide-parameters-'${i}` : 'hide-parameters'}
                className='settings-overview__redirect-panel collapsible-section'
                onClick={() => {
                    setShowParameterDetails(!showParameterDetails);
                    updateMap(i);
                }}>
                <div className='editable-panel__collapsible-button'>
                    <i className={`fa fa-angle-down filter__collapse editable-panel__collapsible-button__override`} />
                </div>

                <div style={{textAlign: 'center'}}>
                    <div className='settings-overview__redirect-panel__title'>{i != null ? 'Source ' + (i + 1) + ' Parameters' : 'Source Parameters'}</div>
                    <div className='settings-overview__redirect-panel__description'>URL: {repoURL}</div>
                </div>
            </div>
        );
    };

    const getExpandedSection = (index?: number): React.ReactFragment => {
        return (
            <React.Fragment>
                <div id={index ? `'show-parameters-'${index}` : 'show-parameters'} className='editable-panel__collapsible-button' style={{zIndex: 1001}}>
                    <i
                        className={`fa fa-angle-up filter__collapse editable-panel__collapsible-button__override`}
                        onClick={() => {
                            setShowParameterDetails(!showParameterDetails);
                            updateMap(index);
                        }}
                    />
                </div>
            </React.Fragment>
        );
    };

    const getErrorSection = (err: React.ReactNode): React.ReactFragment => {
        return (
            <div style={{padding: '1.7em'}}>
                <p style={{textAlign: 'center'}}>{err}</p>
            </div>
        );
    };

    return (
        <>
            {info.sources === undefined ? (
                <React.Fragment>
                    <div>
                        <div className='row'>
                            <div className='columns small-3'>Revision:</div>
                            <div className='columns small-9'>
                                <Revision repoUrl={info.source.repoURL} revision={info.revision} />
                            </div>
                        </div>
                    </div>
                    <RevisionMetadataRows
                        applicationName={app.metadata.name}
                        applicationNamespace={app.metadata.namespace}
                        source={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                        index={0}
                        versionId={recentDeployments[index].id}
                    />

                    {showParameterDetails ? (
                        <div id={`'history-expanded'`} key={`'history-expanded'`} className={classNames('white-box', 'collapsible-section')}>
                            {getExpandedSection()}
                            <DataLoader
                                errorRenderer={err => {
                                    return getErrorSection(err);
                                }}
                                input={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision, appName: app.metadata.name}}
                                load={src => services.repos.appDetails(src, src.appName, app.spec.project, 0, recentDeployments[index].id)}>
                                {(details: models.RepoAppDetails) => (
                                    <div>
                                        <ApplicationParameters
                                            application={{
                                                ...app,
                                                spec: {...app.spec, source: recentDeployments[index].source}
                                            }}
                                            details={details}
                                            tempSource={{...recentDeployments[index].source, targetRevision: recentDeployments[index].revision}}
                                        />
                                    </div>
                                )}
                            </DataLoader>
                        </div>
                    ) : (
                        getCollapsedSection(null, recentDeployments[index].source.repoURL)
                    )}
                </React.Fragment>
            ) : (
                info.sources.map((source, i) => (
                    <React.Fragment key={`${index}_${i}`}>
                        {i > 0 ? <div className='separator' /> : null}
                        <div>
                            <div className='row'>
                                <div className='columns small-3'>Repo URL:</div>
                                <div className='columns small-9'>
                                    <Repo url={source.repoURL} />
                                </div>
                            </div>
                            <div className='row'>
                                <div className='columns small-3'>Revision:</div>
                                <div className='columns small-9'>
                                    <Revision repoUrl={source.repoURL} revision={info.revisions[i]} />
                                </div>
                            </div>
                        </div>
                        <RevisionMetadataRows
                            applicationName={app.metadata.name}
                            applicationNamespace={app.metadata.namespace}
                            source={{...source, targetRevision: recentDeployments[index].revisions[i]}}
                            index={i}
                            versionId={recentDeployments[index].id}
                        />
                        {showSourceDetails.includes(i) ? (
                            <div id={`'history-expanded-'${i}`} key={`'history-expanded-'${i}`} className={classNames('white-box', 'collapsible-section')}>
                                <div id={`'history-expanded-'${i}`} key={`'history-expanded-'${i}`} className='white-box__details'>
                                    {getExpandedSection(i)}
                                    <DataLoader
                                        errorRenderer={err => {
                                            return getErrorSection(err);
                                        }}
                                        input={{
                                            ...source,
                                            targetRevision: recentDeployments[index].revisions[i],
                                            index: i,
                                            versionId: recentDeployments[index].id,
                                            appName: app.metadata.name
                                        }}
                                        load={src => services.repos.appDetails(src, src.appName, app.spec.project, i, recentDeployments[index].id)}>
                                        {(details: models.RepoAppDetails) => (
                                            <React.Fragment>
                                                <div id={'floating_title_' + i} className='editable-panel__sticky-title'>
                                                    <div style={{marginTop: '0px'}}>
                                                        <div>Source {i + 1} Parameters</div>
                                                        <div>Repo URL: {source.repoURL}</div>
                                                        <div>{source.path ? 'Path: ' + source.path : ''}</div>
                                                        <span>
                                                            Revision: <Revision repoUrl={''} revision={info.revisions[i]}></Revision>
                                                        </span>
                                                    </div>
                                                </div>
                                                <ApplicationParameters
                                                    application={{
                                                        ...app,
                                                        spec: {...app.spec, source}
                                                    }}
                                                    details={details}
                                                    tempSource={{...source, targetRevision: recentDeployments[index].revisions[i]}}
                                                />
                                            </React.Fragment>
                                        )}
                                    </DataLoader>
                                </div>
                            </div>
                        ) : (
                            getCollapsedSection(i, source.repoURL)
                        )}
                    </React.Fragment>
                ))
            )}
        </>
    );
};
