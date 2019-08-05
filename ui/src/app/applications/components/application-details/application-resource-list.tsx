import { DropDown } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { ComparisonStatusIcon, HealthStatusIcon, ICON_CLASS_BY_KIND, nodeKey } from '../utils';

export const ApplicationResourceList = ({ resources, onNodeClick, nodeMenu }: {
    resources: models.ResourceStatus[],
    onNodeClick?: (fullName: string) => any,
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode,
}) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-3 xxxlarge-3'>NAME</div>
                <div className='columns small-3 xxxlarge-4'>GROUP/KIND</div>
                <div className='columns small-4 xxxlarge-4'>NAMESPACE</div>
                <div className='columns small-2 xxxlarge-1'>STATUS</div>
            </div>
        </div>
        {resources.sort((first, second) => nodeKey(first).localeCompare(nodeKey(second))).map((res) => (
            <div key={nodeKey(res)} className='argo-table-list__row' onClick={() => onNodeClick(nodeKey(res))}>
                <div className='row'>
                    <div className='columns small-3 xxxlarge-3'>
                        <i className={ICON_CLASS_BY_KIND[res.kind.toLocaleLowerCase()] || 'fa fa-cogs'}/> <span>{res.name}</span>
                    </div>
                    <div className='columns small-3 xxxlarge-4'>{[res.group, res.kind].filter((item) => !!item).join('/')}</div>
                    <div className='columns small-4 xxxlarge-4'>{res.namespace}</div>
                    <div className='columns small-2 xxxlarge-1'>
                        {res.health && <React.Fragment><HealthStatusIcon state={res.health}/> {res.health.status} &nbsp;</React.Fragment>}
                        {res.status && <ComparisonStatusIcon status={res.status} resource={res} label={true}/>}
                        {res.hook && (<i title='Resource lifecycle hook' className='fa fa-anchor' />)}
                        <div className='application-details__node-menu'>
                            <DropDown isMenu={true} anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                <i className='fa fa-ellipsis-v'/>
                            </button>}>
                            {() => nodeMenu({
                                name: res.name,
                                version: res.version,
                                kind: res.kind,
                                namespace: res.namespace,
                                group: res.group,
                                info: null,
                                uid: '',
                                resourceVersion: null,
                                parentRefs: [],
                            })}
                            </DropDown>
                        </div>
                    </div>
                </div>
            </div>
        ))}
    </div>
);
