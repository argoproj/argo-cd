/* eslint-disable no-prototype-builtins */
import {AutocompleteField, Checkbox, DataLoader, DropDownMenu, FormField, HelpIcon, Select} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import {FieldApi, Form, FormApi, FormField as ReactFormField, Text} from 'react-form';
import {YamlEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationSyncOptionsField} from '../application-sync-options/application-sync-options';
import {SetFinalizerOnApplication} from './set-finalizer-on-application';
import './application-create-panel.scss';
import {debounce} from 'lodash-es';
import {SourcePanel} from './source-panel';

const jsonMergePatch = require('json-merge-patch');

const DEFAULT_APP: Partial<models.Application> = {
    apiVersion: 'argoproj.io/v1alpha1',
    kind: 'Application',
    metadata: {
        name: ''
    },
    spec: {
        destination: {
            name: '',
            namespace: '',
            server: ''
        },
        sources: [
            {
                path: '',
                repoURL: '',
                targetRevision: 'HEAD'
            }
        ],
        project: ''
    }
};

const AutoSyncFormField = ReactFormField((props: {fieldApi: FieldApi; className: string}) => {
    const manual = 'Manual';
    const auto = 'Automatic';
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const automated = getValue() as models.Automated;

    return (
        <React.Fragment>
            <label>Sync Policy</label>
            <Select
                value={automated ? auto : manual}
                options={[manual, auto]}
                onChange={opt => {
                    setValue(opt.value === auto ? {prune: false, selfHeal: false} : null);
                }}
            />
            {automated && (
                <div className='application-create-panel__sync-params'>
                    <div className='checkbox-container'>
                        <Checkbox onChange={val => setValue({...automated, prune: val})} checked={!!automated.prune} id='policyPrune' />
                        <label htmlFor='policyPrune'>Prune Resources</label>
                        <HelpIcon title='If checked, Argo will delete resources if they are no longer defined in Git' />
                    </div>
                    <div className='checkbox-container'>
                        <Checkbox onChange={val => setValue({...automated, selfHeal: val})} checked={!!automated.selfHeal} id='policySelfHeal' />
                        <label htmlFor='policySelfHeal'>Self Heal</label>
                        <HelpIcon title='If checked, Argo will force the state defined in Git into the cluster when a deviation in the cluster is detected' />
                    </div>
                </div>
            )}
        </React.Fragment>
    );
});

export const ApplicationCreatePanel = (props: {
    app: models.Application;
    onAppChanged: (app: models.Application) => any;
    createApp: (app: models.Application) => any;
    getFormApi: (api: FormApi) => any;
}) => {
    const [yamlMode, setYamlMode] = React.useState(false);
    const [destFormat, setDestFormat] = React.useState('URL');
    const [retry, setRetry] = React.useState(false);
    const app = deepMerge(DEFAULT_APP, props.app || {});
    const debouncedOnAppChanged = debounce(props.onAppChanged, 800);
    const [sourcesIndexList, setSourcesIndexList] = React.useState([0]);

    React.useEffect(() => {
        if (app?.spec?.destination?.name && app.spec.destination.name !== '') {
            setDestFormat('NAME');
        } else {
            setDestFormat('URL');
        }

        return () => {
            debouncedOnAppChanged.cancel();
        };
    }, [debouncedOnAppChanged]);

    function addSource() {
        const sources = sourcesIndexList;
        const last_index = sources.pop();
        sources.push(last_index);
        sources.push(last_index + 1);
        setSourcesIndexList(sources);
    }

    return (
        <React.Fragment>
            <DataLoader
                key='creation-deps'
                load={() =>
                    Promise.all([
                        services.projects.list('items.metadata.name').then(projects => projects.map(proj => proj.metadata.name).sort()),
                        services.clusters.list().then(clusters => clusters.sort()),
                        services.repos.list()
                    ]).then(([projects, clusters, reposInfo]) => ({projects, clusters, reposInfo}))
                }>
                {({projects, clusters, reposInfo}) => {
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
                                        if (props.app.hasOwnProperty('spec') && props.app.spec.hasOwnProperty('sources')) {
                                            const numSources = props.app.spec.sources.length < 1 ? 1 : props.app.spec.sources.length;
                                            const range = () => Array.from({length: numSources}, (value, key) => key);
                                            setSourcesIndexList(range);
                                        }
                                        return true;
                                    }}
                                />
                            )) || (
                                <Form
                                    validateError={(a: models.Application) => ({
                                        'metadata.name': !a.metadata.name && 'Application Name is required',
                                        'spec.project': !a.spec.project && 'Project Name is required',
                                        'spec.sources[0].repoURL': !a.spec.sources[0].repoURL && 'Repository URL is required',
                                        'spec.sources[0].targetRevision': !a.spec.sources[0].targetRevision && 'Version is required',
                                        'spec.sources[0].path': !a.spec.sources[0].path && !a.spec.sources[0].chart && 'Path is required',
                                        'spec.source[0].chart': !a.spec.sources[0].path && !a.spec.sources[0].chart && 'Chart is required',
                                        // Verify cluster URL when there is no cluster name field or the name value is empty
                                        'spec.destination.server':
                                            !a.spec.destination.server &&
                                            (!a.spec.destination.hasOwnProperty('name') || a.spec.destination.name === '') &&
                                            'Cluster URL is required',
                                        // Verify cluster name when there is no cluster URL field or the URL value is empty
                                        'spec.destination.name':
                                            !a.spec.destination.name &&
                                            (!a.spec.destination.hasOwnProperty('server') || a.spec.destination.server === '') &&
                                            'Cluster name is required'
                                    })}
                                    defaultValues={app}
                                    formDidUpdate={state => debouncedOnAppChanged(state.values as any)}
                                    onSubmit={props.createApp}
                                    getApi={props.getFormApi}>
                                    {api => {
                                        const generalPanel = () => (
                                            <div className='white-box'>
                                                <p>GENERAL</p>
                                                {/*
                                                    Need to specify "type='button'" because the default type 'submit'
                                                    will activate yaml mode whenever enter is pressed while in the panel.
                                                    This causes problems with some entry fields that require enter to be
                                                    pressed for the value to be accepted.

                                                    See https://github.com/argoproj/argo-cd/issues/4576
                                                */}
                                                {!yamlMode && (
                                                    <button
                                                        type='button'
                                                        className='argo-button argo-button--base application-create-panel__yaml-button'
                                                        onClick={() => setYamlMode(true)}>
                                                        Edit as YAML
                                                    </button>
                                                )}
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={api}
                                                        label='Application Name'
                                                        qeId='application-create-field-app-name'
                                                        field='metadata.name'
                                                        component={Text}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={api}
                                                        label='Project Name'
                                                        qeId='application-create-field-project'
                                                        field='spec.project'
                                                        component={AutocompleteField}
                                                        componentProps={{
                                                            items: projects,
                                                            filterSuggestions: true
                                                        }}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        formApi={api}
                                                        field='spec.syncPolicy.automated'
                                                        qeId='application-create-field-sync-policy'
                                                        component={AutoSyncFormField}
                                                    />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField formApi={api} field='metadata.finalizers' component={SetFinalizerOnApplication} />
                                                </div>
                                                <div className='argo-form-row'>
                                                    <label>Sync Options</label>
                                                    <FormField formApi={api} field='spec.syncPolicy.syncOptions' component={ApplicationSyncOptionsField} />
                                                    <ApplicationRetryOptions
                                                        formApi={api}
                                                        field='spec.syncPolicy.retry'
                                                        retry={retry || (api.getFormState().values.spec.syncPolicy && api.getFormState().values.spec.syncPolicy.retry)}
                                                        setRetry={setRetry}
                                                        initValues={api.getFormState().values.spec.syncPolicy ? api.getFormState().values.spec.syncPolicy.retry : null}
                                                    />
                                                </div>
                                            </div>
                                        );

                                        const sourcePanel = () => (
                                            <div className='white-box'>
                                                <p>SOURCES</p>

                                                {sourcesIndexList.map(index => (
                                                    <SourcePanel key={'create_source_' + index} index={index} api={api} reposInfo={reposInfo} appCurrent={props.app} />
                                                ))}
                                                <div className='source-panel-buttons'>
                                                    <button key={'add_source_button'} onClick={() => addSource()} disabled={false} className='argo-button argo-button--base'>
                                                        <i className='fa fa-plus' style={{marginLeft: '-5px', marginRight: '5px'}} />
                                                        <span style={{marginRight: '8px'}} />
                                                        Add Source
                                                    </button>
                                                </div>
                                            </div>
                                        );
                                        const destinationPanel = () => (
                                            <div className='white-box'>
                                                <p>DESTINATION</p>
                                                <div className='row argo-form-row'>
                                                    {(destFormat.toUpperCase() === 'URL' && (
                                                        <div className='columns small-10'>
                                                            <FormField
                                                                formApi={api}
                                                                label='Cluster URL'
                                                                qeId='application-create-field-cluster-url'
                                                                field='spec.destination.server'
                                                                componentProps={{items: clusters.map(cluster => cluster.server)}}
                                                                component={AutocompleteField}
                                                            />
                                                        </div>
                                                    )) || (
                                                        <div className='columns small-10'>
                                                            <FormField
                                                                formApi={api}
                                                                label='Cluster Name'
                                                                qeId='application-create-field-cluster-name'
                                                                field='spec.destination.name'
                                                                componentProps={{items: clusters.map(cluster => cluster.name)}}
                                                                component={AutocompleteField}
                                                            />
                                                        </div>
                                                    )}
                                                    <div className='columns small-2'>
                                                        <div style={{paddingTop: '1.5em'}}>
                                                            <DropDownMenu
                                                                anchor={() => (
                                                                    <p>
                                                                        {destFormat} <i className='fa fa-caret-down' />
                                                                    </p>
                                                                )}
                                                                qeId='application-create-dropdown-destination'
                                                                items={['URL', 'NAME'].map((type: 'URL' | 'NAME') => ({
                                                                    title: type,
                                                                    action: () => {
                                                                        if (destFormat !== type) {
                                                                            const updatedApp = api.getFormState().values as models.Application;
                                                                            if (type === 'URL') {
                                                                                delete updatedApp.spec.destination.name;
                                                                            } else {
                                                                                delete updatedApp.spec.destination.server;
                                                                            }
                                                                            api.setAllValues(updatedApp);
                                                                            setDestFormat(type);
                                                                        }
                                                                    }
                                                                }))}
                                                            />
                                                        </div>
                                                    </div>
                                                </div>
                                                <div className='argo-form-row'>
                                                    <FormField
                                                        qeId='application-create-field-namespace'
                                                        formApi={api}
                                                        label='Namespace'
                                                        field='spec.destination.namespace'
                                                        component={Text}
                                                    />
                                                </div>
                                            </div>
                                        );
                                        return (
                                            <form onSubmit={api.submitForm} role='form' className='width-control'>
                                                {generalPanel()}

                                                {sourcePanel()}

                                                {destinationPanel()}
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
