import * as classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';
import { Observable, Subscription } from 'rxjs';

import { ConnectionStateIcon, DataLoader, FormField, Select } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

export const AppsList = (props: {
    apps: models.AppInfo[],
    selectedApp: models.AppInfo,
    onAppSelected: (app: models.AppInfo) => any,
}) => (
    props.apps.length === 0 ? (
    <div>
        Repository has no applications.
    </div>
    ) : (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-2'>TYPE</div>
                <div className='columns small-10'>PATH</div>
            </div>
        </div>
        {props.apps.map((item, i) => (
            <div className={classNames('argo-table-list__row', { selected: item === props.selectedApp })} key={i} onClick={() => props.onAppSelected(item)}>
                <div className='row'>
                    <div className='columns small-2'>{item.type}</div>
                    <div className='columns small-10'>{item.path}</div>
                </div>
            </div>
        ))}
    </div>
    )
);

export const RepositoryList: React.StatelessComponent<
    {repos: models.Repository[], selectedRepo: string, onSelectedRepo: (repo: string) => any}> = (props, context: AppContext) => (

    props.repos.length > 0 && (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-9'>REPOSITORY</div>
                <div className='columns small-3'>CONNECTION STATUS</div>
            </div>
        </div>
        {props.repos.map((repo) => (
            <div onClick={() => props.onSelectedRepo(repo.repo)}
                    className={classNames('argo-table-list__row', {selected: repo.repo === props.selectedRepo})} key={repo.repo}>
                <div className='row'>
                    <div className='columns small-9'>
                        <i className='icon argo-icon-git'/> {repo.repo}
                    </div>
                    <div className='columns small-3'>
                        <ConnectionStateIcon state={repo.connectionState}/> {repo.connectionState.status}
                    </div>
                </div>
            </div>
        ))}
    </div>) || (
    <div>
        <p>No repositories connected. Connect your first repo to create an application.</p>
        <button className='argo-button argo-button--base' onClick={() => context.router.history.push('/settings/repos?addRepo=true')} >Connect Repo</button>
    </div>
    )
);

RepositoryList.contextTypes = {
    router: PropTypes.object,
};

export const EnvironmentsList = (props: {
    envs: { [key: string]: models.KsonnetEnvironment; },
    selectedEnv: string;
    onEnvsSelectionChanged: (env: string) => any;
}) =>  {
    const environments = Object.keys(props.envs).map((name) => Object.assign(props.envs[name], {name}));
    if (environments.length === 0) {
        return <p>Application has no environments</p>;
    }
    return (
        <div className='argo-table-list argo-table-list--clickable'>
            <div className='argo-table-list__head'>
                <div className='row'>
                    <div className='columns small-3'>NAME</div>
                    <div className='columns small-3'>NAMESPACE</div>
                    <div className='columns small-3'>KUBERNETES CLUSTER</div>
                    <div className='columns small-3'>KUBERNETES VERSION</div>
                </div>
            </div>
            {environments.map((env) => (
                <div className={classNames('argo-table-list__row', {selected: props.selectedEnv === env.name })}
                        key={env.name} onClick={() => props.onEnvsSelectionChanged(env.name)}>
                    <div className='row'>
                        <div className='columns small-3'>
                            {env.name}
                        </div>
                        <div className='columns small-3'>
                            {env.destination.namespace}
                        </div>
                        <div className='columns small-3'>
                            {env.destination.server}
                        </div>
                        <div className='columns small-3'>
                            {env.k8sVersion}
                        </div>
                    </div>
                </div>
            ))}
        </div>
    );
};

export interface NewAppParams {
    applicationName: string;
    path: string;
    repoURL: string;
    environment: string;
    clusterURL: string;
    namespace: string;
    project: string;
    valuesFiles?: string[];
    revision: string;
}

export class AppParams extends React.Component<{
    needEnvironment: boolean,
    needValuesFiles: boolean,
    valuesFiles: string[],
    environments: string[],
    projects: string[],
    appParams: NewAppParams,
    submitForm: Observable<any>,
    onValidationChanged: (isValid: boolean) => any,
    onSubmit: (params: NewAppParams) => any,
}> {

    private formApi: FormApi;
    private subscription: Subscription;

    public componentDidMount() {
        this.subscription = this.props.submitForm.filter((item) => item !== null).subscribe(() => this.formApi && this.formApi.submitForm(null));
    }

    public componentWillUnmount() {
        if (this.subscription != null) {
            this.subscription.unsubscribe();
            this.subscription = null;
        }
    }

    public render() {
        return (
            <Form
                validateError={(params: NewAppParams) => ({
                    project: !params.project && 'Project is required',
                    applicationName: !params.applicationName && 'Application name is required',
                    repoURL: !params.repoURL && 'Repository URL is required',
                    revision: !params.revision && 'Revision is required',
                    path: !params.path && 'Path is required',
                    environment: this.props.needEnvironment && !params.environment && 'Environment is required',
                    clusterURL: !params.clusterURL && 'Cluster URL is required',
                    namespace: !params.namespace && 'Namespace URL is required',
                })}
                defaultValues={this.props.appParams}
                getApi={(api) => this.formApi = api}
                formDidUpdate={(state: any) => this.props.onValidationChanged(state.validationFailures === 0)}
                onSubmit={this.props.onSubmit}>

                {(api) => (
                    <form onSubmit={api.submitForm} role='form' className='width-control'>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Repository URL' field='repoURL' componentProps={{readOnly: true}} component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Revision' field='revision' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Path' field='path' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Project' field='project' component={Select} componentProps={{options: this.props.projects}} />
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Application Name' field='applicationName' component={Text}/>
                        </div>
                        {this.props.needEnvironment && (
                            <div className='argo-form-row'>
                                <FormField formApi={api} label='Environment' field='environment' component={Select}  componentProps={{options: this.props.environments}} />
                            </div>
                        )}
                        {this.props.needValuesFiles && (
                            <div className='argo-form-row'>
                                <FormField formApi={api} label='Values Files' field='valuesFiles' component={Select} componentProps={{
                                    options: this.props.valuesFiles,
                                    multiSelect: true,
                                }} />
                            </div>
                        )}
                        <div className='argo-form-row'>
                            <DataLoader load={() => services.clustersService.list().then((clusters) => clusters.map((item) => item.server))}>
                                {(clusters) => (
                                    <FormField  formApi={api} label='Cluster URL' field='clusterURL' componentProps={{options: clusters}} component={Select}/>
                                )}
                            </DataLoader>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Namespace' field='namespace' component={Text}/>
                        </div>
                    </form>
                )}
            </Form>
        );
    }
}
