import * as PropTypes from 'prop-types';
import * as React from 'react';
import {BehaviorSubject, Observable} from 'rxjs';
import {map} from 'rxjs/operators';

import {AppContext, Consumer} from '../context';

export const Query = (props: {children: (params: URLSearchParams) => React.ReactNode}) => (
    <Consumer>{ctx => props.children(new URLSearchParams(ctx.history.location.search))}</Consumer>
);

export interface ObservableQueryProps {
    children: (params: Observable<URLSearchParams>) => React.ReactNode;
}

export class ObservableQuery extends React.Component<ObservableQueryProps> {
    public static contextTypes = {
        router: PropTypes.object
    };

    private search: BehaviorSubject<string>;
    private stopListen: () => void;

    constructor(props: ObservableQueryProps) {
        super(props);
    }

    public componentWillMount() {
        this.search = new BehaviorSubject(this.appContext.router.history.location.search);
        this.stopListen = this.appContext.router.history.listen(location => {
            this.search.next(location.search);
        });
    }

    public componentWillUnmount() {
        if (this.stopListen) {
            this.stopListen();
            this.stopListen = null;
        }
    }

    public render() {
        return this.props.children(this.search.pipe(map(search => new URLSearchParams(search))));
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
