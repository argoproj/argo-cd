import {Checkbox, DataLoader, Tab, Tabs} from 'argo-ui';
import classNames from 'classnames';
import * as deepMerge from 'deepmerge';
import * as React from 'react';

import {YamlEditor, ClipboardText} from '../../../shared/components';
import {DeepLinks} from '../../../shared/components/deep-links';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {ComparisonStatusIcon, formatCreationTimestamp, getPodReadinessGatesState, getPodStateReason, HealthStatusIcon} from '../utils';
import './application-node-info.scss';
import {ReadinessGatesNotPassedWarning} from './readiness-gates-not-passed-warning';

const RenderContainerState = (props: {container: any}) => {
    const state = (props.container.state?.waiting && 'waiting') || (props.container.state?.terminated && 'terminated') || (props.container.state?.running && 'running');
    const status = props.container.state.waiting?.reason || props.container.state.terminated?.reason || props.container.state.running?.reason;
    const lastState = props.container.lastState?.terminated;
    const msg = props.container.state.waiting?.message || props.container.state.terminated?.message || props.container.state.running?.message;

    return (
        <div className='application-node-info__container'>
            <div className='application-node-info__container--name'>
                {props.container.state?.running ? (
                    <span style={{marginRight: '4px'}}>
                        <i className='fa fa-check-circle' style={{color: 'rgb(24, 190, 148)'}} />
                    </span>
                ) : (
                    (props.container.state.terminated && props.container.state.terminated?.exitCode !== 0) ||
                    (lastState && lastState?.exitCode !== 0 && (
                        <span style={{marginRight: '4px'}}>
                            <i className='fa fa-times-circle' style={{color: 'red'}} />
                        </span>
                    ))
                )}
                {props.container.name}
            </div>
            <div>
                {state && (
                    <>
                        Container is <span className='application-node-info__container--highlight'>{state}</span>
                        {status && ' because of '}
                    </>
                )}
                <span title={msg || ''}>
                    {status && (
                        <span
                            className={classNames('application-node-info__container--highlight', {
                                'application-node-info__container--hint': !!msg
                            })}>
                            {status}
                        </span>
                    )}
                </span>
                {'.'}
                {(props.container.state.terminated?.exitCode === 0 || props.container.state.terminated?.exitCode) && (
                    <>
                        {' '}
                        It exited with <span className='application-node-info__container--highlight'>exit code {props.container.state.terminated.exitCode}.</span>
                    </>
                )}
                <>
                    {' '}
                    It is <span className='application-node-info__container--highlight'>{props.container?.started ? 'started' : 'not started'}</span>
                    <span className='application-node-info__container--highlight'>{status === 'Completed' ? '.' : props.container?.ready ? ' and ready.' : ' and not ready.'}</span>
                </>
                <br />
                {lastState && (
                    <>
                        <>
                            The container last terminated with <span className='application-node-info__container--highlight'>exit code {lastState?.exitCode}</span>
                        </>
                        {lastState?.reason && ' because of '}
                        <span title={props.container.lastState?.message || ''}>
                            {lastState?.reason && (
                                <span
                                    className={classNames('application-node-info__container--highlight', {
                                        'application-node-info__container--hint': !!props.container.lastState?.message
                                    })}>
                                    {lastState?.reason}
                                </span>
                            )}
                        </span>
                        {'.'}
                    </>
                )}
            </div>
        </div>
    );
};

export const ApplicationNodeInfo = (props: {
    application: models.Application;
    node: models.ResourceNode;
    live: models.State;
    links: models.LinksResponse;
    controlled: {summary: models.ResourceStatus; state: models.ResourceDiff};
}) => {
    const attributes: {title: string; value: any}[] = [
        {title: 'KIND', value: props.node.kind},
        {title: 'NAME', value: <ClipboardText text={props.node.name} />},
        {title: 'NAMESPACE', value: <ClipboardText text={props.node.namespace} />}
    ];
    if (props.node.createdAt) {
        attributes.push({
            title: 'CREATED AT',
            value: formatCreationTimestamp(props.node.createdAt)
        });
    }
    if ((props.node.images || []).length) {
        attributes.push({
            title: 'IMAGES',
            value: (
                <div className='application-node-info__labels'>
                    {(props.node.images || []).sort().map(image => (
                        <span className='application-node-info__label' key={image}>
                            {image}
                        </span>
                    ))}
                </div>
            )
        });
    }

    if (props.live) {
        if (props.node.kind === 'Pod') {
            const {reason, message, netContainerStatuses} = getPodStateReason(props.live);
            attributes.push({title: 'STATE', value: reason});
            if (message) {
                attributes.push({title: 'STATE DETAILS', value: message});
            }
            if (netContainerStatuses.length > 0) {
                attributes.push({
                    title: 'CONTAINER STATE',
                    value: (
                        <div className='application-node-info__labels'>
                            {netContainerStatuses.map((container, i) => {
                                return <RenderContainerState key={i} container={container} />;
                            })}
                        </div>
                    )
                });
            }
        } else if (props.node.kind === 'Service') {
            attributes.push({title: 'TYPE', value: props.live.spec.type});
            let hostNames = '';
            const status = props.live.status;
            if (status && status.loadBalancer && status.loadBalancer.ingress) {
                hostNames = (status.loadBalancer.ingress || []).map((item: any) => item.hostname || item.ip).join(', ');
            }
            attributes.push({title: 'HOSTNAMES', value: hostNames});
        } else if (props.node.kind === 'ReplicaSet') {
            attributes.push({title: 'REPLICAS', value: `${props.live.spec?.replicas || 0}/${props.live.status?.readyReplicas || 0}/${props.live.status?.replicas || 0}`});
        }
    }

    if (props.controlled) {
        if (!props.controlled.summary.hook) {
            attributes.push({
                title: 'STATUS',
                value: (
                    <span>
                        <ComparisonStatusIcon status={props.controlled.summary.status} resource={props.controlled.summary} label={true} />
                    </span>
                )
            } as any);
        }
        if (props.controlled.summary.health !== undefined) {
            attributes.push({
                title: 'HEALTH',
                value: (
                    <span>
                        <HealthStatusIcon state={props.controlled.summary.health} /> {props.controlled.summary.health.status}
                    </span>
                )
            } as any);
            if (props.controlled.summary.health.message) {
                attributes.push({title: 'HEALTH DETAILS', value: props.controlled.summary.health.message});
            }
        }
    } else if (props.node && (props.node as ResourceTreeNode).health) {
        const treeNode = props.node as ResourceTreeNode;
        if (treeNode && treeNode.health) {
            attributes.push({
                title: 'HEALTH',
                value: (
                    <span>
                        <HealthStatusIcon state={treeNode.health} /> {treeNode.health.message || treeNode.health.status}
                    </span>
                )
            } as any);
        }
    }
    let showLiveState = true;
    if (props.links) {
        attributes.push({
            title: 'LINKS',
            value: <DeepLinks links={props.links.items} />
        });
    }

    const tabs: Tab[] = [
        {
            key: 'manifest',
            title: 'Live Manifest',
            content: (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        const live = deepMerge(props.live, {}) as any;
                        if (Object.keys(live).length === 0) {
                            showLiveState = false;
                        }

                        if (live?.metadata?.managedFields && pref.appDetails.hideManagedFields) {
                            delete live.metadata.managedFields;
                        }
                        return (
                            <React.Fragment>
                                {showLiveState ? (
                                    <React.Fragment>
                                        <div className='application-node-info__checkboxes'>
                                            <Checkbox
                                                id='hideManagedFields'
                                                checked={!!pref.appDetails.hideManagedFields}
                                                onChange={() =>
                                                    services.viewPreferences.updatePreferences({
                                                        appDetails: {
                                                            ...pref.appDetails,
                                                            hideManagedFields: !pref.appDetails.hideManagedFields
                                                        }
                                                    })
                                                }
                                            />
                                            <label htmlFor='hideManagedFields'>Hide Managed Fields</label>
                                        </div>
                                        <YamlEditor
                                            input={live}
                                            hideModeButtons={!live}
                                            vScrollbar={live}
                                            onSave={(patch, patchType) =>
                                                services.applications.patchResource(
                                                    props.application.metadata.name,
                                                    props.application.metadata.namespace,
                                                    props.node,
                                                    patch,
                                                    patchType
                                                )
                                            }
                                        />
                                    </React.Fragment>
                                ) : (
                                    <div className='application-node-info__err_msg'>
                                        Resource not found in cluster:{' '}
                                        {`${props?.controlled?.state?.targetState?.apiVersion}/${props?.controlled?.state?.targetState?.kind}:${props.node.name}`}
                                        <br />
                                        {props?.controlled?.state?.normalizedLiveState?.apiVersion && (
                                            <span>
                                                Please update your resource specification to use the latest Kubernetes API resources supported by the target cluster. The
                                                recommended syntax is{' '}
                                                {`${props.controlled.state.normalizedLiveState.apiVersion}/${props?.controlled.state.normalizedLiveState?.kind}:${props.node.name}`}
                                            </span>
                                        )}
                                    </div>
                                )}
                            </React.Fragment>
                        );
                    }}
                </DataLoader>
            )
        }
    ];
    if (props.controlled && !props.controlled.summary.hook) {
        tabs.push({
            key: 'diff',
            icon: 'fa fa-file-medical',
            title: 'Diff',
            content: <ApplicationResourcesDiff states={[props.controlled.state]} />
        });
        tabs.push({
            key: 'desiredManifest',
            title: 'Desired Manifest',
            content: <YamlEditor input={props.controlled.state.targetState} hideModeButtons={true} />
        });
    }

    const readinessGatesState = React.useMemo(() => {
        // If containers are not ready then readiness gate status is not important.
        if (!props.live?.status?.containerStatuses?.length) {
            return null;
        }
        if (props.live?.status?.containerStatuses?.some((containerStatus: {ready: boolean}) => !containerStatus.ready)) {
            return null;
        }

        if (props.live && props.node?.kind === 'Pod') {
            return getPodReadinessGatesState(props.live);
        }

        return null;
    }, [props.live, props.node]);

    return (
        <div>
            {Boolean(readinessGatesState) && <ReadinessGatesNotPassedWarning readinessGatesState={readinessGatesState} />}
            <div className='white-box'>
                <div className='white-box__details'>
                    {attributes.map(attr => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>{attr.title}</div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>

            <div className='application-node-info__manifest'>
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => (
                        <Tabs
                            selectedTabKey={(tabs.length > 1 && pref.appDetails.resourceView) || 'manifest'}
                            tabs={tabs}
                            onTabSelected={selected => {
                                services.viewPreferences.updatePreferences({appDetails: {...pref.appDetails, resourceView: selected as any}});
                            }}
                        />
                    )}
                </DataLoader>
            </div>
        </div>
    );
};
