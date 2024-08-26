import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import {Form, FormApi, FormErrors, Text} from 'react-form';
import {ApplicationParameters} from '../../../applications/components/application-parameters/application-parameters';
import {RevisionFormField} from '../../../applications/components/revision-form-field/revision-form-field';
import {RevisionHelpIcon} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import './source-panel.scss';

// This is similar to what is in application-create-panel.tsx. If the create panel
// is modified to support multi-source apps, then we should refactor and common these up
const appTypes = new Array<{field: string; type: models.AppSourceType}>(
    {type: 'Helm', field: 'helm'},
    {type: 'Kustomize', field: 'kustomize'},
    {type: 'Directory', field: 'directory'},
    {type: 'Plugin', field: 'plugin'}
);

// This is similar to the same function in application-create-panel.tsx. If the create panel
// is modified to support multi-source apps, then we should refactor and common these up
function normalizeAppSource(app: models.Application, type: string): boolean {
    const source = app.spec.source;
    // eslint-disable-next-line no-prototype-builtins
    const repoType = (source.hasOwnProperty('chart') && 'helm') || 'git';
    if (repoType !== type) {
        if (type === 'git') {
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

// Use a single source app to represent the 'new source'. This panel will make use of the source field only.
// However, we need to use a template based on an Application so that we can reuse the application-parameters code
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
        source: {
            path: '',
            repoURL: '',
            ref: '',
            targetRevision: 'HEAD'
        },
        sources: [],
        project: ''
    }
};

export const SourcePanel = (props: {
    appCurrent: models.Application;
    onSubmitFailure: (error: string) => any;
    updateApp: (app: models.Application) => any;
    getFormApi: (api: FormApi) => any;
}) => {
    const [explicitPathType, setExplicitPathType] = React.useState<{path: string; type: models.AppSourceType}>(null);
    const appInEdit = deepMerge(DEFAULT_APP, {});

    function normalizeTypeFields(formApi: FormApi, type: models.AppSourceType) {
        const appToNormalize = formApi.getFormState().values;
        for (const item of appTypes) {
            if (item.type !== type) {
                delete appToNormalize.spec.source[item.field];
            }
        }
        formApi.setAllValues(appToNormalize);
    }

    return (
        <React.Fragment>
            <DataLoader key='add-new-source' load={() => Promise.all([services.repos.list()]).then(([reposInfo]) => ({reposInfo}))}>
                {({reposInfo}) => {
                    const repos = reposInfo.map(info => info.repo).sort();
                    return (
                        <div className='new-source-panel'>
                            <Form
                                validateError={(a: models.Application) => {
                                    let samePath = false;
                                    let sameChartVersion = false;
                                    let pathError = null;
                                    let chartError = null;
                                    if (a.spec.source.repoURL && a.spec.source.path) {
                                        props.appCurrent.spec.sources.forEach(source => {
                                            if (source.repoURL === a.spec.source.repoURL && source.path === a.spec.source.path) {
                                                samePath = true;
                                                pathError = 'Provided path in the selected repository URL was already added to this multi-source application';
                                            }
                                        });
                                    }
                                    if (a.spec.source.repoURL && a.spec.source.chart) {
                                        props.appCurrent.spec.sources.forEach(source => {
                                            if (
                                                source.repoURL === a.spec.source.repoURL &&
                                                source.chart === a.spec.source.chart &&
                                                source.targetRevision === a.spec.source.targetRevision
                                            ) {
                                                sameChartVersion = true;
                                                chartError =
                                                    'Version ' +
                                                    source.targetRevision +
                                                    ' of chart ' +
                                                    source.chart +
                                                    ' from the selected repository was already added to this multi-source application';
                                            }
                                        });
                                    }
                                    if (!samePath) {
                                        if (!a.spec.source.path && !a.spec.source.chart && !a.spec.source.ref) {
                                            pathError = 'Path or Ref is required';
                                        }
                                    }
                                    if (!sameChartVersion) {
                                        if (!a.spec.source.chart && !a.spec.source.path && !a.spec.source.ref) {
                                            chartError = 'Chart is required';
                                        }
                                    }
                                    return {
                                        'spec.source.repoURL': !a.spec.source.repoURL && 'Repository URL is required',
                                        // eslint-disable-next-line no-prototype-builtins
                                        'spec.source.targetRevision': !a.spec.source.targetRevision && a.spec.source.hasOwnProperty('chart') && 'Version is required',
                                        'spec.source.path': pathError,
                                        'spec.source.chart': chartError
                                    };
                                }}
                                defaultValues={appInEdit}
                                onSubmitFailure={(errors: FormErrors) => {
                                    let errorString: string = '';
                                    let i = 0;
                                    for (const key in errors) {
                                        if (errors[key]) {
                                            i++;
                                            errorString = errorString.concat(i + '. ' + errors[key] + ' ');
                                        }
                                    }
                                    props.onSubmitFailure(errorString);
                                }}
                                onSubmit={values => {
                                    props.updateApp(values as models.Application);
                                }}
                                getApi={props.getFormApi}>
                                {api => {
                                    // eslint-disable-next-line no-prototype-builtins
                                    const repoType = (api.getFormState().values.spec.source.hasOwnProperty('chart') && 'helm') || 'git';
                                    const repoInfo = reposInfo.find(info => info.repo === api.getFormState().values.spec.source.repoURL);
                                    if (repoInfo) {
                                        normalizeAppSource(appInEdit, repoInfo.type || 'git');
                                    }
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
                                                    <RevisionFormField formApi={api} helpIconTop={'2.5em'} repoURL={api.getFormState().values.spec.source.repoURL} />
                                                    <div className='argo-form-row'>
                                                        <DataLoader
                                                            input={{
                                                                repoURL: api.getFormState().values.spec.source.repoURL,
                                                                revision: api.getFormState().values.spec.source.targetRevision
                                                            }}
                                                            load={async src =>
                                                                (src.repoURL &&
                                                                    (await services.repos
                                                                        .apps(src.repoURL, src.revision, appInEdit.metadata.name, props.appCurrent.spec.project)
                                                                        .then(apps => Array.from(new Set(apps.map(item => item.path))).sort())
                                                                        .catch(() => new Array<string>()))) ||
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
                                                    <div className='argo-form-row'>
                                                        <FormField formApi={api} label='Ref' field={'spec.source.ref'} component={Text}></FormField>
                                                    </div>
                                                </React.Fragment>
                                            )) || (
                                                <DataLoader
                                                    input={{repoURL: api.getFormState().values.spec.source.repoURL}}
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

                                    const typePanel = () => (
                                        <DataLoader
                                            input={{
                                                repoURL: appInEdit.spec.source.repoURL,
                                                path: appInEdit.spec.source.path,
                                                chart: appInEdit.spec.source.chart,
                                                targetRevision: appInEdit.spec.source.targetRevision,
                                                appName: appInEdit.metadata.name
                                            }}
                                            load={async src => {
                                                if (src.repoURL && src.targetRevision && (src.path || src.chart)) {
                                                    return services.repos.appDetails(src, src.appName, props.appCurrent.spec.project, 0, 0).catch(() => ({
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
                                                const type = (explicitPathType && explicitPathType.path === appInEdit.spec.source.path && explicitPathType.type) || details.type;
                                                if (details.type !== type) {
                                                    switch (type) {
                                                        case 'Helm':
                                                            details = {
                                                                type,
                                                                path: details.path,
                                                                helm: {name: '', valueFiles: [], path: '', parameters: [], fileParameters: []}
                                                            };
                                                            break;
                                                        case 'Kustomize':
                                                            details = {type, path: details.path, kustomize: {path: ''}};
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
                                                                    setExplicitPathType({type: item.type, path: appInEdit.spec.source.path});
                                                                    normalizeTypeFields(api, item.type);
                                                                }
                                                            }))}
                                                        />
                                                        <ApplicationParameters
                                                            noReadonlyMode={true}
                                                            application={api.getFormState().values as models.Application}
                                                            details={details}
                                                            tempSource={appInEdit.spec.source}
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
                                            {sourcePanel()}

                                            {typePanel()}
                                        </form>
                                    );
                                }}
                            </Form>
                        </div>
                    );
                }}
            </DataLoader>
        </React.Fragment>
    );
};
