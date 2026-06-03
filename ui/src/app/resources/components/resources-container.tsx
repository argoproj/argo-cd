import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';

import {ResourcesList} from './resources-list/resources-list';

export const ResourceContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={ResourcesList} />
    </Switch>
);
