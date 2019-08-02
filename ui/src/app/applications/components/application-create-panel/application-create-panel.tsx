import { Checkbox, DataLoader, DropDownMenu, FormField, Select } from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import { FieldApi, Form, FormApi, FormField as ReactFormField, Text } from 'react-form';

const jsonMergePatch = require('json-merge-patch');

import { AutocompleteField, CheckboxField, clusterTitle, TagsInputField, YamlEditor } from '../../../shared/components';
import * as models from '../../../shared/models';
import { services } from '../../../shared/services';

require('./application-create-panel.scss');

const appTypes = new Array<{ field: string, type: models.AppSourceType }>(
    { type: 'Helm', field: 'helm'},
    { type: 'Kustomize', field: 'kustomize'},
    { type: 'Ksonnet', field: 'ksonnet'},
    { type: 'Directory', field: 'directory'},
    { type: 'Plugin', field: 'plugin'},
);

const DEFAULT_APP: Partial<models.Application> = {
    apiVersion: 'argoproj.io/v1alpha1',
    metadata: {
        name: '',
    },
    spec: {
        destination: {
            namespace: '',
            server: '',
        },
        source: {
            path: '',
            repoURL: '',
            targetRevision: 'HEAD',
        },
        project: '',
    },
};

const AutoSyncFormField = ReactFormField((props: {fieldApi: FieldApi, className: string }) => {
    const manual = 'Manual';
    const auto = 'Automatic';

    const { fieldApi: {getValue, setValue}} = props;
    const policy = getValue() as models.SyncPolicy;

    return (
        <React.Fragment>
            <label>Sync Policy</label>
            <Select value={policy && policy.automated ? auto : manual} options={[ manual, auto ]} onChange={(opt) => {
                setValue( opt.value === auto ? {automated: { prune: false, selfHeal: false }} : null );
            }} />
            {policy && policy.automated && (
                <div className='application-create-panel__sync-params'>
                    <Checkbox onChange={(val) => setValue({ automated: {...policy.automated, prune: val} })}
                        checked={policy.automated.prune} id='policyPrune' /> <label htmlFor='policyPrune'>
                    Prune Resources</label> <Checkbox onChange={(val) => setValue({ automated: {...policy.automated, selfHeal: val} })}
                        checked={policy.automated.selfHeal} id='policySelfHeal' /> <label htmlFor='selfHeal'>
                    Self Heal</label>
                </div>
            )}
        </React.Fragment>
    );
});

export const ApplicationCreatePanel = (props: {
    app: models.Application,
    onAppChanged: (app: models.Application) => any;
    createApp: (app: models.Application) => any;
    getFormApi: (api: FormApi) => any;
}) => {

    const [yamlMode, setYamlMode] = React.useState(false);
    const [explicitPathType, setExplicitPathType] = React.useState<{ path: string, type: models.AppSourceType }>(null);

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
            <DataLoader key='creation-deps' load={() => Promise.all([
                services.projects.list().then((projects) => projects.map((proj) => proj.metadata.name).sort()),
                services.clusters.list().then((clusters) => clusters
                    .map((cluster) => ({label: clusterTitle(cluster), value: cluster.server}))
                    .sort((first, second) => first.label.localeCompare(second.label)),
                ),
                services.repos.list().then((repos) => repos.map((repo) => repo.repo).sort()),
            ]).then(([projects, clusters, repos]) => ({projects, clusters, repos}))}>
            {({projects, clusters, repos}) => {
                const app = deepMerge(DEFAULT_APP, props.app || {});
                return (
                    <div className='application-create-panel'>
                        {yamlMode && (
                            <YamlEditor minHeight={800} initialEditMode={true} input={app} onCancel={() => setYamlMode(false)} onSave={async (patch) => {
                                props.onAppChanged(jsonMergePatch.apply(app, JSON.parse(patch)));
                                setYamlMode(false);
                                return true;
                            }}/>
                        ) || (
                            <Form validateError={(a: models.Application) => ({
                                'metadata.name': !a.metadata.name && 'Application name is required',
                                'spec.project': !a.spec.project && 'Project name is required',
                                'spec.source.repoURL': !a.spec.source.repoURL && 'Repository URL is required',
                                'spec.source.targetRevision': !a.spec.source.targetRevision && 'Revision is required',
                                'spec.source.path': !a.spec.source.path && 'Path is required',
                                'spec.destination.server': !a.spec.destination.server && 'Cluster is required',
                                'spec.destination.namespace': !a.spec.destination.namespace && 'Namespace is required',
                            })} defaultValues={app} formDidUpdate={(state) => props.onAppChanged(state.values as any)} onSubmit={props.createApp} getApi={props.getFormApi}>
                            {(api) => {
                                const generalPanel = () => (
                                    <div className='white-box'>
                                        <p>GENERAL</p>
                                        {!yamlMode && (
                                            <button className='argo-button argo-button--base application-create-panel__yaml-button' onClick={() => setYamlMode(true)}>
                                                Edit as YAML</button>
                                        )}
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Application Name' field='metadata.name' component={Text}/>
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Project' field='spec.project' component={AutocompleteField} componentProps={{items: projects}} />
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} field='spec.syncPolicy' component={AutoSyncFormField} />
                                        </div>
                                    </div>
                                );

                                const sourcePanel = () => (
                                    <div className='white-box'>
                                        <p>SOURCE</p>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api}
                                                label='Repository URL' field='spec.source.repoURL' component={AutocompleteField} componentProps={{items: repos}}/>
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Revision' field='spec.source.targetRevision' component={Text}/>
                                        </div>
                                        <div className='argo-form-row'>
                                            <DataLoader
                                                    input={{ repoURL: app.spec.source.repoURL, revision: app.spec.source.targetRevision }}
                                                    load={async (src) => src.repoURL &&
                                                        services.repos.apps(src.repoURL, src.revision)
                                                            .then((apps) => Array.from(new Set(apps.map((item) => item.path))).sort())
                                                            .catch(() => new Array<string>()) ||
                                                        new Array<string>()
                                                    }>
                                            {(apps: string[]) => (
                                                <FormField formApi={api} label='Path' field='spec.source.path' component={AutocompleteField} componentProps={{
                                                    items: apps, filterSuggestions: true,
                                                }}/>
                                            )}
                                            </DataLoader>
                                        </div>
                                    </div>
                                );

                                const destinationPanel = () => (
                                    <div className='white-box'>
                                        <p>DESTINATION</p>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api}
                                                label='Cluster' field='spec.destination.server' componentProps={{items: clusters}} component={AutocompleteField}/>
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Namespace' field='spec.destination.namespace' component={Text}/>
                                        </div>
                                    </div>
                                );

                                const typePanel = () => (
                                    <div className='white-box'>
                                        <DataLoader
                                            input={{repoURL: app.spec.source.repoURL, path: app.spec.source.path, targetRevision: app.spec.source.targetRevision }}
                                            load={async (src) => {
                                                if (src.repoURL && src.path) {
                                                    return services.repos.appDetails(src.repoURL, src.path, src.targetRevision).catch(() => ({
                                                        type: 'Directory',
                                                        details: {},
                                                    }));
                                                } else {
                                                    return {
                                                        type: 'Directory',
                                                        details: {},
                                                    };
                                                }
                                            }}>
                                        {(details: models.RepoAppDetails) => {
                                            const type = explicitPathType && explicitPathType.path === app.spec.source.path && explicitPathType.type || details.type;
                                            return (
                                                <React.Fragment>
                                                    <DropDownMenu anchor={() => (<p>{type} <i className='fa fa-caret-down'/></p>)} items={appTypes.map(
                                                        (item) => ({ title: item.type, action: () => {
                                                            setExplicitPathType({ type: item.type, path: app.spec.source.path });
                                                            normalizeTypeFields(api, item.type);
                                                        }}))}
                                                    />
                                                    {type === 'Directory' && (
                                                        <FormField formApi={api}
                                                            label='Include subdirectories' field='spec.source.directory.recurse' component={CheckboxField}/>
                                                    ) || type === 'Kustomize' && (
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={api} label='Name Prefix' field='spec.source.kustomize.namePrefix' component={Text} />
                                                        </div>
                                                    ) || type === 'Ksonnet' && (
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={api} label='Environment' field='spec.source.ksonnet.environment'
                                                                component={AutocompleteField} componentProps={{
                                                                    items: details.ksonnet && Object.keys(details.ksonnet.environments) || [],
                                                                }}
                                                            />
                                                        </div>
                                                    ) || type === 'Helm' && (
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={api} label='Values Files' field='spec.source.helm.valueFiles' componentProps={{
                                                                options: details.helm && details.helm.valueFiles,
                                                                noTagsLabel: 'No values files selected',
                                                            }} component={TagsInputField} />
                                                        </div>
                                                    ) || type === 'Plugin' && (
                                                        <div className='argo-form-row'>
                                                            <FormField formApi={api} label='Name' field='spec.source.plugin.name' component={Text} />
                                                        </div>
                                                    )}
                                                </React.Fragment>
                                            );
                                        }}
                                        </DataLoader>
                                    </div>
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
