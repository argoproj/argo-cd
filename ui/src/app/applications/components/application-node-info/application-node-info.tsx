import {Checkbox, DataLoader, Tab, Tabs} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';

import {YamlEditor, ClipboardText} from '../../../shared/components';
import {DeepLinks} from '../../../shared/components/deep-links';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ResourceTreeNode} from '../application-resource-tree/application-resource-tree';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import {
    ComparisonStatusIcon,
    formatCreationTimestamp,
    getPodReadinessGatesState,
    getPodReadinessGatesState as _getPodReadinessGatesState,
    getPodStateReason,
    HealthStatusIcon
} from '../utils';

import './application-node-info.scss';
import {ReadinessGatesFailedWarning} from './readiness-gates-failed-warning';

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
            const {reason, message} = getPodStateReason(props.live);
            attributes.push({title: 'STATE', value: reason});
            if (message) {
                attributes.push({title: 'STATE DETAILS', value: message});
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
                        if (live?.metadata?.managedFields && pref.appDetails.hideManagedFields) {
                            delete live.metadata.managedFields;
                        }
                        return (
                            <>
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
                                        services.applications.patchResource(props.application.metadata.name, props.application.metadata.namespace, props.node, patch, patchType)
                                    }
                                />
                            </>
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
        if (props.live && props.node?.kind === 'Pod') {
            return getPodReadinessGatesState(props.live);
        }

        return null;
    }, [props.live, props.node]);

    return (
        <div>
            {Boolean(readinessGatesState) && <ReadinessGatesFailedWarning readinessGatesState={readinessGatesState} />}
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
