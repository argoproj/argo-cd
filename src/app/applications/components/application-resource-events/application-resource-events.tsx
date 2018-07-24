import { MockupList } from 'argo-ui';
import * as React from 'react';

import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

require('./application-resource-events.scss');

export class ApplicationResourceEvents extends React.Component<{ applicationName: string, resource?: models.ResourceNode }, { events: models.Event[] }> {

    constructor(props: {applicationName: string, resource?: models.ResourceNode }) {
        super(props);
        this.state = { events: null };
    }

    public async componentDidMount() {
        const events = this.props.resource ?
            services.applications.resourceEvents(this.props.applicationName, this.props.resource.state.metadata.uid, this.props.resource.state.metadata.name) :
            services.applications.events(this.props.applicationName);
        this.setState({ events: await events });
    }

    public render() {
        return (
            <div className='application-resource-events'>
                {this.state.events && (
                    this.state.events.length === 0 && (
                        <p>No events available</p>
                    ) || (
                        <div className='argo-table-list'>
                            <div className='argo-table-list__head'>
                                <div className='row'>
                                    <div className='columns small-2'>REASON</div>
                                    <div className='columns small-6'>MESSAGE</div>
                                    <div className='columns small-2'>COUNT</div>
                                    <div className='columns small-2'>EVENT TIMESTAMP</div>
                                </div>
                            </div>
                            {this.state.events.map((event) => (
                                <div className={`argo-table-list__row application-resource-events__event application-resource-events__event--${event.type}`}
                                     key={event.metadata.uid}>
                                    <div className='row'>
                                        <div className='columns small-2'>{event.reason}</div>
                                        <div className='columns small-6'>{event.message}</div>
                                        <div className='columns small-2'>{event.count}</div>
                                        <div className='columns small-2'>{event.firstTimestamp}</div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )
                ) || (
                    <MockupList height={50} marginTop={10}/>
                )}
            </div>
        );
    }
}
