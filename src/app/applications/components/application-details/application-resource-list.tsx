import * as React from 'react';

import * as models from '../../../shared/models';
import { ComparisonStatusIcon, HealthStatusIcon, ICON_CLASS_BY_KIND, nodeKey } from '../utils';

export const ApplicationResourceList = ({ resources, onNodeClick }: { resources: models.ResourceStatus[], onNodeClick?: (fullName: string) => any }) => (
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
                    </div>
                </div>
            </div>
        ))}
    </div>
);
