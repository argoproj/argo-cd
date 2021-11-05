import {DropDown} from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {ComparisonStatusIcon, HealthStatusIcon, nodeKey} from '../utils';
import {Consumer} from '../../../shared/context';

export const ApplicationResourceList = ({
    resources,
    onNodeClick,
    nodeMenu
}: {
    resources: models.ResourceStatus[];
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
}) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-1 xxxlarge-1' />
                <div className='columns small-2 xxxlarge-2'>NAME</div>
                <div className='columns small-3 xxxlarge-4'>GROUP/KIND</div>
                <div className='columns small-3 xxxlarge-3'>NAMESPACE</div>
                <div className='columns small-2 xxxlarge-2'>STATUS</div>
            </div>
        </div>
        {resources
            .sort((first, second) => nodeKey(first).localeCompare(nodeKey(second)))
            .map(res => (
                <div key={nodeKey(res)} className='argo-table-list__row' onClick={() => onNodeClick(nodeKey(res))}>
                    <div className='row'>
                        <div className='columns small-1 xxxlarge-1'>
                            <div className='application-details__resource-icon'>
                                <ResourceIcon kind={res.kind} />
                                <br />
                                <div>{ResourceLabel({kind: res.kind})}</div>
                            </div>
                        </div>
                        <div className='columns small-2 xxxlarge-2'>
                            {res.name}
                            {res.kind === 'Application' && (
                                <Consumer>
                                    {ctx => (
                                        <span className='application-details__external_link'>
                                            <a href={ctx.baseHref + 'applications/' + res.name} title='Open application'>
                                                <i className='fa fa-external-link-alt' />
                                            </a>
                                        </span>
                                    )}
                                </Consumer>
                            )}
                        </div>
                        <div className='columns small-3 xxxlarge-4'>{[res.group, res.kind].filter(item => !!item).join('/')}</div>
                        <div className='columns small-3 xxxlarge-3'>{res.namespace}</div>
                        <div className='columns small-2 xxxlarge-2'>
                            {res.health && (
                                <React.Fragment>
                                    <HealthStatusIcon state={res.health} /> {res.health.status} &nbsp;
                                </React.Fragment>
                            )}
                            {res.status && <ComparisonStatusIcon status={res.status} resource={res} label={true} />}
                            {res.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                            <div className='application-details__node-menu'>
                                <DropDown
                                    isMenu={true}
                                    anchor={() => (
                                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                            <i className='fa fa-ellipsis-v' />
                                        </button>
                                    )}>
                                    {() =>
                                        nodeMenu({
                                            name: res.name,
                                            version: res.version,
                                            kind: res.kind,
                                            namespace: res.namespace,
                                            group: res.group,
                                            info: null,
                                            uid: '',
                                            resourceVersion: null,
                                            parentRefs: []
                                        })
                                    }
                                </DropDown>
                            </div>
                        </div>
                    </div>
                </div>
            ))}
    </div>
);
