import { DropDownMenu, FormField, FormSelect, PopupApi } from 'argo-ui';
import * as React from 'react';
import { FormApi, Text } from 'react-form';

require('./application-summary.scss');

import { Cluster, clusterTitle, DataLoader, EditablePanel, EditablePanelItem } from '../../../shared/components';
import { Repo, Revision } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

import { ComparisonStatusIcon, HealthStatusIcon, syncStatusMessage } from '../utils';

const urlPattern = new RegExp('^(https?:\\/\\/)?((([a-z\\d]([a-z\\d-]*[a-z\\d])*)\\.)+[a-z]{2,}|((\\d{1,3}\\.){3}\\d{1,3}))'
    + '(\\:\\d+)?(\\/[-a-z\\d%_.~+]*)*(\\?[;&a-z\\d%_.~+=-]*)?(\\#[-a-z\\d_]*)?$', 'i');

function swap(array: any[], a: number, b: number) {
    array = array.slice();
    [array[a], array[b]] = [array[b], array[a]];
    return array;
}

export const ApplicationSummary = (props: {
    app: models.Application,
    updateApp: (app: models.Application) => Promise<any>,
}) => {
    const app = JSON.parse(JSON.stringify(props.app)) as models.Application;

    const attributes = [
        {
            title: 'PROJECT',
            view: (
                <a href={'/settings/projects/' + app.spec.project}>{app.spec.project}</a>
            ),
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.projects.list().then((projs) => projs.map((item) => item.metadata.name))}>
                    {(projects) => <FormField formApi={formApi} field='spec.project' component={FormSelect} componentProps={{options: projects}} />}
                </DataLoader>
            ),
        },
        {
            title: 'CLUSTER',
            view: <Cluster url={app.spec.destination.server} showUrl={true}/> ,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.clusters.list().then((clusters) => clusters.map((cluster) => ({
                    title: clusterTitle(cluster),
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
            view: <Repo url={app.spec.source.repoURL}/>,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.repoURL' component={Text}/>,
        },
        {
            title: 'TARGET REVISION',
            view: <Revision repoUrl={app.spec.source.repoURL} revision={app.spec.source.targetRevision}/>,
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

    async function setAutoSync(ctx: { popup: PopupApi }, confirmationTitle: string, confirmationText: string, prune: boolean, selfHeal: boolean) {
        const confirmed = await ctx.popup.confirm(confirmationTitle, confirmationText);
        if (confirmed) {
            const updatedApp = JSON.parse(JSON.stringify(props.app)) as models.Application;
            updatedApp.spec.syncPolicy = { automated: { prune, selfHeal } };
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

    const items = (app.spec.info || []);
    const [adjustedCount, setAdjustedCount] = React.useState(0);

    const added = new Array<{name: string, value: string, key: string}>();
    for (let i = 0; i < adjustedCount; i++) {
        added.push({ name: '', value: '', key: (items.length + i).toString()});
    }
    for (let i = 0; i > adjustedCount; i--) {
        items.pop();
    }
    const allItems = items.concat(added);
    const infoItems: EditablePanelItem[] = allItems.map((info, i) => ({
        key: i.toString(),
        title: info.name,
        view: info.value.match(urlPattern) ? <a href={info.value} target='__blank'>{info.value}</a> : info.value,
        titleEdit: (formApi: FormApi) => (
            <React.Fragment>
                {i > 0 && <i className='fa fa-sort-up application-summary__sort-icon' onClick={() => {
                    formApi.setValue('spec.info', swap((formApi.getFormState().values.spec.info || []), i, i - 1));
                }}/>}
                <FormField formApi={formApi} field={`spec.info[${[i]}].name`} component={Text} componentProps={{style: { width: '99%' }}} />
                {i < allItems.length - 1 && <i className='fa fa-sort-down application-summary__sort-icon' onClick={() => {
                    formApi.setValue('spec.info', swap((formApi.getFormState().values.spec.info || []), i, i + 1));
                }}/> }
            </React.Fragment>
        ),
        edit: (formApi: FormApi) => (
            <React.Fragment>
                <FormField formApi={formApi} field={`spec.info[${[i]}].value`} component={Text}/>
                <i className='fa fa-times application-summary__remove-icon' onClick={() => {
                    const values = (formApi.getFormState().values.spec.info || []) as Array<any>;
                    formApi.setValue('spec.info', [...values.slice(0, i), ...values.slice(i + 1, values.length)]);
                    setAdjustedCount(adjustedCount - 1);
                }}/>
            </React.Fragment>
        ),
    })).concat({
        key: '-1',
        title: '',
        titleEdit: () => (
            <button className='argo-button argo-button--base' onClick={() => {
                setAdjustedCount(adjustedCount + 1);
            }}>ADD NEW ITEM</button>
        ),
        view: null as any,
        edit: null,
    });
    const [badgeType, setBadgeType] = React.useState('URL');
    const badgeURL = `${location.protocol}//${location.host}/api/badge?name=${props.app.metadata.name}`;
    const appURL = `${location.protocol}//${location.host}/applications/${props.app.metadata.name}`;

    return (
        <React.Fragment>
            <EditablePanel
            save={props.updateApp}
            validate={(input) => ({
                'spec.project': !input.spec.project && 'Project name is required',
                'spec.destination.server': !input.spec.destination.server && 'Cluster is required',
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
                                    ctx, 'Enable Auto-Sync?', 'Are you sure you want to enable automated application synchronization?', false, false)
                                }>Enable Auto-Sync</button>
                            )}
                        </div>
                    </div>

                    {app.spec.syncPolicy && app.spec.syncPolicy.automated && (
                        <React.Fragment>
                            <div className='row white-box__details-row'>
                                <div className='columns small-3'>
                                    Prune Resources
                                </div>
                                <div className='columns small-9'>
                                    {app.spec.syncPolicy.automated.prune && (
                                        <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                            ctx, 'Disable Prune Resources?', 'Are you sure you want to disable resource pruning during automated application synchronization?',
                                            false, app.spec.syncPolicy.automated.selfHeal)
                                        }>Disable</button>
                                    ) || (
                                        <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                            ctx, 'Enable Prune Resources?', 'Are you sure you want to enable resource pruning during automated application synchronization?',
                                            true, app.spec.syncPolicy.automated.selfHeal)
                                        }>Enable</button>
                                    )}
                                </div>
                            </div>
                            <div className='row white-box__details-row'>
                                <div className='columns small-3'>
                                    Self Heal
                                </div>
                                <div className='columns small-9'>
                                    {app.spec.syncPolicy.automated.selfHeal && (
                                        <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                            ctx, 'Disable Self Heal?', 'Are you sure you want to disable automated self healing?',
                                            app.spec.syncPolicy.automated.prune, false)
                                        }>Disable</button>
                                    ) || (
                                        <button className='argo-button argo-button--base' onClick={() => setAutoSync(
                                            ctx, 'Enable Self Heal?', 'Are you sure you want to enable automated self healing?',
                                            app.spec.syncPolicy.automated.prune, true)
                                        }>Enable</button>
                                    )}
                                </div>
                            </div>
                        </React.Fragment>
                    )}
                </div>
            </div>
            )}</Consumer>
            <DataLoader load={() => services.authService.settings()}>
            {(settings) => (
                settings.statusBadgeEnabled && (
                    <div className='white-box'>
                        <div className='white-box__details'>
                            <p>Status Badge     <img src={`/api/badge?name=${props.app.metadata.name}`}/> </p>
                            <div className='white-box__details-row'>
                                <DropDownMenu anchor={() => (<p>{badgeType} <i className='fa fa-caret-down'/></p>)}
                                    items={['URL', 'Markdown', 'Textile', 'Rdoc', 'AsciiDoc'].map((type) => ({ title: type, action: () => setBadgeType(type) }))}
                                />
                                <textarea onClick={(e) => (e.target as HTMLInputElement).select()} className='application-summary__badge' readOnly={true} value={
                                    badgeType === 'URL' ? badgeURL :
                                    badgeType === 'Markdown' ? `[![App Status](${badgeURL})](${appURL})` :
                                    badgeType === 'Textile' ? `!${badgeURL}!:${appURL}` :
                                    badgeType === 'Rdoc' ? `{<img src="${badgeURL}" alt="App Status" />}[${appURL}]` :
                                    badgeType === 'AsciiDoc' ? `image:${badgeURL}["App Status", link="${appURL}"]` : ''
                                }/>
                            </div>
                        </div>
                    </div>
                ) || null
            )}
            </DataLoader>
            <EditablePanel save={(props.updateApp)} values={app} title='Info' items={infoItems} onModeSwitch={() => setAdjustedCount(0)}/>
        </React.Fragment>
    );
};
