import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';

import {UserInfoOverview} from './user-info-overview/user-info-overview';

export const UserInfoContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={UserInfoOverview} />
    </Switch>
);
