import { AppState, Page } from 'argo-ui';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';

import * as models from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';

require('./application-details.scss');

export interface ApplicationDetailsProps extends RouteComponentProps<{ name: string; namespace: string; }> {
    application: models.Application;
    onLoad: (namespace: string, name: string) => any;
}

class Component extends React.Component<ApplicationDetailsProps> {
    public componentDidMount() {
        this.props.onLoad(this.props.match.params.namespace, this.props.match.params.name);
    }

    public render() {
        return (
            <Page title={'Workflow Details'} toolbar={{breadcrumbs: [{title: 'Applications', path: '/applications' }, { title: this.props.match.params.name }] }}>
                <div className='argo-container application-details'>
                    {this.props.application ? this.renderSummary(this.props.application) : (
                        <div>Loading...</div>
                    )}
                </div>
            </Page>
        );
    }

    private renderSummary(app: models.Application) {
        const attributes = [
            {title: 'NAMESPACE', value: app.metadata.namespace},
            {title: 'REPO URL', value: (
                <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                    <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                </a> )},
            {title: 'PATH', value: app.spec.source.path},
            {title: 'ENVIRONMENT', value: app.spec.source.environment},
        ];
        return (
            <div className='white-box'>
                <div className='white-box__details'>
                    {attributes.map((attr) => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>
                                {attr.title}
                            </div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
        );
    }
}

export const ApplicationDetails = connect((state: AppState<State>) => ({
    application: state.page.application,
}), (dispatch) => ({
    onLoad: (namespace: string, name: string) => dispatch(actions.loadApplication(name)),
}))(Component);
