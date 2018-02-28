import { AppContext, AppState, MockupList, Page } from 'argo-ui';
import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { Subscription } from 'rxjs';

import * as models from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';

require('./applications-list.scss');

export interface ApplicationProps {
    onLoad: () => any;
    applications: models.Application[];
    changesSubscription: Subscription;
}

class Component extends React.Component<ApplicationProps> {

    public componentDidMount() {
        this.props.onLoad();
    }

    public componentWillUnmount() {
        if (this.props.changesSubscription) {
            this.props.changesSubscription.unsubscribe();
        }
    }

    public render() {
        return (
            <Page title='Applications' toolbar={{breadcrumbs: [{ title: 'Applications', path: '/applications' }]}}>
                <div className='argo-container applications-list'>
                    {this.props.applications ? (
                        <div className='argo-table-list argo-table-list--clickable'>
                            {this.props.applications.map((app) => (
                                <div key={app.metadata.name} className='argo-table-list__row'>
                                    <div className='row' onClick={() => this.appContext.router.history.push(`/applications/${app.metadata.namespace}/${app.metadata.name}`)}>
                                        <div className='columns small-3'>
                                            <div className='row'>
                                                <div className='columns small-12'>
                                                    <i className='argo-icon-application icon'/> <span className='applications-list__title'>{app.metadata.name}</span>
                                                </div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-6'>STATUS:</div>
                                                <div className='columns small-6'>
                                                    <i className={classNames('fa', {
                                                        'fa-check-circle': app.status.comparisonResult.status === models.ComparisonStatuses.Equal,
                                                        'fa-times': app.status.comparisonResult.status === models.ComparisonStatuses.Different,
                                                        'fa-exclamation-circle': app.status.comparisonResult.status === models.ComparisonStatuses.Error,
                                                        'fa-circle-o-notch status-icon--running status-icon--spin':
                                                            app.status.comparisonResult.status === models.ComparisonStatuses.Unknown,
                                                    })}/> {app.status.comparisonResult.status}
                                                </div>
                                            </div>
                                        </div>
                                        <div className='columns small-9 applications-list__info'>
                                            <div className='row'>
                                                <div className='columns small-3'>NAMESPACE:</div>
                                                <div className='columns small-9'>{app.metadata.namespace}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>REPO URL:</div>
                                                <div className='columns small-9'>
                                                    <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                                                        <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                                                    </a>
                                                </div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>PATH:</div>
                                                <div className='columns small-9'>{app.spec.source.path}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>ENVIRONMENT:</div>
                                                <div className='columns small-9'>{app.spec.source.environment}</div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : <MockupList height={50} marginTop={30}/>}
                </div>
            </Page>
        );
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}

(Component as React.ComponentClass).contextTypes = {
    router: PropTypes.object,
};

export const ApplicationsList = connect((state: AppState<State>) => {
    return {
        applications: state.page.applications,
        changesSubscription: state.page.changesSubscription,
    };
}, (dispatch) => ({
    onLoad: () => dispatch(actions.loadAppsList()),
}))(Component);
