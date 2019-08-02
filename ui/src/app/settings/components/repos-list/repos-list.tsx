import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormApi, Text, TextArea} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {CheckboxField, ConnectionStateIcon, DataLoader, EmptyState, ErrorNotification, Page, Repo} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

require('./repos-list.scss');

interface NewSSHRepoParams {
    url: string;
    sshPrivateKey: string;
    insecure: boolean;
    enableLfs: boolean;
}

interface NewHTTPSRepoParams {
    url: string;
    username: string;
    password: string;
    tlsClientCertData: string;
    tlsClientCertKey: string;
    insecure: boolean;
    enableLfs: boolean;
}

export class ReposList extends React.Component<RouteComponentProps<any>> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object,
    };

    private formApiSSH: FormApi;
    private formApiHTTPS: FormApi;
    private loader: DataLoader;

    public render() {
        return (
            <Page title='Repositories' toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Repositories'}],
                actionMenu: {
                    className: 'fa fa-plus',
                    items: [{
                        title: 'Connect Repo using SSH',
                        action: () => this.showConnectSSHRepo = true,
                    }, {
                        title: 'Connect Repo using HTTPS',
                        action: () => this.showConnectHTTPSRepo = true,
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
                                                        <i className='icon argo-icon-git'/> <Repo url={repo.repo}/>
                                                    </div>
                                                    <div className='columns small-3'>
                                                        <ConnectionStateIcon
                                                            state={repo.connectionState}/> {repo.connectionState.status}
                                                        <DropDownMenu anchor={() => <button
                                                            className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                            <i className='fa fa-ellipsis-v'/>
                                                        </button>} items={[{
                                                            title: 'Create application',
                                                            action: () => this.appContext.apis.navigation.goto('/applications', {
                                                                new: JSON.stringify({spec: {source: {repoURL: repo.repo}}}),
                                                            }),
                                                        }, {
                                                            title: 'Disconnect',
                                                            action: () => this.disconnectRepo(repo.repo),
                                                        }]}/>
                                                    </div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>) || (
                                    <EmptyState icon='argo-icon-git'>
                                        <h4>No repositories connected</h4>
                                        <h5>Connect your repo to deploy apps.</h5>
                                        <button className='argo-button argo-button--base'
                                                onClick={() => this.showConnectSSHRepo = true}>Connect Repo using SSH
                                        </button> <button className='argo-button argo-button--base'
                                                onClick={() => this.showConnectHTTPSRepo = true}>Connect Repo using HTTPS
                                        </button>
                                    </EmptyState>
                                )
                            )}
                        </DataLoader>
                    </div>
                </div>
                <SlidingPanel isShown={this.showConnectHTTPSRepo} onClose={() => this.showConnectHTTPSRepo = false} header={(
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => this.formApiHTTPS.submitForm(null)}>
                            Connect
                        </button> <button onClick={() => this.showConnectHTTPSRepo = false}
                                className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                )}>
                    <h4>Connect Git repo using HTTPS</h4>
                    <Form onSubmit={(params) => this.connectHTTPSRepo(params as NewHTTPSRepoParams)}
                          getApi={(api) => this.formApiHTTPS = api}
                          validateError={(params: NewHTTPSRepoParams) => ({
                              url: !params.url && 'Repo URL is required',
                              password: !params.password && params.username && 'Password is required if username is given.',
                              tlsClientCertKey: !params.tlsClientCertKey && params.tlsClientCertData && 'TLS client cert key is required if TLS client cert is given.',
                          })}>
                        {(formApi) => (
                            <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Repository URL' field='url' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Username (optional)' field='username' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Password (optional)' field='password' component={Text}
                                               componentProps={{type: 'password'}}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='TLS client certificate (optional)' field='tlsClientCertData' component={TextArea}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='TLS client certificate key (optional)' field='tlsClientCertKey' component={TextArea}/>
                                </div>
                               <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Enable LFS support' field='enableLfs' component={CheckboxField}/>
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
                <SlidingPanel isShown={this.showConnectSSHRepo} onClose={() => this.showConnectSSHRepo = false} header={(
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => this.formApiSSH.submitForm(null)}>
                            Connect
                        </button> <button onClick={() => this.showConnectSSHRepo = false}
                                className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                )}>
                    <h4>Connect Git repo using SSH</h4>
                    <Form onSubmit={(params) => this.connectSSHRepo(params as NewSSHRepoParams)}
                          getApi={(api) => this.formApiSSH = api}
                          validateError={(params: NewSSHRepoParams) => ({
                              url: !params.url && 'Repo URL is required',
                              sshPrivateKey: !params.sshPrivateKey && 'SSH private key data is required for connecting SSH repositories',
                          })}>
                        {(formApi) => (
                            <form onSubmit={formApi.submitForm} role='form' className='repos-list width-control'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Repository URL' field='url' component={Text}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='SSH private key data' field='sshPrivateKey' component={TextArea}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Skip server verification' field='insecure' component={CheckboxField}/>
                                </div>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='Enable LFS support' field='enableLfs' component={CheckboxField}/>
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
            </Page>
        );
    }

    /*
    private isHTTPSUrl(url: string) {
        if (url.match(/^https:\/\/.*$/gi)) {
            return true;
        } else {
            return false;
        }
    }
    */

    private clearConnectSSHForm() {
        this.formApiSSH.resetAll();
    }

    private clearConnectHTTPSForm() {
        this.formApiHTTPS.resetAll();
    }

    private async connectSSHRepo(params: NewSSHRepoParams) {
        try {
            await services.repos.createSSH(params);
            this.showConnectSSHRepo = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to connect repository' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private async connectHTTPSRepo(params: NewHTTPSRepoParams) {
        try {
            await services.repos.createHTTPS(params);
            this.showConnectSSHRepo = false;
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

    private get showConnectHTTPSRepo() {
        return new URLSearchParams(this.props.location.search).get('addHTTPSRepo') === 'true';
    }

    private set showConnectHTTPSRepo(val: boolean) {
        this.clearConnectHTTPSForm();
        this.appContext.router.history.push(`${this.props.match.url}?addHTTPSRepo=${val}`);
    }

    private get showConnectSSHRepo() {
        return new URLSearchParams(this.props.location.search).get('addSSHRepo') === 'true';
    }

    private set showConnectSSHRepo(val: boolean) {
        this.clearConnectSSHForm();
        this.appContext.router.history.push(`${this.props.match.url}?addSSHRepo=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
