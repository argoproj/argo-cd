import { AppState, models, Page } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import { connect } from 'react-redux';
import { RouteComponentProps } from 'react-router';

import * as appModels from '../../../shared/models';
import * as actions from '../../actions';
import { State } from '../../state';
import { ApplicationComponentDiff } from '../application-component-diff/application-component-diff';

require('./application-details.scss');

export interface ApplicationDetailsProps extends RouteComponentProps<{ name: string; namespace: string; }> {
    application: appModels.Application;
    onLoad: (namespace: string, name: string) => any;
}

class Component extends React.Component<ApplicationDetailsProps, { expandedRows: number[] }> {

    constructor(props: ApplicationDetailsProps) {
        super(props);
        this.state = { expandedRows: [] };
    }

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

    private renderSummary(app: appModels.Application) {
        const attributes = [
            {title: 'NAMESPACE', value: app.metadata.namespace},
            {title: 'REPO URL', value: (
                <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                    <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                </a> )},
            {title: 'PATH', value: app.spec.source.path},
            {title: 'ENVIRONMENT', value: app.spec.source.environment},
        ];
        const deltas = (app.status.comparisonResult.deltaDiffs || []).map((item) => JSON.parse(item));
        const targetComponents = (app.status.comparisonResult.targetState || []).map((item) => JSON.parse(item) as models.TypeMeta & { metadata: models.ObjectMeta });
        return (
            <div>
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
                <div>
                    <h6>Components:</h6>
                    <div className='argo-table-list argo-table-list--clickable'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-3'>Kind</div>
                            <div className='columns small-2'>API Version</div>
                            <div className='columns small-4'>Name</div>
                            <div className='columns small-3'>Status</div>
                        </div>
                    </div>
                        {targetComponents.map((component, i) => (
                            <div className='argo-table-list__row' key={i} onClick={() => this.toggleRow(i)}>
                                <div className='row'>
                                    <div className='columns small-3'>{component.kind}</div>
                                    <div className='columns small-2'>{component.apiVersion}</div>
                                    <div className='columns small-4'>{component.metadata.name}</div>
                                    <div className='columns small-3'>
                                        {deltas[i] ? <span>Different</span> : <span> Equal </span>}
                                        <i className={classNames('fa application-details__toggle-manifest', {
                                            'fa-angle-down': this.state.expandedRows.indexOf(i) === -1,
                                            'fa-angle-up': this.state.expandedRows.indexOf(i) !== -1})}/>
                                    </div>
                                </div>
                                <div className={classNames('application-details__manifest', {'application-details__manifest--expanded': this.state.expandedRows.includes(i)})}>
                                    <ApplicationComponentDiff key={i} delta={deltas[i]} component={component}/>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        );
    }

    private toggleRow(row: number) {
        const rows = this.state.expandedRows.slice();
        const index = rows.indexOf(row);
        if (index > -1) {
            rows.splice(index, 1);
        } else {
            rows.push(row);
        }
        this.setState({ expandedRows: rows });
    }
}

export const ApplicationDetails = connect((state: AppState<State>) => ({
    application: state.page.application,
}), (dispatch) => ({
    onLoad: (namespace: string, name: string) => dispatch(actions.loadApplication(name)),
}))(Component);
