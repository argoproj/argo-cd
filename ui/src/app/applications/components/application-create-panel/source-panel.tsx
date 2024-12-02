import {AutocompleteField, DataLoader, DropDownMenu} from 'argo-ui';
import {FormField} from 'argo-ui';
import * as deepMerge from 'deepmerge';
import * as React from 'react';
import * as ReactForm from 'react-form';
import {FormApi, Text} from 'react-form';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import {RevisionHelpIcon} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
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
function normalizeAppSource(index: number, app: models.Application, repoType: string, type: string): boolean {
    const source = app.spec.sources[index];
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
            name: '',
            targetRevision: 'HEAD'
        },
        sources: [],
        project: ''
    }
};

export const SourcePanel = (props: {
    index: number;
    api: ReactForm.FormApi;
    reposInfo: models.Repository[];
    appCurrent: models.Application;
    onSubmitFailure?: (error: string) => any;
    updateApp?: (app: models.Application) => any;
    getFormApi?: (api: FormApi) => any;
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

    const repos = props.reposInfo.map(info => info.repo).sort();
    const repoType = 'git';

    return (
        <div className='white-box'>
            <p>SOURCE {props.index + 1}</p>
            <div className='row argo-form-row'>
                <div className='columns small-10'>
                    <FormField
                        formApi={props.api}
                        qeId={'application-create-source' + props.index + '-field-repository-url'}
                        label='Repository URL'
                        field={'spec.sources[' + props.index + '].repoURL'}
                        component={AutocompleteField}
                        componentProps={{items: repos}}
                    />
                </div>
                <div className='columns small-2'>
                    <div style={{paddingTop: '1.5em'}}>
                        {
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
                                            const updatedApp = props.api.getFormState().values as models.Application;
                                            if (normalizeAppSource(props.index, updatedApp, repoType, type)) {
                                                props.api.setAllValues(updatedApp);
                                            }
                                        }
                                    }
                                }))}
                            />
                        }
                    </div>
                </div>
            </div>
            <div className='row argo-form-row'>
                <div className='columns small-10'>
                    <FormField formApi={props.api} label='Name' field={'spec.sources[' + props.index + '].name'} component={Text}></FormField>
                </div>
            </div>
            {(repoType === 'git' && (
                <React.Fragment>
                    <RevisionFormField formApi={props.api} helpIconTop={'2.5em'} repoURL={props.api.getFormState().values.spec?.source?.repoURL} />
                    <div className='argo-form-row'>
                        <DataLoader
                            input={{
                                repoURL: props.api.getFormState().values.spec?.source?.repoURL,
                                revision: props.api.getFormState().values.spec?.source?.targetRevision
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
                                    formApi={props.api}
                                    label='Path'
                                    qeId={'application-create-source' + props.index + '-field-path'}
                                    field={'spec.sources[' + props.index + '].path'}
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
                        <FormField formApi={props.api} label='Ref' field={'spec.sources[' + props.index + '].ref'} component={Text}></FormField>
                    </div>
                </React.Fragment>
            )) || (
                <DataLoader
                    input={{repoURL: props.api.getFormState().values.spec.sources[props.index].repoURL}}
                    load={async src => (src.repoURL && services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())) || new Array<models.HelmChart>()}>
                    {(charts: models.HelmChart[]) => {
                        const selectedChart = charts.find(chart => chart.name === props.api.getFormState().values.spec?.source?.chart);
                        return (
                            <div className='row argo-form-row'>
                                <div className='columns small-10'>
                                    <FormField
                                        formApi={props.api}
                                        label='Chart'
                                        field={'spec.sources[' + props.index + '].chart'}
                                        component={AutocompleteField}
                                        componentProps={{
                                            items: charts.map(chart => chart.name),
                                            filterSuggestions: true
                                        }}
                                    />
                                </div>
                                <div className='columns small-2'>
                                    <FormField
                                        formApi={props.api}
                                        field={'spec.sources[' + props.index + '].targetRevision'}
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
            <DataLoader
                input={{
                    repoURL: appInEdit.spec?.source?.repoURL,
                    path: appInEdit.spec?.source?.path,
                    chart: appInEdit.spec?.source?.chart,
                    targetRevision: appInEdit.spec?.source?.targetRevision,
                    appName: appInEdit.metadata.name
                }}
                load={async src => {
                    if (src?.repoURL && src?.targetRevision && (src?.path || src?.chart)) {
                        return services.repos.appDetails(src, src?.appName, props.appCurrent.spec?.project, 0, 0).catch(() => ({
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
                    const type = (explicitPathType && explicitPathType.path === appInEdit.spec?.source?.path && explicitPathType.type) || details.type;
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
                                        setExplicitPathType({type: item.type, path: appInEdit.spec?.source?.path});
                                        normalizeTypeFields(props.api, item.type);
                                    }
                                }))}
                            />
                            <ApplicationParameters
                                noReadonlyMode={true}
                                application={props.api.getFormState().values as models.Application}
                                details={details}
                                tempSource={appInEdit.spec.source}
                                save={async updatedApp => {
                                    props.api.setAllValues(updatedApp);
                                }}
                            />
                        </React.Fragment>
                    );
                }}
            </DataLoader>
        </div>
    );
};
