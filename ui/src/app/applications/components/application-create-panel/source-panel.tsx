import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'react-form';
import {RevisionHelpIcon} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import {getAppDefaultSource} from '../utils';

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

export interface SourcePanelProps {
    formApi: FormApi;
    repos: string[];
    repoInfo?: models.Repository;
    sourceIndex?: number;
    suppressMultiSourceHeading?: boolean;
    currentRepoType?: React.MutableRefObject<string | undefined>;
    lastGitOrHelmUrl?: React.MutableRefObject<string>;
    lastOciUrl?: React.MutableRefObject<string>;
}

export const SourcePanel = (props: SourcePanelProps) => {
    const internalRepoType = React.useRef<string | undefined>(undefined);
    const internalLastGit = React.useRef('');
    const internalLastOci = React.useRef('');
    const isMulti = props.sourceIndex !== undefined;
    const lastGitOrHelmUrl = isMulti ? internalLastGit : props.lastGitOrHelmUrl;
    const lastOciUrl = isMulti ? internalLastOci : props.lastOciUrl;
    const currentRepoType = isMulti ? internalRepoType : props.currentRepoType;

    const currentApp = props.formApi.getFormState().values as models.Application;
    const currentSource = getSourceForPanel(currentApp, props.sourceIndex);
    const repoType = currentSource?.repoURL?.startsWith('oci://') ? 'oci' : (currentSource && Object.prototype.hasOwnProperty.call(currentSource, 'chart') && 'helm') || 'git';

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
                                            if (repoType === 'git' || repoType === 'helm') {
                                                lastGitOrHelmUrl.current = source.repoURL;
                                            } else {
                                                lastOciUrl.current = source.repoURL;
                                            }
                                            currentRepoType.current = type;
                                            switch (type) {
                                                case 'git':
                                                case 'oci':
                                                    if (Object.prototype.hasOwnProperty.call(source, 'chart')) {
                                                        source.path = source.chart;
                                                        delete source.chart;
                                                    }
                                                    source.targetRevision = 'HEAD';
                                                    source.repoURL = type === 'git' ? lastGitOrHelmUrl.current : lastOciUrl.current === '' ? 'oci://' : lastOciUrl.current;
                                                    break;
                                                case 'helm':
                                                    if (Object.prototype.hasOwnProperty.call(source, 'path')) {
                                                        source.chart = source.path;
                                                        delete source.path;
                                                    }
                                                    source.targetRevision = '';
                                                    source.repoURL = lastGitOrHelmUrl.current;
                                                    break;
                                            }
                                            props.formApi.setAllValues(updatedApp);
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
        </React.Fragment>
    );
};
