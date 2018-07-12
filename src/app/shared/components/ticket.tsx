import * as moment from 'moment';
import * as React from 'react';
import {Observable, Subscription} from 'rxjs';

export class Ticker extends React.Component<{intervalMs?: number, children?: ((time: moment.Moment) => React.ReactNode)}, {time: moment.Moment}> {

    private subscription: Subscription;

    constructor(props: {intervalMs?: number, children?: ((time: moment.Moment) => React.ReactNode)}) {
        super(props);
        this.state = { time: moment() };
        this.subscription = Observable.interval(props.intervalMs || 1000).subscribe(() => this.setState({ time: moment() }));
    }

    public render() {
        return this.props.children(this.state.time);
    }

    public componentWillUnmount() {
        if (this.subscription != null) {
            this.subscription.unsubscribe();
            this.subscription = null;
        }
    }
}
