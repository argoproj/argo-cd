import * as React from 'react';
import Moment from 'react-moment';
import {Pod, ResourceName} from '../../../shared/models';
import {isYoungerThanXMinutes} from '../utils';
import {formatMetric} from './pod-view';

export const PodTooltip = (props: {pod: Pod}) => {
    const pod = props.pod;

    return (
        <div>
            <div className='row'>
                <div className='columns small-12'>{pod.metadata.name}</div>
            </div>
            <div className='row'>
                <div className='columns small-6'>Health:</div>
                <div className='columns small-6'>{pod.health}</div>
            </div>
            {(pod.info || [])
                .filter(i => {
                    //filter out 0 values for CPU and mem on pod info
                    return i.name !== 'Node' && !((i.name === ResourceName.ResourceCPU || i.name === ResourceName.ResourceMemory) && parseInt(i.value, 10) === 0);
                })
                .map(i => {
                    const isPodRequests = i.name === ResourceName.ResourceCPU || i.name === ResourceName.ResourceMemory;
                    const formattedValue = isPodRequests ? formatMetric(i.name as ResourceName, parseInt(i.value, 10)) : i.value;

                    //this is just to show cpu and mem info with "Requests" as prefix
                    const label = i.name === ResourceName.ResourceCPU ? 'Requests CPU:' : i.name === ResourceName.ResourceMemory ? 'Requests MEM:' : `${i.name}:`;

                    return (
                        <div className='row' key={i.name}>
                            <div className='columns small-6' style={{whiteSpace: 'nowrap'}}>
                                {label}
                            </div>
                            <div className='columns small-6'>{formattedValue}</div>
                        </div>
                    );
                })}
            {pod.createdAt && (
                <div className='row'>
                    <div className='columns small-6'>
                        <span>Created: </span>
                    </div>
                    <div className='columns small-6'>
                        <Moment fromNow={true} ago={true}>
                            {pod.createdAt}
                        </Moment>
                        <span> ago</span>
                    </div>
                    {isYoungerThanXMinutes(pod, 30) && (
                        <div className='columns small-12'>
                            <span>
                                <i className='fas fa-circle circle-icon' /> &nbsp;
                                <span>pod age less than 30min</span>
                            </span>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};
