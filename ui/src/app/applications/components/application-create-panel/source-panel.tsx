import {AutocompleteField, DataLoader, DropDownMenu, FormField, Text} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import {getAppDefaultSource} from '../utils';
import {inferSourceRepositoryType, isRefOnlySource, normalizeRefOnlySource, normalizeSourceForRepositoryType, SourceRepositoryType} from '../shared/app-source-edit';

function getSourceForPanel(app: models.Application, sourceIndex?: number): models.ApplicationSource | null {
    if (sourceIndex !== undefined) {
        return app.spec.sources?.[sourceIndex] ?? null;
    }
    return getAppDefaultSource(app);
}

function fieldPath(sourceIndex: number | undefined, field: string): string {
    if (sourceIndex !== undefined) {
        return `spec.sources[${sourceIndex}].${field}`;
    }
    return `spec.source.${field}`;
}

function registeredRepositoryType(repoInfo: models.Repository | undefined, source: models.ApplicationSource | null): SourceRepositoryType {
    if (!repoInfo) {
        return inferSourceRepositoryType(source ?? undefined);
    }
    const type = repoInfo.type?.toLowerCase();
    return type === 'helm' || type === 'oci' ? type : 'git';
}

function updateSource(formApi: FormApi, sourceIndex: number | undefined, transform: (source: models.ApplicationSource) => models.ApplicationSource): void {
    const app = formApi.getFormState().values as models.Application;
    const source = getSourceForPanel(app, sourceIndex);
    if (!source) {
        return;
    }
    const updatedSource = transform(source);
    if (updatedSource === source) {
        return;
    }

    if (sourceIndex === undefined) {
        formApi.setAllValues({...app, spec: {...app.spec, source: updatedSource}});
        return;
    }

    const sources = [...(app.spec.sources || [])];
    if (!sources[sourceIndex]) {
        return;
    }
    sources[sourceIndex] = updatedSource;
    formApi.setAllValues({...app, spec: {...app.spec, sources}});
}

export interface SourcePanelProps {
    formApi: FormApi;
    repos: string[];
    repoInfo?: models.Repository;
    sourceIndex?: number;
    suppressMultiSourceHeading?: boolean;
    lastGitOrHelmUrl?: React.MutableRefObject<string>;
    lastOciUrl?: React.MutableRefObject<string>;
}

export const SourcePanel = (props: SourcePanelProps) => {
    const internalLastGit = React.useRef('');
    const internalLastOci = React.useRef('');
    const isMulti = props.sourceIndex !== undefined;
    const lastGitOrHelmUrl = isMulti ? internalLastGit : props.lastGitOrHelmUrl || internalLastGit;
    const lastOciUrl = isMulti ? internalLastOci : props.lastOciUrl || internalLastOci;

    const currentApp = props.formApi.getFormState().values as models.Application;
    const currentSource = getSourceForPanel(currentApp, props.sourceIndex);
    const repoType = registeredRepositoryType(props.repoInfo, currentSource);
    const currentRepoURL = currentSource?.repoURL;
    const refOnly = isRefOnlySource(currentSource ?? undefined);

    React.useEffect(() => {
        updateSource(props.formApi, props.sourceIndex, source => normalizeSourceForRepositoryType(normalizeRefOnlySource(source), repoType));
    }, [props.formApi, props.sourceIndex, repoType, currentRepoURL, refOnly]);

    const idx = props.sourceIndex;
    const qeSourceN = isMulti && idx !== undefined ? idx + 1 : 0;
    const specSourceForRevision = isMulti ? currentApp.spec.sources?.[props.sourceIndex] : currentApp.spec.source;

    return (
        <React.Fragment>
            {isMulti && !props.suppressMultiSourceHeading && (
                <p className='application-create-panel__multi-source-title' style={{marginTop: idx > 0 ? '1em' : 0}}>
                    SOURCE {idx + 1}
                </p>
            )}
            <div style={{display: 'flex', alignItems: 'flex-start'}}>
                <div style={{flex: '1 1 auto', minWidth: 0}}>
                    <FormField
                        formApi={props.formApi}
                        label='Repository URL'
                        qeId={isMulti ? `application-create-source-${qeSourceN}-field-repository-url` : 'application-create-field-repository-url'}
                        field={fieldPath(idx, 'repoURL')}
                        component={AutocompleteField}
                        componentProps={{
                            items: props.repos,
                            filterSuggestions: true
                        }}
                    />
                </div>
                <div style={{flex: '0 0 auto', minWidth: '7rem'}}>
                    <div style={{paddingTop: '1.5em'}}>
                        {(props.repoInfo && (
                            <React.Fragment>
                                <span>{(props.repoInfo.type || 'git').toUpperCase()}</span> <i className='fa fa-check' />
                            </React.Fragment>
                        )) || (
                            <DropDownMenu
                                anchor={() => (
                                    <p>
                                        {repoType.toUpperCase()} <i className='fa fa-caret-down' />
                                    </p>
                                )}
                                qeId={isMulti ? `application-create-dropdown-source-repository-${qeSourceN}` : 'application-create-dropdown-source-repository'}
                                items={['git', 'helm', 'oci'].map((type: 'git' | 'helm' | 'oci') => ({
                                    title: type.toUpperCase(),
                                    action: () => {
                                        if (repoType !== type) {
                                            const updatedApp = props.formApi.getFormState().values as models.Application;
                                            const source = getSourceForPanel(updatedApp, props.sourceIndex);
                                            if (!source) {
                                                return;
                                            }
                                            // These refs are written from a DropDownMenu click handler, which runs on
                                            // user interaction rather than during render, so ref access here is safe.
                                            /* eslint-disable react-hooks/refs */
                                            if (repoType === 'git' || repoType === 'helm') {
                                                lastGitOrHelmUrl.current = source.repoURL;
                                            } else {
                                                lastOciUrl.current = source.repoURL;
                                            }
                                            /* eslint-enable react-hooks/refs */
                                            const targetRepoURL = type === 'oci' ? (lastOciUrl.current === '' ? 'oci://' : lastOciUrl.current) : lastGitOrHelmUrl.current;
                                            updateSource(props.formApi, props.sourceIndex, current => ({
                                                ...normalizeSourceForRepositoryType(current, type),
                                                repoURL: targetRepoURL,
                                                targetRevision: type === 'helm' ? '' : 'HEAD'
                                            }));
                                        }
                                    }
                                }))}
                            />
                        )}
                    </div>
                </div>
            </div>

            {(repoType === 'oci' && (
                <React.Fragment>
                    <RevisionFormField
                        formApi={props.formApi}
                        helpIconTop={'2.5em'}
                        repoURL={specSourceForRevision?.repoURL || ''}
                        repoType={repoType}
                        fieldValue={fieldPath(idx, 'targetRevision')}
                    />
                    <div className='argo-form-row'>
                        <DataLoader
                            input={{repoURL: specSourceForRevision?.repoURL, revision: specSourceForRevision?.targetRevision}}
                            load={async src =>
                                src.repoURL &&
                                // TODO: for autocomplete we need to fetch paths that are used by other apps within the same project making use of the same OCI repo
                                new Array<string>()
                            }>
                            {(paths: string[]) => (
                                <FormField
                                    formApi={props.formApi}
                                    label='Path'
                                    qeId={isMulti ? `application-create-source-${qeSourceN}-field-path` : 'application-create-field-path'}
                                    field={fieldPath(idx, 'path')}
                                    component={AutocompleteField}
                                    componentProps={{
                                        items: paths,
                                        filterSuggestions: true
                                    }}
                                />
                            )}
                        </DataLoader>
                    </div>
                </React.Fragment>
            )) ||
                (repoType === 'git' && (
                    <React.Fragment>
                        <RevisionFormField
                            formApi={props.formApi}
                            helpIconTop={'2.5em'}
                            repoURL={specSourceForRevision?.repoURL || ''}
                            repoType={repoType}
                            fieldValue={fieldPath(idx, 'targetRevision')}
                        />
                        <div className='argo-form-row'>
                            <DataLoader
                                input={{repoURL: specSourceForRevision?.repoURL, revision: specSourceForRevision?.targetRevision}}
                                load={async src =>
                                    (src.repoURL &&
                                        services.repos
                                            .apps(src.repoURL, src.revision, currentApp.metadata.name, currentApp.spec.project)
                                            .then(apps => Array.from(new Set(apps.map(item => item.path))).sort((a, b) => a.localeCompare(b)))
                                            .catch(() => new Array<string>())) ||
                                    new Array<string>()
                                }>
                                {(apps: string[]) => (
                                    <FormField
                                        formApi={props.formApi}
                                        label='Path'
                                        qeId={isMulti ? `application-create-source-${qeSourceN}-field-path` : 'application-create-field-path'}
                                        field={fieldPath(idx, 'path')}
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
                        input={{repoURL: specSourceForRevision?.repoURL}}
                        load={async src => (src.repoURL && services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())) || new Array<models.HelmChart>()}>
                        {(charts: models.HelmChart[]) => {
                            const spec = props.formApi.getFormState().values.spec;
                            const chartName = isMulti ? spec.sources?.[props.sourceIndex as number]?.chart : spec.source?.chart;
                            const selectedChart = charts.find(chart => chart.name === chartName);
                            return (
                                <div className='row argo-form-row'>
                                    <div className='columns small-10'>
                                        <FormField
                                            formApi={props.formApi}
                                            label='Chart'
                                            field={fieldPath(idx, 'chart')}
                                            component={AutocompleteField}
                                            componentProps={{
                                                items: charts.map(chart => chart.name),
                                                filterSuggestions: true
                                            }}
                                        />
                                    </div>
                                    <div className='columns small-2'>
                                        <FormField
                                            formApi={props.formApi}
                                            field={fieldPath(idx, 'targetRevision')}
                                            component={AutocompleteField}
                                            componentProps={{
                                                items: (selectedChart && selectedChart.versions) || [],
                                                filterSuggestions: true
                                            }}
                                        />
                                        <RevisionHelpIcon type='helm' />
                                    </div>
                                </div>
                            );
                        }}
                    </DataLoader>
                )}
            {isMulti && repoType === 'git' && (
                <div className='argo-form-row'>
                    <FormField formApi={props.formApi} label='Ref' qeId={`application-create-source-${qeSourceN}-field-ref`} field={fieldPath(idx, 'ref')} component={Text} />
                </div>
            )}
        </React.Fragment>
    );
};
