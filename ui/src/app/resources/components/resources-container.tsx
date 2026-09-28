import * as React from 'react';
import {Route, RouteComponentProps, Switch} from 'react-router';

import {EmptyState, Page} from '../../shared/components';
import {AuthSettingsCtx} from '../../shared/context';

import {ResourcesList} from './resources-list/resources-list';

const RESOURCES_VIEW_DOCS_URL = 'https://argo-cd.readthedocs.io/en/stable/user-guide/resources-view/';

const ResourcesDisabled = () => (
    <Page title='Resources' useTitleOnly={true} toolbar={{breadcrumbs: [{title: 'Resources', path: '/resources'}]}}>
        <div className='row'>
            <div className='columns small-12'>
                <EmptyState icon='argo-icon-catalog'>
                    <h4>The Resources view is not enabled on this instance</h4>
                    <h5>Contact your administrator to enable the view.</h5>
                    <a href={RESOURCES_VIEW_DOCS_URL} target='_blank' rel='noopener noreferrer'>
                        Refer to the documentation
                    </a>
                </EmptyState>
            </div>
        </div>
    </Page>
);

const ResourcesEntry = (props: RouteComponentProps<any>) => {
    const authSettings = React.useContext(AuthSettingsCtx);
    if (authSettings && !authSettings.resourceViewEnabled) {
        return <ResourcesDisabled />;
    }
    return <ResourcesList {...props} />;
};

export const ResourceContainer = (props: RouteComponentProps<any>) => (
    <Switch>
        <Route exact={true} path={`${props.match.path}`} component={ResourcesEntry} />
    </Switch>
);
