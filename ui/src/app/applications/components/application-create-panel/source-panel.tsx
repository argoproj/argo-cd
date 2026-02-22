import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'react-form';
import {RevisionHelpIcon} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import {getAppDefaultSource} from '../utils';

interface SourcePanelProps {
    formApi: FormApi;
    repos: string[];
    repoInfo?: models.Repository;
    currentRepoType: React.MutableRefObject<string>;
    lastGitOrHelmUrl: React.MutableRefObject<string>;
    lastOciUrl: React.MutableRefObject<string>;
}

export const SourcePanel = (props: SourcePanelProps) => {
    const currentApp = props.formApi.getFormState().values as models.Application;
    const currentSource = getAppDefaultSource(currentApp);
    const repoType = currentSource?.repoURL?.startsWith('oci://') ? 'oci' : (currentSource && Object.prototype.hasOwnProperty.call(currentSource, 'chart') && 'helm') || 'git';

    return (
        <React.Fragment>
            <div style={{display: 'flex', alignItems: 'flex-start'}}>
                <div style={{flex: '1 1 auto', minWidth: 0}}>
                    <FormField
                        formApi={props.formApi}
                        label='Repository URL'
                        qeId='application-create-field-repository-url'
                        field='spec.source.repoURL'
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
                                qeId='application-create-dropdown-source-repository'
                                items={['git', 'helm', 'oci'].map((type: 'git' | 'helm' | 'oci') => ({
                                    title: type.toUpperCase(),
                                    action: () => {
                                        if (repoType !== type) {
                                            const updatedApp = props.formApi.getFormState().values as models.Application;
                                            const source = getAppDefaultSource(updatedApp);
                                            // Save the previous URL value for later use
                                            if (repoType === 'git' || repoType === 'helm') {
                                                props.lastGitOrHelmUrl.current = source.repoURL;
                                            } else {
                                                props.lastOciUrl.current = source.repoURL;
                                            }
                                            props.currentRepoType.current = type;
                                            switch (type) {
                                                case 'git':
                                                case 'oci':
                                                    if (Object.prototype.hasOwnProperty.call(source, 'chart')) {
                                                        source.path = source.chart;
                                                        delete source.chart;
                                                    }
                                                    source.targetRevision = 'HEAD';
                                                    source.repoURL =
                                                        type === 'git' ? props.lastGitOrHelmUrl.current : props.lastOciUrl.current === '' ? 'oci://' : props.lastOciUrl.current;
                                                    break;
                                                case 'helm':
                                                    if (Object.prototype.hasOwnProperty.call(source, 'path')) {
                                                        source.chart = source.path;
                                                        delete source.path;
                                                    }
                                                    source.targetRevision = '';
                                                    source.repoURL = props.lastGitOrHelmUrl.current;
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
                    <RevisionFormField formApi={props.formApi} helpIconTop={'2.5em'} repoURL={currentApp.spec.source.repoURL} repoType={repoType} />
                    <div className='argo-form-row'>
                        <DataLoader
                            input={{repoURL: currentApp.spec.source.repoURL, revision: currentApp.spec.source.targetRevision}}
                            load={async src =>
                                src.repoURL &&
                                // TODO: for autocomplete we need to fetch paths that are used by other apps within the same project making use of the same OCI repo
                                new Array<string>()
                            }>
                            {(paths: string[]) => (
                                <FormField
                                    formApi={props.formApi}
                                    label='Path'
                                    qeId='application-create-field-path'
                                    field='spec.source.path'
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
                        <RevisionFormField formApi={props.formApi} helpIconTop={'2.5em'} repoURL={currentApp.spec.source.repoURL} repoType={repoType} />
                        <div className='argo-form-row'>
                            <DataLoader
                                input={{repoURL: currentApp.spec.source.repoURL, revision: currentApp.spec.source.targetRevision}}
                                load={async src =>
                                    (src.repoURL &&
                                        services.repos
                                            .apps(src.repoURL, src.revision, currentApp.metadata.name, currentApp.spec.project)
                                            .then(apps => Array.from(new Set(apps.map(item => item.path))).sort())
                                            .catch(() => new Array<string>())) ||
                                    new Array<string>()
                                }>
                                {(apps: string[]) => (
                                    <FormField
                                        formApi={props.formApi}
                                        label='Path'
                                        qeId='application-create-field-path'
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
                        input={{repoURL: currentApp.spec.source.repoURL}}
                        load={async src => (src.repoURL && services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())) || new Array<models.HelmChart>()}>
                        {(charts: models.HelmChart[]) => {
                            const selectedChart = charts.find(chart => chart.name === props.formApi.getFormState().values.spec.source.chart);
                            return (
                                <div className='row argo-form-row'>
                                    <div className='columns small-10'>
                                        <FormField
                                            formApi={props.formApi}
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
                                            formApi={props.formApi}
                                            field='spec.source.targetRevision'
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
