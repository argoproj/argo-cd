import {DropDown} from 'argo-ui';
import * as React from 'react';
import * as classNames from 'classnames';
import * as models from '../../../shared/models';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {ComparisonStatusIcon, HealthStatusIcon, nodeKey, createdOrNodeKey} from '../utils';
import {Consumer} from '../../../shared/context';
import * as _ from 'lodash';
import Moment from 'react-moment';
import {format} from 'date-fns';
import {ResourceNode, ResourceRef} from '../../../shared/models';
import './application-resource-list.scss';

export const ApplicationResourceList = ({
    resources,
    onNodeClick,
    nodeMenu,
    tree
}: {
    resources: models.ResourceStatus[];
    onNodeClick?: (fullName: string) => any;
    nodeMenu?: (node: models.ResourceNode) => React.ReactNode;
    tree?: models.ApplicationTree;
}) => {
    function getResNode(nodes: ResourceNode[], nodeId: string): models.ResourceNode {
        for (const node of nodes) {
            if (nodeKey(node) === nodeId) {
                return node;
            }
        }
        return null;
    }
    const parentNode = ((resources || []).length > 0 && (getResNode(tree.nodes, nodeKey(resources[0])) as ResourceNode)?.parentRefs?.[0]) || ({} as ResourceRef);
    const searchParams = new URLSearchParams(window.location.search);
    const view = searchParams.get('view');

    const ParentRefDetails = () => {
        return Object.keys(parentNode).length > 0 ? (
            <div className='resource-parent-node-info-title'>
                <div>Parent Node Info</div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Name:</div>
                    <div>{parentNode?.name}</div>
                </div>
                <div className='resource-parent-node-info-title__label'>
                    <div>Kind:</div>
                    <div>{parentNode?.kind}</div>
                </div>
            </div>
        ) : (
            <div />
        );
    };
    return (
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
                        {(parentNode.kind === 'Rollout' || parentNode.kind === 'Deployment') && <div className='columns small-1 xxxlarge-1'>REVISION</div>}
                        <div className='columns small-2 xxxlarge-1'>CREATED AT</div>
                        <div className='columns small-2 xxxlarge-1'>STATUS</div>
                    </div>
                </div>
                {resources
                    .sort((first, second) => -createdOrNodeKey(first).localeCompare(createdOrNodeKey(second)))
                    .map(res => (
                        <div
                            key={nodeKey(res)}
                            className={classNames('argo-table-list__row', {
                                'application-resource-tree__node--orphaned': res.orphaned
                            })}
                            onClick={() => onNodeClick(nodeKey(res))}>
                            <div className='row'>
                                <div className='columns small-1 xxxlarge-1'>
                                    <div className='application-details__resource-icon'>
                                        <ResourceIcon kind={res.kind} />
                                        <br />
                                        <div>{ResourceLabel({kind: res.kind})}</div>
                                    </div>
                                </div>
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
                                <div className='columns small-1 xxxlarge-1'>{[res.group, res.kind].filter(item => !!item).join('/')}</div>
                                <div className='columns small-1 xxxlarge-1'>{res.syncWave || '-'}</div>
                                <div className='columns small-2 xxxlarge-1'>{res.namespace}</div>
                                {res.kind === 'ReplicaSet' &&
                                    ((getResNode(tree.nodes, nodeKey(res)) as ResourceNode).info || [])
                                        .filter(tag => !tag.name.includes('Node'))
                                        .slice(0, 4)
                                        .map((tag, i) => {
                                            return (
                                                <div key={i} className='columns small-1 xxxlarge-1'>
                                                    {tag?.value?.split(':')[1] || '-'}
                                                </div>
                                            );
                                        })}

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
                                <div className='columns small-2 xxxlarge-1'>
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
                                            {nodeMenu({
                                                name: res.name,
                                                version: res.version,
                                                kind: res.kind,
                                                namespace: res.namespace,
                                                group: res.group,
                                                info: null,
                                                uid: '',
                                                resourceVersion: null,
                                                parentRefs: []
                                            })}
                                        </DropDown>
                                    </div>
                                </div>
                            </div>
                        </div>
                    ))}
            </div>
        </div>
    );
};
