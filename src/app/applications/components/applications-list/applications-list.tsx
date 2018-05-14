import { AppContext, AppState, MockupList, SlidingPanel } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';
import { Field, InjectedFormProps, reduxForm, submit } from 'redux-form';
import { Subscription } from 'rxjs';
import { FormField, Page } from '../../../shared/components';
import * as models from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';
import { ComparisonStatusIcon, HealthStatusIcon } from '../utils';

require('./applications-list.scss');

export interface ApplicationProps extends RouteComponentProps<{}> {
    onLoad: () => any;
    createApp: () => any;
    applications: models.Application[];
    changesSubscription: Subscription;
    showNewAppPanel: boolean;
}

interface NewAppParams {
    applicationName: string;
    path: string;
    repoURL: string;
    environment: string;
    clusterURL: string;
    namespace: string;
}

const required = (value: any) => (value ? undefined : 'Required');

const NewAppForm = reduxForm<NewAppParams>({
    form: 'newAppForm',
    onSubmit: (params, dispatch) => {
        dispatch(actions.createApplication(params.applicationName, {
            environment: params.environment,
            path: params.path,
            repoURL: params.repoURL,
            targetRevision: null,
            componentParameterOverrides: null,
        }, {
            server: params.clusterURL,
            namespace: params.namespace,
        }));
    },
})((props: InjectedFormProps<NewAppParams, {}>) => (
    <form role='form' className='width-control'>
        <div className='argo-form-row'>
            <Field validate={required} name='applicationName' component={FormField} type='text' label='Application Name'/>
        </div>
        <div className='argo-form-row'>
            <Field validate={required} name='repoURL' component={FormField} type='text' label='Repository URL'/>
        </div>
        <div className='argo-form-row'>
            <Field validate={required} name='path' component={FormField} type='text' label='Path'/>
        </div>
        <div className='argo-form-row'>
            <Field validate={required} name='environment' component={FormField} type='text' label='Environment'/>
        </div>
        <div className='argo-form-row'>
            <Field name='clusterURL' component={FormField} type='text' label='Cluster URL'/>
        </div>
        <div className='argo-form-row'>
            <Field name='namespace' component={FormField} type='text' label='Namespace'/>
        </div>
    </form>
));

class Component extends React.Component<ApplicationProps> {

    public static contextTypes = {
        router: PropTypes.object,
    };

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
        <Page title='Applications' toolbar={{
                breadcrumbs: [{ title: 'Applications', path: '/applications' }],
                actionMenu: {
                    className: 'fa fa-plus',
                    items: [{
                        title: 'New Application',
                        action: () => this.setNewAppPanelVisible(true),
                    }],
                },
            }} >
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
                                            <div className='row applications-list__icons'>
                                                <div className='columns small-6'>STATUS:</div>
                                                <div className='columns small-6'>
                                                    <ComparisonStatusIcon status={app.status.comparisonResult.status} /> {app.status.comparisonResult.status}
                                                </div>
                                            </div>
                                            <div className='row applications-list__icons'>
                                                <div className='columns small-6'>HEALTH:</div>
                                                <div className='columns small-6'>
                                                    <HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                                                </div>
                                            </div>
                                        </div>
                                        <div className='columns small-9 applications-list__info'>
                                            <div className='row'>
                                                <div className='columns small-3'>CLUSTER:</div>
                                                <div className='columns small-9'>{app.spec.destination.server}</div>
                                            </div>
                                            <div className='row'>
                                                <div className='columns small-3'>NAMESPACE:</div>
                                                <div className='columns small-9'>{app.spec.destination.namespace}</div>
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
                <SlidingPanel isShown={this.props.showNewAppPanel} onClose={() => this.setNewAppPanelVisible(false)} isMiddle={true} header={(
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.props.createApp()}>
                                Create
                            </button> <button onClick={() => this.setNewAppPanelVisible(false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    )}>
                    <NewAppForm/>
                </SlidingPanel>
            </Page>
        );
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private setNewAppPanelVisible(isVisible: boolean) {
        this.appContext.router.history.push(`${this.props.match.url}?new=${isVisible}`);
    }
}

export const ApplicationsList = connect((state: AppState<State>) => {
    return {
        applications: state.page.applications,
        changesSubscription: state.page.changesSubscription,
        showNewAppPanel: new URLSearchParams(state.router.location.search).get('new') === 'true',
    };
}, (dispatch) => ({
    onLoad: () => dispatch(actions.loadAppsList()),
    createApp: () => dispatch(submit('newAppForm')),
}))(Component);
