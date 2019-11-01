import {Checkbox, DataLoader, DropDownMenu, FormField, Select} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import {FieldApi, Form, FormApi, FormField as ReactFormField, Text} from 'react-form';
import {AutocompleteField, clusterTitle, RevisionHelpIcon, YamlEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';

const jsonMergePatch = require('json-merge-patch');

require('./application-create-panel.scss');

const appTypes = new Array<{field: string; type: models.AppSourceType}>(
    {type: 'Helm', field: 'helm'},
    {type: 'Kustomize', field: 'kustomize'},
    {type: 'Ksonnet', field: 'ksonnet'},
    {type: 'Directory', field: 'directory'},
    {type: 'Plugin', field: 'plugin'}
);

const DEFAULT_APP: Partial<models.Application> = {
    apiVersion: 'argoproj.io/v1alpha1',
    kind: 'Application',
    metadata: {
        name: ''
    },
    spec: {
        destination: {
            namespace: '',
            server: ''
        },
        source: {
            path: '',
            repoURL: '',
            targetRevision: 'HEAD'
        },
        project: ''
    }
};

const AutoSyncFormField = ReactFormField((props: {fieldApi: FieldApi; className: string}) => {
    const manual = 'Manual';
    const auto = 'Automatic';

    const {
        fieldApi: {getValue, setValue}
    } = props;
    const policy = getValue() as models.SyncPolicy;

    return (
        <React.Fragment>
            <label>Sync Policy</label>
            <Select
                value={policy && policy.automated ? auto : manual}
                options={[manual, auto]}
                onChange={opt => {
                    setValue(opt.value === auto ? {automated: {prune: false, selfHeal: false}} : null);
                }}
            />
            {policy && policy.automated && (
                <div className='application-create-panel__sync-params'>
                    <Checkbox onChange={val => setValue({automated: {...policy.automated, prune: val}})} checked={policy.automated.prune} id='policyPrune' />{' '}
                    <label htmlFor='policyPrune'>Prune Resources</label>{' '}
                    <Checkbox onChange={val => setValue({automated: {...policy.automated, selfHeal: val}})} checked={policy.automated.selfHeal} id='policySelfHeal' />{' '}
                    <label htmlFor='selfHeal'>Self Heal</label>
                </div>
            )}
        </React.Fragment>
    );
});

function normalizeAppSource(app: models.Application, type: string): boolean {
    const repoType = (app.spec.source.hasOwnProperty('chart') && 'helm') || 'git';
    if (repoType !== type) {
        if (type === 'git') {
            app.spec.source.path = app.spec.source.chart;
            delete app.spec.source.chart;
            app.spec.source.targetRevision = 'HEAD';
        } else {
            app.spec.source.chart = app.spec.source.path;
            delete app.spec.source.path;
            app.spec.source.targetRevision = '';
        }
        return true;
    }
    return false;
}

export const ApplicationCreatePanel = (props: {
    app: models.Application;
    onAppChanged: (app: models.Application) => any;
    createApp: (app: models.Application) => any;
    getFormApi: (api: FormApi) => any;
}) => {
    const [yamlMode, setYamlMode] = React.useState(false);
    const [explicitPathType, setExplicitPathType] = React.useState<{path: string; type: models.AppSourceType}>(null);

    function normalizeTypeFields(formApi: FormApi, type: models.AppSourceType) {
        const app = formApi.getFormState().values;
        for (const item of appTypes) {
            if (item.type !== type) {
                delete app.spec.source[item.field];
            }
        }
        formApi.setAllValues(app);
    }

    return (
        <React.Fragment>
            <DataLoader
                key='creation-deps'
                load={() =>
                    Promise.all([
                        services.projects.list().then(projects => projects.map(proj => proj.metadata.name).sort()),
                        services.clusters
                            .list()
                            .then(clusters =>
                                clusters.map(cluster => ({label: clusterTitle(cluster), value: cluster.server})).sort((first, second) => first.label.localeCompare(second.label))
                            ),
                        services.repos.list()
                    ]).then(([projects, clusters, reposInfo]) => ({projects, clusters, reposInfo}))
                }>
                {({projects, clusters, reposInfo}) => {
                    const repos = reposInfo.map(info => info.repo).sort();
                    const app = deepMerge(DEFAULT_APP, props.app || {});
                    const repoInfo = reposInfo.find(info => info.repo === app.spec.source.repoURL);
                    if (repoInfo) {
                        normalizeAppSource(app, repoInfo.type || 'git');
                    }
                    return (
                        <div className='application-create-panel'>
                            {(yamlMode && (
                                <YamlEditor
                                    minHeight={800}
                                    initialEditMode={true}
                                    input={app}
                                    onCancel={() => setYamlMode(false)}
                                    onSave={async patch => {
                                        props.onAppChanged(jsonMergePatch.apply(app, JSON.parse(patch)));
                                        setYamlMode(false);
                                        return true;
                                    }}
                                />
                            )) || (
                                <Form
                                    validateError={(a: models.Application) => ({
                                        'metadata.name': !a.metadata.name && 'Application name is required',
                                        'spec.project': !a.spec.project && 'Project name is required',
                                        'spec.source.repoURL': !a.spec.source.repoURL && 'Repository URL is required',
                                        'spec.source.targetRevision': !a.spec.source.targetRevision && a.spec.source.hasOwnProperty('chart') && 'Version is required',
                                        'spec.source.path': !a.spec.source.path && !a.spec.source.chart && 'Path is required',
                                        'spec.source.chart': !a.spec.source.path && !a.spec.source.chart && 'Chart is required',
                                        'spec.destination.server': !a.spec.destination.server && 'Cluster is required',
                                        'spec.destination.namespace': !a.spec.destination.namespace && 'Namespace is required'
                                    })}
                                    defaultValues={app}
                                    formDidUpdate={state => props.onAppChanged(state.values as any)}
                                    onSubmit={props.createApp}
                                    getApi={props.getFormApi}>
                                    {api => {
                                        const generalPanel = () => (
                                            <div className='white-box'>
                                                <p>GENERAL</p>
                                                {!yamlMode && (
                                                    <button className='argo-button argo-button--base application-create-panel__yaml-button' onClick={() => setYamlMode(true)}>
                                                        Edit as YAML
                                                    </button>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField formApi={api} label='Application Name' field='metadata.name' component={Text} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={api}
                                                        label='Project'
                                                        field='spec.project'
                                                        component={AutocompleteField}
                                                        componentProps={{items: projects}}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={api} field='spec.syncPolicy' component={AutoSyncFormField} />
                                                </div>
                                            </div>
                                        );

                                        const repoType = (api.getFormState().values.spec.source.hasOwnProperty('chart') && 'helm') || 'git';
                                        const sourcePanel = () => (
                                            <div className='white-box'>
                                                <p>SOURCE</p>
                                                <div className='row argo-form-row'>
                                                    <div className='columns small-10'>
                                                        <FormField
                                                            formApi={api}
                                                            label='Repository URL'
                                                            field='spec.source.repoURL'
                                                            component={AutocompleteField}
                                                            componentProps={{items: repos}}
                                                        />
                                                    </div>
                                                    <div className='columns small-2'>
                                                        <div style={{paddingTop: '1.5em'}}>
                                                            {(repoInfo && (
                                                                <React.Fragment>
                                                                    <span>{(repoInfo.type || 'git').toUpperCase()}</span> <i className='fa fa-check' />
                                                                </React.Fragment>
                                                            )) || (
                                                                <DropDownMenu
                                                                    anchor={() => (
                                                                        <p>
                                                                            {repoType.toUpperCase()} <i className='fa fa-caret-down' />
                                                                        </p>
                                                                    )}
                                                                    items={['git', 'helm'].map((type: 'git' | 'helm') => ({
                                                                        title: type.toUpperCase(),
                                                                        action: () => {
                                                                            if (repoType !== type) {
                                                                                const updatedApp = api.getFormState().values as models.Application;
                                                                                if (normalizeAppSource(updatedApp, type)) {
                                                                                    api.setAllValues(updatedApp);
                                                                                }
                                                                            }
                                                                        }
                                                                    }))}
                                                                />
                                                            )}
                                                        </div>
                                                    </div>
                                                </div>
                                                {(repoType === 'git' && (
                                                    <React.Fragment>
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={api} label='Revision' field='spec.source.targetRevision' component={Text} />
                                                            <RevisionHelpIcon type='git' />
                                                        </div>
                                                        <div className='argo-form-row'>
                                                            <DataLoader
                                                                input={{repoURL: app.spec.source.repoURL, revision: app.spec.source.targetRevision}}
                                                                load={async src =>
                                                                    (src.repoURL &&
                                                                        services.repos
                                                                            .apps(src.repoURL, src.revision)
                                                                            .then(apps => Array.from(new Set(apps.map(item => item.path))).sort())
                                                                            .catch(() => new Array<string>())) ||
                                                                    new Array<string>()
                                                                }>
                                                                {(apps: string[]) => (
                                                                    <FormField
                                                                        formApi={api}
                                                                        label='Path'
                                                                        field='spec.source.path'
                                                                        component={AutocompleteField}
                                                                        componentProps={{
                                                                            items: apps,
                                                                            filterSuggestions: true
                                                                        }}
                                                                    />
                                                                )}
                                                            </DataLoader>
                                                        </div>
                                                    </React.Fragment>
                                                )) || (
                                                    <DataLoader
                                                        input={{repoURL: app.spec.source.repoURL}}
                                                        load={async src =>
                                                            (src.repoURL && services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())) ||
                                                            new Array<models.HelmChart>()
                                                        }>
                                                        {(charts: models.HelmChart[]) => {
                                                            const selectedChart = charts.find(chart => chart.name === api.getFormState().values.spec.source.chart);
                                                            return (
                                                                <div className='row argo-form-row'>
                                                                    <div className='columns small-10'>
                                                                        <FormField
                                                                            formApi={api}
                                                                            label='Chart'
                                                                            field='spec.source.chart'
                                                                            component={AutocompleteField}
                                                                            componentProps={{
                                                                                items: charts.map(chart => chart.name),
                                                                                filterSuggestions: true
                                                                            }}
                                                                        />
                                                                    </div>
                                                                    <div className='columns small-2'>
                                                                        <FormField
                                                                            formApi={api}
                                                                            field='spec.source.targetRevision'
                                                                            component={AutocompleteField}
                                                                            componentProps={{
                                                                                items: (selectedChart && selectedChart.versions) || []
                                                                            }}
                                                                        />
                                                                        <RevisionHelpIcon type='helm' />
                                                                    </div>
                                                                </div>
                                                            );
                                                        }}
                                                    </DataLoader>
                                                )}
                                            </div>
                                        );

                                        const destinationPanel = () => (
                                            <div className='white-box'>
                                                <p>DESTINATION</p>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={api}
                                                        label='Cluster'
                                                        field='spec.destination.server'
                                                        componentProps={{items: clusters}}
                                                        component={AutocompleteField}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={api} label='Namespace' field='spec.destination.namespace' component={Text} />
                                                </div>
                                            </div>
                                        );

                                        const typePanel = () => (
                                            <DataLoader
                                                input={{
                                                    repoURL: app.spec.source.repoURL,
                                                    path: app.spec.source.path,
                                                    chart: app.spec.source.chart,
                                                    targetRevision: app.spec.source.targetRevision
                                                }}
                                                load={async src => {
                                                    if (src.repoURL && src.targetRevision && (src.path || src.chart)) {
                                                        return services.repos.appDetails(src).catch(() => ({
                                                            type: 'Directory',
                                                            details: {}
                                                        }));
                                                    } else {
                                                        return {
                                                            type: 'Directory',
                                                            details: {}
                                                        };
                                                    }
                                                }}>
                                                {(details: models.RepoAppDetails) => {
                                                    const type = (explicitPathType && explicitPathType.path === app.spec.source.path && explicitPathType.type) || details.type;
                                                    if (details.type !== type) {
                                                        switch (type) {
                                                            case 'Helm':
                                                                details = {type, path: details.path, helm: {name: '', valueFiles: [], path: '', parameters: []}};
                                                                break;
                                                            case 'Kustomize':
                                                                details = {type, path: details.path, kustomize: {path: ''}};
                                                                break;
                                                            case 'Ksonnet':
                                                                details = {type, path: details.path, ksonnet: {name: '', path: '', environments: {}, parameters: []}};
                                                                break;
                                                            case 'Plugin':
                                                                details = {type, path: details.path, plugin: {name: '', env: []}};
                                                                break;
                                                            // Directory
                                                            default:
                                                                details = {type, path: details.path, directory: {}};
                                                                break;
                                                        }
                                                    }
                                                    return (
                                                        <React.Fragment>
                                                            <DropDownMenu
                                                                anchor={() => (
                                                                    <p>
                                                                        {type} <i className='fa fa-caret-down' />
                                                                    </p>
                                                                )}
                                                                items={appTypes.map(item => ({
                                                                    title: item.type,
                                                                    action: () => {
                                                                        setExplicitPathType({type: item.type, path: app.spec.source.path});
                                                                        normalizeTypeFields(api, item.type);
                                                                    }
                                                                }))}
                                                            />
                                                            <ApplicationParameters
                                                                noReadonlyMode={true}
                                                                application={app}
                                                                details={details}
                                                                save={async updatedApp => {
                                                                    api.setAllValues(updatedApp);
                                                                }}
                                                            />
                                                        </React.Fragment>
                                                    );
                                                }}
                                            </DataLoader>
                                        );

                                        return (
                                            <form onSubmit={api.submitForm} role='form' className='width-control'>
                                                {generalPanel()}

                                                {sourcePanel()}

                                                {destinationPanel()}

                                                {typePanel()}
                                            </form>
                                        );
                                    }}
                                </Form>
                            )}
                        </div>
                    );
                }}
            </DataLoader>
        </React.Fragment>
    );
};
