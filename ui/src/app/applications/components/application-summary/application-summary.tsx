import { ErrorNotification, FormField, FormSelect, NotificationType, PopupApi } from 'argo-ui';
import * as React from 'react';
import { FormApi, Text } from 'react-form';

require('./application-summary.scss');

import { DataLoader, EditablePanel } from '../../../shared/components';
import { Consumer, ContextApis } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

import { ComparisonStatusIcon, HealthStatusIcon, syncStatusMessage } from '../utils';

export const ApplicationSummary = (props: {
    app: models.Application,
    updateApp: (app: models.Application) => Promise<any>,
}) => {
    const app = JSON.parse(JSON.stringify(props.app)) as models.Application;

    const attributes = [
        {
            title: 'PROJECT',
            view: app.spec.project,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.projects.list().then((items) => items.map((item) => item.metadata.name))}>
                    {(projects) => <FormField formApi={formApi} field='spec.project' component={FormSelect} componentProps={{options: projects}} />}
                </DataLoader>
            ),
        },
        {
            title: 'CLUSTER',
            view: app.spec.destination.server,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.clusters.list().then((clusters) => clusters.map((cluster) => ({
                    title: `${cluster.name || 'in-cluster'}: ${cluster.server}`,
                    value: cluster.server,
                })))}>
                    {(clusters) => (
                        <FormField formApi={formApi} field='spec.destination.server' componentProps={{options: clusters}} component={FormSelect}/>
                    )}
                </DataLoader>
            ),
        },
        {
            title: 'NAMESPACE',
            view: app.spec.destination.namespace,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.destination.namespace' component={Text}/>,
        },
        {
            title: 'REPO URL',
            view: (
                <a href={app.spec.source.repoURL} target='_blank' onClick={(event) => event.stopPropagation()}>
                    <i className='fa fa-external-link'/> {app.spec.source.repoURL}
                </a>
            ),
        },
        {
            title: 'TARGET REVISION',
            view: app.spec.source.targetRevision || 'HEAD',
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.targetRevision' component={Text}/>,
        },
        {
            title: 'PATH',
            view: app.spec.source.path,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.path' component={Text}/>,
        },
        {title: 'STATUS', view: (
            <span><ComparisonStatusIcon status={app.status.sync.status}/> {app.status.sync.status} {syncStatusMessage(app)}
            </span>
        )},
        {title: 'HEALTH', view: (
            <span><HealthStatusIcon state={app.status.health}/> {app.status.health.status}</span>
        )},
    ];

    const urls = app.status.summary.externalURLs || [];
    if (urls.length > 0) {
        attributes.push({title: 'URLs', view: (
            <React.Fragment>
                {urls.map((item) => <a key={item} href={item} target='__blank'>{item} &nbsp;</a>)}
            </React.Fragment>
        )});
    }

    if ((app.status.summary.images || []).length) {
        attributes.push({title: 'IMAGES', view: (
            <div className='application-summary__labels'>
                {(app.status.summary.images || []).sort().map((image) => (<span className='application-summary__label' key={image}>{image}</span>))}
            </div>
        ) });
    }

    async function setAutoSync(ctx: { popup: PopupApi }, confirmationTitle: string, confirmationText: string, prune: boolean) {
        const confirmed = await ctx.popup.confirm(confirmationTitle, confirmationText);
        if (confirmed) {
            const updatedApp = JSON.parse(JSON.stringify(props.app)) as models.Application;
            updatedApp.spec.syncPolicy = { automated: { prune } };
            props.updateApp(updatedApp);
        }
    }

    async function unsetAutoSync(ctx: { popup: PopupApi }) {
        const confirmed = await ctx.popup.confirm('Disable Auto-Sync?', 'Are you sure you want to disable automated application synchronization');
        if (confirmed) {
            const updatedApp = JSON.parse(JSON.stringify(props.app)) as models.Application;
            updatedApp.spec.syncPolicy = { automated: null };
            props.updateApp(updatedApp);
        }
    }

    class EditableInfoList extends React.Component<{}, { editing: boolean, saving: boolean }> {
        get isMounted() {
            return this.mounted;
        }

        set isMounted(mounted: boolean) {
            this.mounted = mounted;
        }

        get updatedApp() {
            return this.newApp;
        }

        set updatedApp(newApp: models.Application) {
            this.newApp = newApp;
        }

        private mounted = false;

        private newApp: models.Application;

        private urlPattern = new RegExp('^(https?:\\/\\/)?((([a-z\\d]([a-z\\d-]*[a-z\\d])*)\\.)+[a-z]{2,}|((\\d{1,3}\\.){3}\\d{1,3}))'
            + '(\\:\\d+)?(\\/[-a-z\\d%_.~+]*)*(\\?[;&a-z\\d%_.~+=-]*)?(\\#[-a-z\\d_]*)?$', 'i');

        constructor(listProps: {}) {
            super(listProps);
            this.state = { editing: false, saving: false };
        }

        public componentDidMount() {
            this.isMounted = true;
        }

        public render() {
            return (
                <Consumer>{(ctx) => (
                    <div className='white-box'>
                        <div className='white-box__details'>
                            <div className='editable-panel__buttons'>
                                <button onClick={() => { this.save(ctx); }} className='argo-button argo-button--base' disabled={this.state.saving}>
                                    {(this.state.editing || this.state.saving) ? 'Save' : 'Edit'}</button>&nbsp;
                                {(this.state.editing || this.state.saving) && <button onClick={() => {
                                    this.setState({ editing: false });
                                }} className='argo-button argo-button--base-o' disabled={this.state.saving}>Cancel</button>}
                            </div>
                            <p>Info</p>
                            <div className='argo-table-list'>
                                {this.state.editing && <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-3'>Name</div>
                                        <div className='columns small-9'>Value</div>
                                    </div>
                                </div>}
                                <div className='argo-table-list__row'>
                                    {((this.state.editing || this.state.saving) ? this.updatedApp : app).spec.info &&
                                        ((this.state.editing || this.state.saving) ? this.updatedApp : app).spec.info.map((info, i) =>
                                        <div key={i} className='row' style={{fontSize: '0.8125rem'}}>
                                            {!this.state.editing ? <React.Fragment>
                                                <div className='columns small-3'>
                                                    {info.name}
                                                </div>
                                                <div className='columns small-9'>
                                                    {!!info.value.match(this.urlPattern) ? <a target='_blank' href={info.value}>{info.value}</a> : info.value}
                                                </div>
                                            </React.Fragment> : <React.Fragment>
                                                <div className='columns small-3'>
                                                    <input className='argo-field' type='text' value={this.updatedApp.spec.info[i].name} onChange={(event) => {
                                                        this.updatedApp.spec.info[i].name = event.target.value;
                                                        this.setState({});
                                                    }}/>
                                                </div>
                                                <div className='columns small-8'>
                                                    <input className='argo-field' type='text' value={this.updatedApp.spec.info[i].value} onChange={(event) => {
                                                        this.updatedApp.spec.info[i].value = event.target.value;
                                                        this.setState({});
                                                    }}/>
                                                </div>
                                                <div className='columns small-1'>
                                                    <i className='fa fa-times' onClick={() => {
                                                        this.updatedApp.spec.info.splice(i, 1);
                                                        this.setState({});
                                                    }} style={{cursor: 'pointer'}}/>
                                                </div>
                                            </React.Fragment>}
                                        </div>)}
                                    {this.state.editing && <div className='row'>
                                        <div className='columns small-4'>
                                            <a onClick={() => {
                                                const newInfo = {name: '', value: ''};
                                                this.updatedApp.spec.info = ((this.updatedApp.spec.info || []).concat(newInfo));
                                                this.setState({});
                                            }}>Add info</a>
                                        </div>
                                    </div>}
                                </div>
                            </div>
                        </div>
                    </div>
                )}</Consumer>
            );
        }

        public componentWillUnmount() {
            this.isMounted = false;
        }

        private async save(ctx: ContextApis) {
            const editing = !this.state.editing;
            this.setState({ editing });

            if (editing) {
                this.updatedApp = JSON.parse(JSON.stringify(app)) as models.Application;
                return;
            }

            this.updatedApp.spec.info = this.updatedApp.spec.info.filter((info: models.Info) => {
                return info.name != '' && info.value != '';
            });

            try {
                this.setState({ saving: true });
                await props.updateApp(this.updatedApp);
                if (this.isMounted) {
                    this.setState({ saving: false });
                }
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to save changes' e={e}/>,
                    type: NotificationType.Error,
                });
            } finally {
                if (this.isMounted) {
                    this.setState({ saving: false });
                }
            }
        }
    }

    return (
        <React.Fragment>
            <EditablePanel
            save={props.updateApp}
            validate={(input) => ({
                'spec.project': !input.spec.project && 'Project name is required',
                'spec.destination.server': !input.spec.destination.server && 'Cluster URL is required',
                'spec.destination.namespace': !input.spec.destination.namespace && 'Namespace is required',
            })} values={app} title={app.metadata.name.toLocaleUpperCase()} items={attributes} />
            <Consumer>{(ctx) => (
            <div className='white-box'>
                <div className='white-box__details'>
                    <p>Sync Policy</p>
                    <div className='row white-box__details-row'>
                        <div className='columns small-3'>
                            {app.spec.syncPolicy && app.spec.syncPolicy.automated && <span>Automated</span> || <span>None</span>}
                        </div>
                        <div className='columns small-9'>
                            {app.spec.syncPolicy && app.spec.syncPolicy.automated && (
                                <button className='argo-button argo-button--base' onClick={() => unsetAutoSync(ctx)}>Disable Auto-Sync</button>
                            ) || (
                                <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                    ctx, 'Enable Auto-Sync?', 'Are you sure you want to enable automated application synchronization?', false)
                                }>Enable Auto-Sync</button>
                            )}
                        </div>
                    </div>

                    {app.spec.syncPolicy && app.spec.syncPolicy.automated && (
                        <div className='row white-box__details-row'>
                            <div className='columns small-3'>
                                Prune Resources
                            </div>
                            <div className='columns small-9'>
                                {app.spec.syncPolicy.automated.prune && (
                                    <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                        ctx, 'Disable Prune Resources?', 'Are you sure you want to disable resource pruning during automated application synchronization?', false)
                                    }>Disable</button>
                                ) || (
                                    <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                        ctx, 'Enable Prune Resources?', 'Are you sure you want to enable resource pruning during automated application synchronization?', true)
                                    }>Enable</button>
                                )}
                            </div>
                        </div>
                    )}

                </div>
            </div>
            )}</Consumer>

            <EditableInfoList />
        </React.Fragment>
    );
};
