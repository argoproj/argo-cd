import * as React from 'react';
import {Redirect, Route, RouteComponentProps, Switch} from 'react-router';

import {AccountDetails} from './account-details/account-details';
import {AccountsList} from './accounts-list/accounts-list';
import {CertsList} from './certs-list/certs-list';
import {ClusterDetails} from './cluster-details/cluster-details';
import {ClustersList} from './clusters-list/clusters-list';
import {GpgKeysList} from './gpgkeys-list/gpgkeys-list';
import {ProjectDetails} from './project-details/project-details';
import {ProjectsList} from './projects-list/projects-list';
import {ReposList} from './repos-list/repos-list';
import {SettingsOverview} from './settings-overview/settings-overview';
import {AppearanceList} from './appearance-list/appearance-list';

export const SettingsContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={SettingsOverview} />
        <Route exact={true} path={`${props.match.path}/repos`} component={ReposList} />
        <Route exact={true} path={`${props.match.path}/certs`} component={CertsList} />
        <Route exact={true} path={`${props.match.path}/gpgkeys`} component={GpgKeysList} />
        <Route exact={true} path={`${props.match.path}/clusters`} component={ClustersList} />
        <Route exact={true} path={`${props.match.path}/clusters/:server`} component={ClusterDetails} />
        <Route exact={true} path={`${props.match.path}/projects`} component={ProjectsList} />
        <Route exact={true} path={`${props.match.path}/projects/:name`} component={ProjectDetails} />
        <Route exact={true} path={`${props.match.path}/accounts`} component={AccountsList} />
        <Route exact={true} path={`${props.match.path}/accounts/:name`} component={AccountDetails} />
        <Route exact={true} path={`${props.match.path}/appearance`} component={AppearanceList} />
        <Redirect path='*' to={`${props.match.path}`} />
    </Switch>
);
