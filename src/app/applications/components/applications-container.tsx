import * as React from 'react';
import { Route, RouteComponentProps, Switch } from 'react-router';
import { ApplicationsList } from './applications-list/applications-list';

export const ApplicationsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={ApplicationsList}/>
    </Switch>
);
