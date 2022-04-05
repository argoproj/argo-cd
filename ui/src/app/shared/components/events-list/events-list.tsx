import * as moment from 'moment';
import * as React from 'react';
import {ago} from 'argo-ui/v2';

import * as models from '../../models';
import {SelectNode} from '../../../applications/components/application-details/application-details';
import {Context} from '../../context';
import {ApplicationTree, ObjectReference, ResourceNode} from '../../models';
import {ResourceIcon} from '../../../applications/components/resource-icon';
import {ResourceLabel} from '../../../applications/components/resource-label';

require('./events-list.scss');

function timestampSort(first: models.Event, second: models.Event): number {
    if (first.lastTimestamp && !second.lastTimestamp) {
        return moment(second.eventTime).diff(first.lastTimestamp);
    } else if (!first.lastTimestamp && second.lastTimestamp) {
        return moment(second.lastTimestamp).diff(first.eventTime);
    } else if (!first.lastTimestamp && !second.lastTimestamp) {
        return moment(second.eventTime).diff(first.eventTime);
    }
    return moment(second.lastTimestamp).diff(first.lastTimestamp);
}

function getTimeElements(timestamp: string) {
    const readableFormat = (d: Date) => moment(d).calendar();
    const dateOfEvent = new Date(timestamp);
    return (
        <>
            <div className='events-list__event__ago'>{ago(dateOfEvent)}</div>
            <div className='events-list__event__time'>{readableFormat(dateOfEvent)}</div>
        </>
    );
}

export const EventsList = (props: {events: models.Event[]; showResourceLink?: boolean; appNodes?: ResourceNode[]; appName?: string; onResourceClicked?: () => void}) => {
    const events = props.events.sort((first, second) => timestampSort(first, second));
    const appContext = React.useContext(Context);

    function getVersion(involvedObject: ObjectReference) {
        let group = '';
        let version = '';
        const groupKind = involvedObject.apiVersion.split('/');
        if (groupKind.length > 1) {
            [group, version] = groupKind;
        } else {
            version = groupKind[0];
        }
        return [group, version];
    }

    const hasNode = (involvedObject: models.ObjectReference, appNodes?: ResourceNode[], appName?: string): boolean => {
        if (!appNodes) {
            return false;
        }

        if (involvedObject.kind === 'Application' && involvedObject.name === appName) {
            return true;
        }

        return appNodes.some(
            node => node.uid === involvedObject.uid && node.name === involvedObject.name && node.namespace === involvedObject.namespace && node.kind === involvedObject.kind
        );
    };

    const selectNode = (involvedObject: models.ObjectReference) => {
        if (props.onResourceClicked) {
            props.onResourceClicked();
        }
        const [group] = getVersion(involvedObject);
        const fullName = [group, involvedObject.kind, involvedObject.namespace, involvedObject.name].join('/');
        SelectNode(fullName, 0, involvedObject.kind === 'Application' ? 'event' : 'events', appContext);
    };
    return (
        <div className='events-list'>
            {(events.length === 0 && <p>No events available</p>) || (
                <div className='argo-table-list'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            {props.showResourceLink && (
                                <React.Fragment>
                                    <div className='columns small-1 xxlarge-1'>KIND</div>
                                    <div className='columns small-2 xxlarge-2'>RESOURCE</div>
                                </React.Fragment>
                            )}
                            <div className='columns small-2 xxlarge-2'>REASON</div>
                            <div className={`columns small-${props.showResourceLink ? '2' : '4'} xxlarge-${props.showResourceLink ? '3' : '5'}`}>MESSAGE</div>
                            <div className='columns small-2 xxlarge-1'>COUNT</div>
                            <div className={`columns small-${props.showResourceLink ? '1' : '2'} xxlarge-${props.showResourceLink ? '1' : '2'}`}>FIRST OCCURRED</div>
                            <div className={`columns small-${props.showResourceLink ? '1' : '2'} xxlarge-${props.showResourceLink ? '1' : '2'}`}>LAST OCCURRED</div>
                        </div>
                    </div>
                    {events.map(event => {
                        const eventHasNode = hasNode(event.involvedObject, props.appNodes, props.appName);
                        return (
                            <div className={`argo-table-list__row events-list__event events-list__event--${event.type}`} key={event.metadata.uid}>
                                <div className='row'>
                                    {props.showResourceLink && (
                                        <React.Fragment>
                                            <div className='columns small-1 xxlarge-1'>
                                                <div className='events-list__event__node-kind-icon'>
                                                    <ResourceIcon kind={event.involvedObject.kind} />
                                                    <br />
                                                    {ResourceLabel({kind: event.involvedObject.kind})}
                                                </div>
                                            </div>
                                            <div className='columns small-2 xxlarge-2'>
                                                <button
                                                    className={'resource-name' + (eventHasNode ? ' resource-link' : '')}
                                                    title={"View this resource's events"}
                                                    onClick={eventHasNode ? () => selectNode(event.involvedObject) : undefined}>
                                                    {event.involvedObject.name}
                                                </button>
                                            </div>
                                        </React.Fragment>
                                    )}
                                    <div className='columns small-2 xxlarge-2'>{event.reason}</div>
                                    <div className={`columns small-${props.showResourceLink ? '2' : '4'} xxlarge-${props.showResourceLink ? '3' : '5'}`}>{event.message}</div>
                                    <div className='columns small-2 xxlarge-1'>{event.count}</div>
                                    <div className={`columns small-${props.showResourceLink ? '1' : '2'} xxlarge-${props.showResourceLink ? '1' : '2'}`}>
                                        {event.firstTimestamp ? getTimeElements(event.firstTimestamp) : getTimeElements(event.eventTime)}
                                    </div>
                                    <div className={`columns small-${props.showResourceLink ? '1' : '2'} xxlarge-${props.showResourceLink ? '1' : '2'}`}>
                                        {event.lastTimestamp ? getTimeElements(event.lastTimestamp) : getTimeElements(event.eventTime)}
                                    </div>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </div>
    );
};
