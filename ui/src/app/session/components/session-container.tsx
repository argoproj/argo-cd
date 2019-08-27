import * as React from 'react';
import {  Route, RouteComponentProps, Switch } from 'react-router';

import { SessionOverview } from './session-overview/session-overview';

export const SessionContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={SessionOverview}/>
    </Switch>
);
