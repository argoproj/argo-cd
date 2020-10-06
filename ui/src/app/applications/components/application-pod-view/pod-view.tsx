import {Tooltip} from 'argo-ui';
import * as React from 'react';

import './pod-view.scss';
import {GetNodes} from "./pod-view-mock-service";

export interface Node {
    name: string;
    maxCPU: number;
    maxMem: number;
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

export class PodView extends React.Component {
    public render() {
        return <div style={{display: 'flex'}}>{GetNodes().map(n => Node(n))}</div>;
    }
}

function Node(node: Node) {
    return (
        <div className='node white-box'>
            <div className='node__container node__container--header'>
                <p>{node.name.toUpperCase()}</p>
            </div>
            <div className='node__container'>
                <div className='node__container node__container--stats'>
                    {Stat(10, node.maxCPU, 'CPU')}
                    {Stat(220, node.maxMem, 'MEM')}
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
            <div className={`node__pod node__pod--${PodStatus.Healthy}`} />
        </Tooltip>
    );
}

function Stat(cur: number, max: number, label: string) {
    return (
        <div className='node__pod__stat node__container'>
            <Tooltip content={`${cur} / ${max} used`}>
                <div className='node__pod__stat__bar'>
                    <div className='node__pod__stat__bar--fill' style={{height: `${100 * (cur / max)}%`}} />
                </div>
            </Tooltip>
            <div className='node__label'>{label}</div>
        </div>
    );
}
