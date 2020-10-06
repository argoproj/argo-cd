import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as React from 'react';

import {GetNodes} from './pod-view-mock-service';
import './pod-view.scss';

export interface Node {
    name: string;
    cpu: Stat;
    mem: Stat;
    pods: Pod[];
}

export interface Pod {
    name: string;
    status: PodStatus;
}

export enum PodStatus {
    Healthy = 'healthy',
    OutOfSync = 'out-of-sync',
    Degraded = 'degraded'
}

export type PodStatuses = keyof typeof PodStatus;

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
        <div className='node white-box' key={node.name}>
            <div className='node__container node__container--header'>
                <span>{node.name.toUpperCase()}</span>
                <i className='fa fa-info-circle' style={{marginLeft: 'auto'}} />
                <DropDownMenu
                    anchor={() => (
                        <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                            <i className='fa fa-ellipsis-v' />
                        </button>
                    )}
                    items={[{title: 'hello', action: () => null}, {title: 'world', action: () => null}]}
                />
            </div>
            <div className='node__container'>
                <div className='node__container node__container--stats'>
                    {Stat(node.cpu)}
                    {Stat(node.mem)}
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
        <Tooltip content={pod.name}>
            <div className={`node__pod node__pod--${pod.status}`} />
        </Tooltip>
    );
}

function Stat(stat: Stat) {
    return (
        <div className='node__pod__stat node__container'>
            <Tooltip content={`${stat.cur} / ${stat.max} used`}>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * (stat.cur / stat.max)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{stat.name.toUpperCase()}</div>
        </div>
    );
}
