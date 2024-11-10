import * as React from 'react';
import Moment from 'react-moment';
import {Pod} from '../../../shared/models';
import {isYoungerThanXMinutes, podRequests} from '../utils';
import {formatSize} from './pod-view';

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
                    return i.name !== 'Node' && !((i.name === podRequests.CPU || i.name === podRequests.MEMORY) && parseInt(i.value, 10) === 0);
                })
                .map(i => {
                    //formatted the values for cpu and mem
                    const formattedValue = formatPodMetric(i.name, i.value);
                    return (
                        <div className='row' key={i.name}>
                            <div className='columns small-6' style={{whiteSpace: 'nowrap'}}>
                                {i.name}:
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

function formatPodMetric(name: string, value: string) {
    const numericValue = parseInt(value, 10);

    switch (name) {
        case podRequests.CPU:
            return `${numericValue}m`;
        case podRequests.MEMORY: {
            return formatSize(numericValue / 1000); // divide by 1000 to convert "milli bytes" to bytes
        }
        default:
            return value;
    }
}
