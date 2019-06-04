import { DropDownMenu, FormField, NotificationType, SlidingPanel } from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';
import { RouteComponentProps } from 'react-router';

import { ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, Page } from '../../../shared/components';
import { AppContext } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

require('./repos-list.scss');

interface NewRepoParams {
    url: string;
    username: string;
    password: string;
}

export class ReposList extends React.Component<RouteComponentProps<any>> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object,
    };

    private formApi: FormApi;
    private loader: DataLoader;

    public render() {
        return (
            <Page title='Repositories' toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Repositories'}],
                actionMenu: {
                    className: 'fa fa-plus',
                    items: [{
                        title: 'Connect Repo',
                        action: () => this.showConnectRepo = true,
                    }],
                },
            }}>
                <div className='repos-list'>
                    <div className='argo-container'>
                    <DataLoader load={() => services.repos.list()} ref={(loader) => this.loader = loader}>
                    {(repos: models.Repository[]) => (
                        repos.length > 0 && (
                        <div className='argo-table-list'>
                            <div className='argo-table-list__head'>
                                <div className='row'>
                                    <div className='columns small-9'>REPOSITORY</div>
                                    <div className='columns small-3'>CONNECTION STATUS</div>
                                </div>
                            </div>
                            {repos.map((repo) => (
                                <div className='argo-table-list__row' key={repo.repo}>
                                    <div className='row'>
                                        <div className='columns small-9'>
                                        <i className='icon argo-icon-git'/> {repo.repo}
                                        </div>
                                        <div className='columns small-3'>
                                            <ConnectionStateIcon state={repo.connectionState}/> {repo.connectionState.status}
                                            <DropDownMenu anchor={() => <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                <i className='fa fa-ellipsis-v'/>
                                            </button>} items={[{
                                                title: 'Create application',
                                                action: () => this.appContext.apis.navigation.goto('/applications', {
                                                    new: JSON.stringify({spec: { source: { repoURL: repo.repo } } }),
                                                }),
                                            }, {
                                                title: 'Disconnect',
                                                action: () => this.disconnectRepo(repo.repo),
                                            }]}/>
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div> ) || (
                            <EmptyState icon='argo-icon-git'>
                                <h4>No repositories connected</h4>
                                <h5>Connect your repo to deploy apps.</h5>
                                <button className='argo-button argo-button--base' onClick={() => this.showConnectRepo = true}>Connect Repo</button>
                            </EmptyState>
                        )
                    )}
                    </DataLoader>
                    </div>
                </div>
                <SlidingPanel isShown={this.showConnectRepo} onClose={() => this.showConnectRepo = false} header={(
                    <div>
                    <button className='argo-button argo-button--base' onClick={() => this.formApi.submitForm(null)}>
                        Connect
                    </button> <button onClick={() => this.showConnectRepo = false} className='argo-button argo-button--base-o'>
                        Cancel
                    </button>
                    </div>
                )}>
                    <h4>Connect Git repo</h4>
                    <Form onSubmit={(params) => this.connectRepo(params as NewRepoParams)}
                        getApi={(api) => this.formApi = api}
                        validateError={(params: NewRepoParams) => ({
                            url: !params.url && 'Repo URL is required',
                        })}>
                        {(formApi) => (
                            <form onSubmit={formApi.submitForm} role='form' className='width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Repository URL' field='url' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Username' field='username' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Password' field='password' component={Text} componentProps={{type: 'password'}}/>
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
            </Page>
        );
    }

    private async connectRepo(params: NewRepoParams) {
        try {
            await services.repos.create(params);
            this.showConnectRepo = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to connect repository' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private async disconnectRepo(repo: string) {
        const confirmed = await this.appContext.apis.popup.confirm(
            'Disconnect repository', `Are you sure you want to disconnect '${repo}'?`);
        if (confirmed) {
            await services.repos.delete(repo);
            this.loader.reload();
        }
    }

    private get showConnectRepo() {
        return new URLSearchParams(this.props.location.search).get('addRepo') === 'true';
    }

    private set showConnectRepo(val: boolean) {
        this.appContext.router.history.push(`${this.props.match.url}?addRepo=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
