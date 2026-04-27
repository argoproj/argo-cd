/* eslint-disable no-prototype-builtins */
import {AutocompleteField, Checkbox, DataLoader, DropDownMenu, FormField, HelpIcon, Select} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import {FieldApi, Form, FormApi, FormField as ReactFormField, Text} from 'react-form';
import {cloneDeep, debounce} from 'lodash-es';
import {YamlEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {AuthSettingsCtx} from '../../../shared/context';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationSyncOptionsField} from '../application-sync-options/application-sync-options';
import {SetFinalizerOnApplication} from './set-finalizer-on-application';
import {HydratorSourcePanel} from './hydrator-source-panel';
import {CollapsibleMultiSourceSection} from './collapsible-multi-source-section';
import {SourcePanel} from './source-panel';
import './application-create-panel.scss';
import {getAppDefaultSource} from '../utils';
import {APP_SOURCE_TYPES, normalizeTypeFieldsForSource} from '../shared/app-source-edit';

const jsonMergePatch = require('json-merge-patch');

const DEFAULT_APP: Partial<models.Application> = {
    apiVersion: 'argoproj.io/v1alpha1',
    kind: 'Application',
    metadata: {
        name: ''
    },
    spec: {
        destination: {
            name: undefined,
            namespace: '',
            server: undefined
        },
        source: {
            path: '',
            repoURL: '',
            targetRevision: 'HEAD'
        },
        sources: [],
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
                    setValue(opt.value === auto ? {prune: false, selfHeal: false, enabled: true} : null);
                }}
            />
            {automated && (
                <div className='application-create-panel__sync-params'>
                    <div className='checkbox-container'>
                        <Checkbox onChange={val => setValue({...automated, enabled: val})} checked={automated.enabled === undefined ? true : automated.enabled} id='policyEnable' />
                        <label htmlFor='policyEnable'>Enable Auto-Sync</label>
                        <HelpIcon title='If checked, application will automatically sync when changes are detected' />
                    </div>
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

function normalizeAppSource(app: models.Application, type: string): boolean {
    const source = getAppDefaultSource(app);
    const repoType = source.repoURL.startsWith('oci://') ? 'oci' : (source.hasOwnProperty('chart') && 'helm') || 'git';
    if (repoType !== type) {
        if (type === 'git' || type === 'oci') {
            source.path = source.chart;
            delete source.chart;
            source.targetRevision = 'HEAD';
        } else {
            source.chart = source.path;
            delete source.path;
            source.targetRevision = '';
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
    const [retry, setRetry] = React.useState(false);
    const app = deepMerge(DEFAULT_APP, props.app || {});
    const debouncedOnAppChanged = debounce(props.onAppChanged, 800);
    const [destinationFieldChanges, setDestinationFieldChanges] = React.useState({destFormat: 'URL', destFormatChanged: null});
    const comboSwitchedFromPanel = React.useRef(false);
    const currentRepoType = React.useRef(undefined);
    const lastGitOrHelmUrl = React.useRef('');
    const lastOciUrl = React.useRef('');
    const [isHydratorEnabled, setIsHydratorEnabled] = React.useState(!!app.spec.sourceHydrator);
    const [savedSyncSource, setSavedSyncSource] = React.useState(app.spec.sourceHydrator?.syncSource || {targetBranch: '', path: ''});
    let destinationComboValue = destinationFieldChanges.destFormat;
    const authSettingsCtx = React.useContext(AuthSettingsCtx);

    const [multiSourceMode, setMultiSourceMode] = React.useState(() => (app.spec?.sources?.length ?? 0) >= 2);

    React.useEffect(() => {
        comboSwitchedFromPanel.current = false;
    }, []);

    React.useEffect(() => {
        return () => {
            debouncedOnAppChanged.cancel();
        };
    }, [debouncedOnAppChanged]);

    const currentName = app.spec.destination.name;
    const currentServer = app.spec.destination.server;
    if (destinationFieldChanges.destFormatChanged !== null) {
        if (destinationComboValue == 'NAME') {
            if (currentName === undefined && currentServer !== undefined && comboSwitchedFromPanel.current === false) {
                destinationComboValue = 'URL';
            } else {
                delete app.spec.destination.server;
                if (currentName === undefined) {
                    app.spec.destination.name = '';
                }
            }
        } else {
            if (currentServer === undefined && currentName !== undefined && comboSwitchedFromPanel.current === false) {
                destinationComboValue = 'NAME';
            } else {
                delete app.spec.destination.name;
                if (currentServer === undefined) {
                    app.spec.destination.server = '';
                }
            }
        }
    } else {
        if (currentName === undefined && currentServer === undefined) {
            destinationComboValue = destinationFieldChanges.destFormat;
            app.spec.destination.server = '';
        } else {
            if (currentName != undefined) {
                destinationComboValue = 'NAME';
            } else {
                destinationComboValue = 'URL';
            }
        }
    }

    const onCreateApp = (data: models.Application) => {
        if (destinationComboValue === 'URL') {
            delete data.spec.destination.name;
        } else {
            delete data.spec.destination.server;
        }

        if (data.spec.sourceHydrator && !data.spec.sourceHydrator.hydrateTo?.targetBranch) {
            delete data.spec.sourceHydrator.hydrateTo;
        }

        if (multiSourceMode && data.spec.sources && data.spec.sources.length > 0) {
            delete data.spec.source;
        }

        props.createApp(data);
    };

    function handleAddSource(api: FormApi) {
        const updated = cloneDeep(api.getFormState().values) as models.Application;
        if (!multiSourceMode) {
            updated.spec.sources = [{...(updated.spec.source || {path: '', repoURL: '', targetRevision: 'HEAD'})}, {path: '', repoURL: '', targetRevision: 'HEAD'}];
            delete updated.spec.source;
            delete updated.spec.sourceHydrator;
            setIsHydratorEnabled(false);
            setMultiSourceMode(true);
        } else {
            if (!updated.spec.sources) {
                updated.spec.sources = [];
            }
            updated.spec.sources.push({path: '', repoURL: '', targetRevision: 'HEAD'});
        }
        api.setAllValues(updated);
    }

    function handleRemoveSource(api: FormApi, index: number) {
        const updated = cloneDeep(api.getFormState().values) as models.Application;
        const sources = updated.spec.sources;
        if (!sources || index < 0 || index >= sources.length) {
            return;
        }
        sources.splice(index, 1);
        if (sources.length === 0) {
            updated.spec.source = {path: '', repoURL: '', targetRevision: 'HEAD'};
            delete updated.spec.sources;
            setMultiSourceMode(false);
        } else if (sources.length === 1) {
            updated.spec.source = sources[0];
            delete updated.spec.sources;
            setMultiSourceMode(false);
        }
        api.setAllValues(updated);
    }

    return (
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
                const repos = reposInfo.map(info => info.repo).sort();
                const repoInfo = reposInfo.find(info => info.repo === app.spec.source?.repoURL);
                if (repoInfo) {
                    normalizeAppSource(app, repoInfo.type || currentRepoType.current || 'git');
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
                                    const next = jsonMergePatch.apply(app, JSON.parse(patch)) as models.Application;
                                    props.onAppChanged(next);
                                    setYamlMode(false);
                                    setMultiSourceMode((next.spec?.sources?.length ?? 0) >= 2);
                                    return true;
                                }}
                            />
                        )) || (
                            <Form
                                validateError={(a: models.Application) => {
                                    const hasHydrator = !!a.spec.sourceHydrator;
                                    const source = a.spec.source;

                                    const destinationErrors = {
                                        'spec.destination.server':
                                            !a.spec.destination.server && (!a.spec.destination.hasOwnProperty('name') || a.spec.destination.name === '')
                                                ? 'Cluster URL is required'
                                                : undefined,
                                        'spec.destination.name':
                                            !a.spec.destination.name && (!a.spec.destination.hasOwnProperty('server') || a.spec.destination.server === '')
                                                ? 'Cluster name is required'
                                                : undefined
                                    };

                                    if (multiSourceMode && !hasHydrator) {
                                        const errs: Record<string, string | undefined> = {
                                            'metadata.name': !a.metadata.name ? 'Application Name is required' : undefined,
                                            'spec.project': !a.spec.project ? 'Project Name is required' : undefined,
                                            ...destinationErrors
                                        };
                                        const sources = a.spec.sources || [];
                                        for (let i = 0; i < sources.length; i++) {
                                            const s = sources[i];
                                            errs[`spec.sources[${i}].repoURL`] = !s?.repoURL ? 'Repository URL is required' : undefined;
                                            errs[`spec.sources[${i}].targetRevision`] = !s?.targetRevision && s?.hasOwnProperty('chart') ? 'Version is required' : undefined;
                                            errs[`spec.sources[${i}].path`] = !s?.path && !s?.chart ? 'Path is required' : undefined;
                                            errs[`spec.sources[${i}].chart`] = !s?.path && !s?.chart ? 'Chart is required' : undefined;
                                        }
                                        return errs;
                                    }

                                    return {
                                        'metadata.name': !a.metadata.name ? 'Application Name is required' : undefined,
                                        'spec.project': !a.spec.project ? 'Project Name is required' : undefined,
                                        'spec.source.repoURL': !hasHydrator && !source?.repoURL ? 'Repository URL is required' : undefined,
                                        'spec.source.targetRevision':
                                            !hasHydrator && !source?.targetRevision && source?.hasOwnProperty('chart') ? 'Version is required' : undefined,
                                        'spec.source.path': !hasHydrator && !source?.path && !source?.chart ? 'Path is required' : undefined,
                                        'spec.source.chart': !hasHydrator && !source?.path && !source?.chart ? 'Chart is required' : undefined,
                                        ...destinationErrors
                                    };
                                }}
                                defaultValues={app}
                                formDidUpdate={state => debouncedOnAppChanged(state.values as any)}
                                onSubmit={onCreateApp}
                                getApi={props.getFormApi}>
                                {api => {
                                    const formApp = api.getFormState().values as models.Application;

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
                                                <FormField formApi={api} label='Application Name' qeId='application-create-field-app-name' field='metadata.name' component={Text} />
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

                                    const sourcePanel = () => {
                                        if (multiSourceMode) {
                                            const count = formApp.spec.sources?.length ?? 0;
                                            return (
                                                <div className='white-box'>
                                                    <p>SOURCES</p>
                                                    {Array.from({length: count}, (_, i) => (
                                                        <CollapsibleMultiSourceSection
                                                            key={`msrc-${i}`}
                                                            index={i}
                                                            formApi={api}
                                                            repos={repos}
                                                            reposInfo={reposInfo}
                                                            formApp={formApp}
                                                            canRemove={count >= 2}
                                                            onRemove={() => handleRemoveSource(api, i)}
                                                        />
                                                    ))}
                                                    <div className='application-create-panel__add-source'>
                                                        <button type='button' className='argo-button argo-button--base' onClick={() => handleAddSource(api)}>
                                                            <i className='fa fa-plus' style={{marginLeft: '-5px', marginRight: '5px'}} />
                                                            Add Source
                                                        </button>
                                                    </div>
                                                </div>
                                            );
                                        }

                                        return (
                                            <div className='white-box'>
                                                <p>SOURCE</p>
                                                {authSettingsCtx?.hydratorEnabled && (
                                                    <div className='row argo-form-row'>
                                                        <div className='columns small-12'>
                                                            <div className='checkbox-container'>
                                                                <Checkbox
                                                                    onChange={(val: boolean) => {
                                                                        const updatedApp = api.getFormState().values as models.Application;
                                                                        if (val) {
                                                                            if (!updatedApp.spec.sourceHydrator) {
                                                                                updatedApp.spec.sourceHydrator = {
                                                                                    drySource: {
                                                                                        repoURL: updatedApp.spec.source.repoURL,
                                                                                        targetRevision: updatedApp.spec.source.targetRevision,
                                                                                        path: updatedApp.spec.source.path
                                                                                    },
                                                                                    syncSource: savedSyncSource
                                                                                };
                                                                                delete updatedApp.spec.source;
                                                                            }
                                                                        } else if (updatedApp.spec.sourceHydrator) {
                                                                            setSavedSyncSource(updatedApp.spec.sourceHydrator.syncSource);
                                                                            updatedApp.spec.source = updatedApp.spec.sourceHydrator.drySource;
                                                                            delete updatedApp.spec.sourceHydrator;
                                                                        }
                                                                        api.setAllValues(updatedApp);
                                                                        setIsHydratorEnabled(val);
                                                                    }}
                                                                    checked={!!(api.getFormState().values as models.Application).spec.sourceHydrator}
                                                                    id='enable-source-hydrator'
                                                                />
                                                                <label htmlFor='enable-source-hydrator'>enable source hydrator</label>
                                                            </div>
                                                        </div>
                                                    </div>
                                                )}
                                                {isHydratorEnabled ? (
                                                    <HydratorSourcePanel formApi={api} repos={repos} />
                                                ) : (
                                                    <React.Fragment>
                                                        <SourcePanel
                                                            formApi={api}
                                                            repos={repos}
                                                            repoInfo={repoInfo}
                                                            currentRepoType={currentRepoType}
                                                            lastGitOrHelmUrl={lastGitOrHelmUrl}
                                                            lastOciUrl={lastOciUrl}
                                                        />
                                                        <div className='application-create-panel__add-source'>
                                                            <button type='button' className='argo-button argo-button--base' onClick={() => handleAddSource(api)}>
                                                                <i className='fa fa-plus' style={{marginLeft: '-5px', marginRight: '5px'}} />
                                                                Add Source
                                                            </button>
                                                        </div>
                                                    </React.Fragment>
                                                )}
                                            </div>
                                        );
                                    };
                                    const destinationPanel = () => (
                                        <div className='white-box'>
                                            <p>DESTINATION</p>
                                            <div className='row argo-form-row'>
                                                {(destinationComboValue.toUpperCase() === 'URL' && (
                                                    <div className='columns small-10'>
                                                        <FormField
                                                            formApi={api}
                                                            label='Cluster URL'
                                                            qeId='application-create-field-cluster-url'
                                                            field='spec.destination.server'
                                                            componentProps={{
                                                                items: clusters.map(cluster => cluster.server),
                                                                filterSuggestions: true
                                                            }}
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
                                                            componentProps={{
                                                                items: clusters.map(cluster => cluster.name),
                                                                filterSuggestions: true
                                                            }}
                                                            component={AutocompleteField}
                                                        />
                                                    </div>
                                                )}
                                                <div className='columns small-2'>
                                                    <div style={{paddingTop: '1.5em'}}>
                                                        <DropDownMenu
                                                            anchor={() => (
                                                                <p>
                                                                    {destinationComboValue} <i className='fa fa-caret-down' />
                                                                </p>
                                                            )}
                                                            qeId='application-create-dropdown-destination'
                                                            items={['URL', 'NAME'].map((type: 'URL' | 'NAME') => ({
                                                                title: type,
                                                                action: () => {
                                                                    if (destinationComboValue !== type) {
                                                                        destinationComboValue = type;
                                                                        comboSwitchedFromPanel.current = true;
                                                                        setDestinationFieldChanges({destFormat: type, destFormatChanged: 'changed'});
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

                                    const typePanel = () => {
                                        const liveApp = api.getFormState().values as models.Application;
                                        const liveSrc = liveApp.spec.source;
                                        return (
                                            <DataLoader
                                                input={{
                                                    repoURL: liveSrc?.repoURL,
                                                    path: liveSrc?.path,
                                                    chart: liveSrc?.chart,
                                                    targetRevision: liveSrc?.targetRevision,
                                                    appName: liveApp.metadata.name,
                                                    project: liveApp.spec.project
                                                }}
                                                load={async src => {
                                                    if (src.repoURL && src.targetRevision && (src.path || src.chart)) {
                                                        return services.repos.appDetails(src, src.appName, src.project, 0, 0).catch(() => ({
                                                            type: 'Directory',
                                                            details: {}
                                                        }));
                                                    }
                                                    return {
                                                        type: 'Directory',
                                                        details: {}
                                                    };
                                                }}>
                                                {(details: models.RepoAppDetails) => {
                                                    const pathKey = (liveSrc?.chart || liveSrc?.path || '') as string;
                                                    const type = (explicitPathType && explicitPathType.path === pathKey && explicitPathType.type) || details.type;
                                                    let d = details;
                                                    if (d.type !== type) {
                                                        switch (type) {
                                                            case 'Helm':
                                                                d = {
                                                                    type,
                                                                    path: d.path,
                                                                    helm: {name: '', valueFiles: [], path: '', parameters: [], fileParameters: []}
                                                                };
                                                                break;
                                                            case 'Kustomize':
                                                                d = {type, path: d.path, kustomize: {path: ''}};
                                                                break;
                                                            case 'Plugin':
                                                                d = {type, path: d.path, plugin: {name: '', env: []}};
                                                                break;
                                                            default:
                                                                d = {type, path: d.path, directory: {}};
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
                                                                qeId='application-create-dropdown-source'
                                                                items={APP_SOURCE_TYPES.map(item => ({
                                                                    title: item.type,
                                                                    action: () => {
                                                                        setExplicitPathType({type: item.type, path: pathKey});
                                                                        normalizeTypeFieldsForSource(api, item.type, undefined);
                                                                    }
                                                                }))}
                                                            />
                                                            <ApplicationParameters
                                                                noReadonlyMode={true}
                                                                application={liveApp}
                                                                details={d}
                                                                save={async updatedApp => {
                                                                    api.setAllValues(updatedApp);
                                                                }}
                                                            />
                                                        </React.Fragment>
                                                    );
                                                }}
                                            </DataLoader>
                                        );
                                    };

                                    return (
                                        <form onSubmit={api.submitForm} role='form' className='width-control'>
                                            {generalPanel()}

                                            {sourcePanel()}

                                            {destinationPanel()}

                                            {!multiSourceMode && typePanel()}
                                        </form>
                                    );
                                }}
                            </Form>
                        )}
                    </div>
                );
            }}
        </DataLoader>
    );
};
