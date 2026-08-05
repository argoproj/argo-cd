import * as React from 'react';
import {Redirect, Route, RouteComponentProps, Switch} from 'react-router';
import {KeybindingProvider} from 'argo-ui/v2';

import {Spinner} from '../../shared/components';

const AccountDetails = React.lazy(() => import('./account-details/account-details').then(m => ({default: m.AccountDetails})));
const AccountsList = React.lazy(() => import('./accounts-list/accounts-list').then(m => ({default: m.AccountsList})));
const AdvancedSettings = React.lazy(() => import('./advanced-settings/advanced-settings').then(m => ({default: m.AdvancedSettings})));
const CertsList = React.lazy(() => import('./certs-list/certs-list').then(m => ({default: m.CertsList})));
const ClusterDetails = React.lazy(() => import('./cluster-details/cluster-details').then(m => ({default: m.ClusterDetails})));
const ClustersList = React.lazy(() => import('./clusters-list/clusters-list').then(m => ({default: m.ClustersList})));
const GpgKeysList = React.lazy(() => import('./gpgkeys-list/gpgkeys-list').then(m => ({default: m.GpgKeysList})));
const ProjectDetails = React.lazy(() => import('./project-details/project-details').then(m => ({default: m.ProjectDetails})));
const ProjectsList = React.lazy(() => import('./projects-list/projects-list').then(m => ({default: m.ProjectsList})));
const ReposList = React.lazy(() => import('./repos-list/repos-list').then(m => ({default: m.ReposList})));
const SettingsOverview = React.lazy(() => import('./settings-overview/settings-overview').then(m => ({default: m.SettingsOverview})));
const AppearanceList = React.lazy(() => import('./appearance-list/appearance-list').then(m => ({default: m.AppearanceList})));

export const SettingsContainer = (props: RouteComponentProps<any>) => (
    <KeybindingProvider>
        <React.Suspense fallback={<Spinner show={true} />}>
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
                <Route exact={true} path={`${props.match.path}/advanced`} component={AdvancedSettings} />
                <Redirect path='*' to={`${props.match.path}`} />
            </Switch>
        </React.Suspense>
    </KeybindingProvider>
);
