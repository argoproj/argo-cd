import {DropDown, Tooltip} from 'argo-ui';
import * as React from 'react';
import * as classNames from 'classnames';
import * as models from '../../../shared/models';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {ComparisonStatusIcon, HealthStatusIcon, nodeKey, createdOrNodeKey, isSameNode} from '../utils';
import {AppDetailsPreferences} from '../../../shared/services';
import {Consumer} from '../../../shared/context';
import Moment from 'react-moment';
import {format} from 'date-fns';
import {ResourceNode} from '../../../shared/models';
import './application-resource-list.scss';

export interface ApplicationResourceListProps {
    pref: AppDetailsPreferences;
    resources: models.ResourceStatus[];
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    tree?: models.ApplicationTree;
}

export const ApplicationResourceList = (props: ApplicationResourceListProps) => {
    const nodeByKey = new Map<string, models.ResourceNode>();
    props.tree?.nodes?.forEach(res => nodeByKey.set(nodeKey(res), res));

    const firstParentNode = props.resources.length > 0 && (nodeByKey.get(nodeKey(props.resources[0])) as ResourceNode)?.parentRefs?.[0];
    const isSameParent = firstParentNode && props.resources?.every(x => (nodeByKey.get(nodeKey(x)) as ResourceNode)?.parentRefs?.every(p => isSameNode(p, firstParentNode)));
    const isSameKind = props.resources?.every(x => x.group === props.resources[0].group && x.kind === props.resources[0].kind);
    const view = props.pref.view;

    const ParentRefDetails = () => {
        return isSameParent ? (
            <div className='resource-parent-node-info-title'>
                <div>Parent Node Info</div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Name:</div>
                    <div>{firstParentNode.name}</div>
                </div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Kind:</div>
                    <div>{firstParentNode.kind}</div>
                </div>
            </div>
        ) : (
            <div />
        );
    };
    return (
        props.resources.length > 0 && (
            <div>
                {/* Display only when the view is set to  or network */}
                {(view === 'tree' || view === 'network') && (
                    <div className='resource-details__header' style={{paddingTop: '20px'}}>
                        <ParentRefDetails />
                    </div>
                )}
                <div className='argo-table-list argo-table-list--clickable'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-1 xxxlarge-1' />
                            <div className='columns small-2 xxxlarge-1'>NAME</div>
                            <div className='columns small-1 xxxlarge-1'>GROUP/KIND</div>
                            <div className='columns small-1 xxxlarge-1'>SYNC ORDER</div>
                            <div className='columns small-2 xxxlarge-1'>NAMESPACE</div>
                            {isSameKind && props.resources[0].kind === 'ReplicaSet' && <div className='columns small-1 xxxlarge-1'>REVISION</div>}
                            <div className='columns small-2 xxxlarge-1'>CREATED AT</div>
                            <div className='columns small-2 xxxlarge-1'>STATUS</div>
                        </div>
                    </div>
                    {props.resources
                        .sort((first, second) => -createdOrNodeKey(first).localeCompare(createdOrNodeKey(second)))
                        .map(res => {
                            const groupkindjoin = [res.group, res.kind].filter(item => !!item).join('/');
                            return (
                                <div
                                    key={nodeKey(res)}
                                    className={classNames('argo-table-list__row', {
                                        'application-resource-tree__node--orphaned': res.orphaned
                                    })}
                                    onClick={() => props.onNodeClick && props.onNodeClick(nodeKey(res))}>
                                    <div className='row'>
                                        <div className='columns small-1 xxxlarge-1'>
                                            <div className='application-details__resource-icon'>
                                                <ResourceIcon kind={res.kind} />
                                                <br />
                                                <div>{ResourceLabel({kind: res.kind})}</div>
                                            </div>
                                        </div>
                                        <Tooltip content={res.name} enabled={!!res.name}>
                                            <div className='columns small-2 xxxlarge-1 application-details__item'>
                                                <span className='application-details__item_text'>{res.name}</span>
                                                {res.kind === 'Application' && (
                                                    <Consumer>
                                                        {ctx => (
                                                            <span className='application-details__external_link'>
                                                                <a
                                                                    href={ctx.baseHref + 'applications/' + res.namespace + '/' + res.name}
                                                                    onClick={e => e.stopPropagation()}
                                                                    title='Open application'>
                                                                    <i className='fa fa-external-link-alt' />
                                                                </a>
                                                            </span>
                                                        )}
                                                    </Consumer>
                                                )}
                                            </div>
                                        </Tooltip>
                                        <Tooltip content={groupkindjoin}>
                                            <div className='columns small-1 xxxlarge-1'>{groupkindjoin}</div>
                                        </Tooltip>
                                        <Tooltip content={res.syncWave} enabled={!!res.syncWave}>
                                            <div className='columns small-1 xxxlarge-1'>{res.syncWave || '-'}</div>
                                        </Tooltip>
                                        <Tooltip content={res.namespace} enabled={!!res.namespace}>
                                            <div className='columns small-2 xxxlarge-1'>{res.namespace}</div>
                                        </Tooltip>
                                        {isSameKind &&
                                            res.kind === 'ReplicaSet' &&
                                            ((nodeByKey.get(nodeKey(res)) as ResourceNode).info || [])
                                                .filter(tag => !tag.name.includes('Node'))
                                                .slice(0, 4)
                                                .map((tag, i) => {
                                                    return (
                                                        <div key={i} className='columns small-1 xxxlarge-1'>
                                                            {tag?.value?.split(':')[1] || '-'}
                                                        </div>
                                                    );
                                                })}
                                        <Tooltip content={res.createdAt} enabled={!!res.createdAt}>
                                            <div className='columns small-2 xxxlarge-1'>
                                                {res.createdAt && (
                                                    <span>
                                                        <Moment fromNow={true} ago={true}>
                                                            {res.createdAt}
                                                        </Moment>
                                                        &nbsp;ago &nbsp; {format(new Date(res.createdAt), 'MM/dd/yy')}
                                                    </span>
                                                )}
                                            </div>
                                        </Tooltip>
                                        <div className='columns small-2 xxxlarge-1'>
                                            {res.health && (
                                                <React.Fragment>
                                                    <HealthStatusIcon state={res.health} /> {res.health.status} &nbsp;
                                                </React.Fragment>
                                            )}
                                            {res.status && <ComparisonStatusIcon status={res.status} resource={res} label={true} />}
                                            {res.hook && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                                            {props.nodeMenu && (
                                                <div className='application-details__node-menu'>
                                                    <DropDown
                                                        isMenu={true}
                                                        anchor={() => (
                                                            <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                <i className='fa fa-ellipsis-v' />
                                                            </button>
                                                        )}>
                                                        {() => props.nodeMenu(nodeByKey.get(nodeKey(res)))}
                                                    </DropDown>
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            );
                        })}
                </div>
            </div>
        )
    );
};
