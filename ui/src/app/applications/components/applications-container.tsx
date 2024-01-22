import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';
import {ApplicationDetailsForApplications, ApplicationDetailsForApplicationSets} from './application-details/application-details';
import {ApplicationFullscreenLogs} from './application-fullscreen-logs/application-fullscreen-logs';
import {ApplicationsList} from './applications-list/applications-list';
// import {ApplicationSetsList, ApplicationsList} from './applications-list/applications-list';

export const ApplicationsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} render={() => <ApplicationsList objectListKind="application" {...props}/>} />
        <Route exact={true} path={`${props.match.path}/:name`} render={(props) => 
            props.match.path.includes("applicationset") ? (
                <ApplicationDetailsForApplicationSets  objectListKind='applicationset' {...props} />
            ) : (
                <ApplicationDetailsForApplications  objectListKind='application' {...props} />
            )
         } />
        <Route exact={true} path={`${props.match.path}/:appnamespace/:name`} render={(props) => 
            props.match.path.includes("applicationset") ? (
                <ApplicationDetailsForApplicationSets objectListKind='applicationset' {...props} />
            ) : (
                <ApplicationDetailsForApplications objectListKind='application' {...props} />
            )
         } />
        <Route exact={true} path={`${props.match.path}/:name/:namespace/:container/logs`} component={ApplicationFullscreenLogs} />
        <Route exact={true} path={`${props.match.path}/:appnamespace/:name/:namespace/:container/logs`} component={ApplicationFullscreenLogs} />

        {/* <Route exact={true} path={`${props.match.path}`} component={ApplicationSetsList} /> */}
    </Switch>
);
