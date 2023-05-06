import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';
// import {ApplicationDetails} from '../../applications/components/application-details/application-details';
import {ApplicationSetsList} from './applications-list/applications-list';

export const ApplicationsetsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={ApplicationSetsList} />
        {/* <Route exact={true} path={`${props.match.path}/:name`} component={ApplicationDetails} /> */}
        {/* <Route exact={true} path={`${props.match.path}/:appnamespace/:name`} component={ApplicationDetails} /> */}
    </Switch>
);
