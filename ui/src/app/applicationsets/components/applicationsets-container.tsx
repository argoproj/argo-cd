import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';

// NOTE: These components still live in the `applications/` module. They are imported here so the
// ApplicationSet route has its own dedicated entry point/container. A refactor should be made to
// move the ApplicationSet list and details views out of `applications/` into this module
// (and dropping the shared `objectListKind` prop) later on.
import {ApplicationDetails} from '../../applications/components/application-details/application-details';
import {ApplicationSetsList} from '../../applications/components/applications-list/application-sets-list';

export const ApplicationSetsContainer = (props: RouteComponentProps<any>) => {
    return (
        <Switch>
            <Route exact={true} path={`${props.match.path}`} render={() => <ApplicationSetsList {...(props as any)} />} />
            <Route exact={true} path={`${props.match.path}/:name`} render={routeProps => <ApplicationDetails objectListKind='applicationset' {...(routeProps as any)} />} />
            <Route
                exact={true}
                path={`${props.match.path}/:appnamespace/:name`}
                render={routeProps => <ApplicationDetails objectListKind='applicationset' {...(routeProps as any)} />}
            />
        </Switch>
    );
};
