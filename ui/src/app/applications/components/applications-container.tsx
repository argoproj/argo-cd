import * as React from 'react';
import { Route, RouteComponentProps, Switch } from 'react-router';
import { ApplicationDetails } from './application-details/application-details';
import { ApplicationsList } from './applications-list/applications-list';

export const ApplicationsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={ApplicationsList}/>
        <Route exact={true} path={`${props.match.path}/:name`} component={ApplicationDetails}/>
    </Switch>
);
