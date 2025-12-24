import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';
import {ApplicationDetails} from './application-details/application-details';
import {ApplicationFullscreenLogs} from './application-fullscreen-logs/application-fullscreen-logs';
import {ApplicationsList} from './applications-list/applications-list';

export const ApplicationsContainer = (props: RouteComponentProps<any>) => {
    // Determine objectListKind from the route path
    const objectListKind = props.match.path.includes('/applicationsets') ? 'applicationset' : 'application';

    return (
        <Switch>
            <Route exact={true} path={`${props.match.path}`} render={() => <ApplicationsList objectListKind={objectListKind} {...props} />} />
            <Route exact={true} path={`${props.match.path}/:name`} render={routeProps => <ApplicationDetails objectListKind={objectListKind} {...routeProps} />} />
            <Route exact={true} path={`${props.match.path}/:appnamespace/:name`} render={routeProps => <ApplicationDetails objectListKind={objectListKind} {...routeProps} />} />
            <Route exact={true} path={`${props.match.path}/:name/:namespace/:container/logs`} component={ApplicationFullscreenLogs} />
            <Route exact={true} path={`${props.match.path}/:appnamespace/:name/:namespace/:container/logs`} component={ApplicationFullscreenLogs} />
        </Switch>
    );
};
