import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as React from 'react';

import {Node, Pod, ResourceStat} from '../../../shared/models';
import {GetNodes} from './pod-view-mock-service';
import './pod-view.scss';

export interface Stat {
    name: string;
    cur: number;
    max: number;
}

export class PodView extends React.Component {
    public render() {
        return <DataLoader load={() => GetNodes(5)}>{nodes => <div className='nodes-container'>{nodes.map(n => Node(n))}</div>}</DataLoader>;
    }
}

function Node(node: Node) {
    return (
        <div className='node white-box' key={node.metadata.name}>
            <div className='node__container node__container--header'>
                <span>{node.metadata.name.toUpperCase()}</span>
                <i className='fa fa-info-circle' style={{marginLeft: 'auto'}} />
                <DropDownMenu
                    anchor={() => (
                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                            <i className='fa fa-ellipsis-v' />
                        </button>
                    )}
                    items={[
                        {
                            title: (
                                <React.Fragment>
                                    <i className='fa fa-info-circle' /> Node Details
                                </React.Fragment>
                            ),
                            action: () => null
                        },
                        {
                            title: (
                                <React.Fragment>
                                    <i className='fa fa-history' /> History
                                </React.Fragment>
                            ),
                            action: () => null
                        },
                        {
                            title: (
                                <React.Fragment>
                                    <i className='fa fa-times-circle' /> Delete
                                </React.Fragment>
                            ),
                            action: () => null
                        },
                        {
                            title: (
                                <React.Fragment>
                                    <i className='fa fa-redo' /> Refresh
                                </React.Fragment>
                            ),
                            action: () => null
                        }
                    ]}
                />
            </div>
            <div className='node__container'>
                <div className='node__container node__container--stats'>
                    {node.status.capacity.map(r => {
                        Stat(r);
                    })}
                </div>
                <div className='node__pod-container node__container'>
                    <div className='node__pod-container__pods'>{node.pods.map(p => Pod(p))}</div>
                    <div className='node__label'>PODS</div>
                </div>
            </div>
        </div>
    );
}

function Pod(pod: Pod) {
    return (
        <Tooltip content={pod.metadata.name}>
            <div className={`node__pod node__pod--${pod.status}`} />
        </Tooltip>
    );
}

function Stat(stat: ResourceStat) {
    return (
        <div className='node__pod__stat node__container'>
            <Tooltip content={`${stat.used} / ${stat.quantity} used`}>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * (stat.used / stat.quantity)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{stat.name.toUpperCase()}</div>
        </div>
    );
}
