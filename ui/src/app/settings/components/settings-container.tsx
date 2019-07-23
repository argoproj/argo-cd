import * as React from 'react';
import { Redirect, Route, RouteComponentProps, Switch } from 'react-router';

import { CertsList } from './certs-list/certs-list';
import { ClustersList } from './clusters-list/clusters-list';
import { ProjectDetails } from './project-details/project-details';
import { ProjectsList } from './projects-list/projects-list';
import { ReposList } from './repos-list/repos-list';
import { SettingsOverview } from './settings-overview/settings-overview';

export const SettingsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={SettingsOverview}/>
        <Route exact={true} path={`${props.match.path}/repos`} component={ReposList}/>
        <Route exact={true} path={`${props.match.path}/certs`} component={CertsList}/>
        <Route exact={true} path={`${props.match.path}/clusters`} component={ClustersList}/>
        <Route exact={true} path={`${props.match.path}/projects`} component={ProjectsList}/>
        <Route exact={true} path={`${props.match.path}/projects/:name`} component={ProjectDetails}/>
        <Redirect path='*' to={`${props.match.path}`}/>
    </Switch>
);
