import * as classNames from 'classnames';
import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';
import { Observable, Subscription } from 'rxjs';

import { ConnectionStateIcon, FormField } from '../../../shared/components';
import * as models from '../../../shared/models';

export const AppsList = (props: {apps: models.KsonnetAppSpec[], selectedApp: models.KsonnetAppSpec, onAppSelected: (app: models.KsonnetAppSpec) => any}) => (
    <div className='argo-table-list argo-table-list--clickable'>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-4'>APPLICATION NAME</div>
                <div className='columns small-6'>PATH</div>
                <div className='columns small-2'>ENVIRONMENTS COUNT</div>
            </div>
        </div>
        {props.apps.map((app, i) => (
            <div className={classNames('argo-table-list__row', { selected: app === props.selectedApp })} key={i} onClick={() => props.onAppSelected(app)}>
                <div className='row'>
                    <div className='columns small-4'>{app.name}</div>
                    <div className='columns small-6'>{app.path}</div>
                    <div className='columns small-2'>{Object.keys(app.environments).length}</div>
                </div>
            </div>
        ))}
    </div>
);

export const RepositoryList = (props: {repos: models.Repository[], selectedRepo: string, onSelectedRepo: (repo: string) => any}) => (
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
    </div>
);

export const EnvironmentsList = (props: {
    envs: { [key: string]: models.KsonnetEnvironment; },
    selectedEnv: string;
    onEnvsSelectionChanged: (env: string) => any;
}) =>  {
    const environments = Object.keys(props.envs).map((name) => Object.assign(props.envs[name], {name}));
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
}

export class AppParams extends React.Component<{
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
                    applicationName: !params.applicationName && 'Application name is required',
                    repoURL: !params.repoURL && 'Repository URL is required',
                    path: !params.path && 'Path is required',
                    environment: !params.environment && 'Environment is required',
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
                            <FormField formApi={api} label='Application Name' field='applicationName' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Repository URL' field='repoURL' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Path' field='path' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Environment' field='environment' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Cluster URL' field='clusterURL' component={Text}/>
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
