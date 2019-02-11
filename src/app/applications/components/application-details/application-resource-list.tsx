import { DropDownMenu, MenuItem } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { ComparisonStatusIcon, HealthStatusIcon, ICON_CLASS_BY_KIND, nodeKey } from '../utils';

export const ApplicationResourceList = ({ resources, onNodeClick, nodeMenuItems }: {
    resources: models.ResourceStatus[],
    onNodeClick?: (fullName: string) => any,
    nodeMenuItems?: (node: models.ResourceNode) => MenuItem[],
}) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-3'>NAME</div>
                <div className='columns small-3'>GROUP/KIND</div>
                <div className='columns small-2'>NAMESPACE</div>
                <div className='columns small-2'>SYNC</div>
                <div className='columns small-2'>HEALTH</div>
            </div>
        </div>
        {resources.map((res) => (
            <div key={nodeKey(res)} className='argo-table-list__row' onClick={() => onNodeClick(nodeKey(res))}>
                <div className='row'>
                    <div className='columns small-3'>
                        <i className={ICON_CLASS_BY_KIND[res.kind.toLocaleLowerCase()] || 'fa fa-gears'}/> <span>{res.name}</span>
                    </div>
                    <div className='columns small-3'>{[res.group, res.kind].filter((item) => !!item).join('/')}</div>
                    <div className='columns small-2'>{res.namespace}</div>
                    <div className='columns small-2'>
                        {res.status && <ComparisonStatusIcon status={res.status}/>} {res.status}
                    </div>
                    <div className='columns small-2'>
                        {res.health && <HealthStatusIcon state={res.health}/>} {res.health.status}
                        {res.hook && (<i title='Resource lifecycle hook' className='fa fa-anchor' />)}
                        <div className='application-details__node-menu'>
                            <DropDownMenu anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                <i className='fa fa-ellipsis-v'/>
                            </button>} items={nodeMenuItems({
                                name: res.name,
                                version: res.version,
                                kind: res.kind,
                                namespace: res.namespace,
                                group: res.group,
                                info: null,
                                children: null,
                                resourceVersion: null,
                            })}/>
                        </div>
                    </div>
                </div>
            </div>
        ))}
    </div>
);
