import {AutocompleteField, DataLoader, ErrorNotification, FormField, FormSelect, getNestedField, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FieldApi, FormApi, FormField as ReactFormField, Text, TextArea} from 'react-form';
import {cloneDeep} from 'lodash-es';
import {
    ArrayInputField,
    ArrayValueField,
    CheckboxField,
    Expandable,
    MapValueField,
    NameValueEditor,
    StringValueField,
    NameValue,
    TagsInputField,
    ValueEditor,
    Paginate,
    RevisionHelpIcon,
    Revision,
    Repo,
    EditablePanel,
    EditablePanelItem,
    Spinner
} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ApplicationSourceDirectory, Plugin} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ImageTagFieldEditor} from './kustomize';
import * as kustomize from './kustomize-image';
import {VarsInputField} from './vars-input-field';
import {concatMaps} from '../../../shared/utils';
import {deleteSourceAction, getAppDefaultSource, helpTip} from '../utils';
import * as jsYaml from 'js-yaml';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import classNames from 'classnames';
import {ApplicationParametersSource} from './application-parameters-source';

import './application-parameters.scss';
import {AppContext} from '../../../shared/context';
import {SourcePanel} from './source-panel';

const TextWithMetadataField = ReactFormField((props: {metadata: {value: string}; fieldApi: FieldApi; className: string}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const metadata = getValue() || props.metadata;

    return <input className={props.className} value={metadata.value} onChange={el => setValue({...metadata, value: el.target.value})} />;
});

function distinct<T>(first: IterableIterator<T>, second: IterableIterator<T>) {
    return Array.from(new Set(Array.from(first).concat(Array.from(second))));
}

function overridesFirst(first: {overrideIndex: number; metadata: {name: string}}, second: {overrideIndex: number; metadata: {name: string}}) {
    if (first.overrideIndex === second.overrideIndex) {
        return first.metadata.name.localeCompare(second.metadata.name);
    }
    if (first.overrideIndex < 0) {
        return 1;
    } else if (second.overrideIndex < 0) {
        return -1;
    }
    return first.overrideIndex - second.overrideIndex;
}

function processPath(path: string) {
    if (path !== null && path !== undefined) {
        if (path === '.') {
            return '(root)';
        }
        return path;
    }
    return '';
}

function getParamsEditableItems(
    app: models.Application,
    title: string,
    fieldsPath: string,
    removedOverrides: boolean[],
    setRemovedOverrides: React.Dispatch<boolean[]>,
    params: {
        key?: string;
        overrideIndex: number;
        original: string;
        metadata: {name: string; value: string};
    }[],
    component: React.ComponentType = TextWithMetadataField
) {
    return params
        .sort(overridesFirst)
        .map((param, i) => ({
            key: param.key,
            title: param.metadata.name,
            view: (
                <span title={param.metadata.value}>
                    {param.overrideIndex > -1 && <span className='fa fa-gavel' title={`Original value: ${param.original}`} />} {param.metadata.value}
                </span>
            ),
            edit: (formApi: FormApi) => {
                const labelStyle = {position: 'absolute', right: 0, top: 0, zIndex: 11} as any;
                const overrideRemoved = removedOverrides[i];
                const fieldItemPath = `${fieldsPath}[${i}]`;
                return (
                    <React.Fragment>
                        {(overrideRemoved && <span>{param.original}</span>) || (
                            <FormField
                                formApi={formApi}
                                field={fieldItemPath}
                                component={component}
                                componentProps={{
                                    metadata: param.metadata
                                }}
                            />
                        )}
                        {param.metadata.value !== param.original && !overrideRemoved && (
                            <a
                                onClick={() => {
                                    formApi.setValue(fieldItemPath, null);
                                    removedOverrides[i] = true;
                                    setRemovedOverrides(removedOverrides);
                                }}
                                style={labelStyle}>
                                Remove override
                            </a>
                        )}
                        {overrideRemoved && (
                            <a
                                onClick={() => {
                                    formApi.setValue(fieldItemPath, getNestedField(app, fieldsPath)[i]);
                                    removedOverrides[i] = false;
                                    setRemovedOverrides(removedOverrides);
                                }}
                                style={labelStyle}>
                                Keep override
                            </a>
                        )}
                    </React.Fragment>
                );
            }
        }))
        .map((item, i) => ({...item, before: (i === 0 && <p style={{marginTop: '1em'}}>{title}</p>) || null}));
}

export const ApplicationParameters = (props: {
    application: models.Application;
    details?: models.RepoAppDetails;
    save?: (application: models.Application, query: {validate?: boolean}) => Promise<any>;
    noReadonlyMode?: boolean;
    pageNumber?: number;
    setPageNumber?: (x: number) => any;
    collapsedSources?: boolean[];
    handleCollapse?: (i: number, isCollapsed: boolean) => void;
    appContext?: AppContext;
    tempSource?: models.ApplicationSource;
}) => {
    const app = cloneDeep(props.application);
    const source = getAppDefaultSource(app); // For source field
    const appSources = app?.spec.sources;
    const [removedOverrides, setRemovedOverrides] = React.useState(new Array<boolean>());
    const collapsible = props.collapsedSources !== undefined && props.handleCollapse !== undefined;
    const [createApi, setCreateApi] = React.useState(null);
    const [isAddingSource, setIsAddingSource] = React.useState(false);
    const [isSavingSource, setIsSavingSource] = React.useState(false);
    const [appParamsDeletedState, setAppParamsDeletedState] = React.useState([]);

    if (app.spec.sources?.length > 0 && !props.details) {
        // For multi-source case only
        return (
            <div className='application-parameters'>
                <div className='source-panel-buttons'>
                    <button key={'add_source_button'} onClick={() => setIsAddingSource(true)} disabled={false} className='argo-button argo-button--base'>
                        {helpTip('Add a new source and append it to the sources field')}
                        <span style={{marginRight: '8px'}} />
                        Add Source
                    </button>
                </div>
                <Paginate
                    showHeader={false}
                    data={app.spec.sources}
                    page={props.pageNumber}
                    preferencesKey={'5'}
                    onPageChange={page => {
                        props.setPageNumber(page);
                    }}>
                    {data => {
                        const listOfPanels: JSX.Element[] = [];
                        data.forEach(appSource => {
                            const i = app.spec.sources.indexOf(appSource);
                            listOfPanels.push(getEditablePanelForSources(i, appSource));
                        });
                        return listOfPanels;
                    }}
                </Paginate>
                <SlidingPanel
                    isShown={isAddingSource}
                    onClose={() => setIsAddingSource(false)}
                    header={
                        <div>
                            <button
                                key={'source_panel_save_button'}
                                className='argo-button argo-button--base'
                                disabled={isSavingSource}
                                onClick={() => createApi && createApi.submitForm(null)}>
                                <Spinner show={isSavingSource} style={{marginRight: '5px'}} />
                                Save
                            </button>{' '}
                            <button
                                key={'source_panel_cancel_button_'}
                                onClick={() => {
                                    setIsAddingSource(false);
                                    setIsSavingSource(false);
                                }}
                                className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <SourcePanel
                        appCurrent={props.application}
                        getFormApi={api => {
                            setCreateApi(api);
                        }}
                        onSubmitFailure={errors => {
                            props.appContext.apis.notifications.show({
                                content: 'Cannot add source: ' + errors.toString(),
                                type: NotificationType.Warning
                            });
                        }}
                        updateApp={async updatedAppSource => {
                            setIsSavingSource(true);
                            props.application.spec.sources.push(updatedAppSource.spec.source);
                            try {
                                await services.applications.update(props.application);
                                setIsAddingSource(false);
                            } catch (e) {
                                props.application.spec.sources.pop();
                                props.appContext.apis.notifications.show({
                                    content: <ErrorNotification title='Unable to create source' e={e} />,
                                    type: NotificationType.Error
                                });
                            } finally {
                                setIsSavingSource(false);
                            }
                        }}
                    />
                </SlidingPanel>
            </div>
        );
    } else {
        // For the three other references of ApplicationParameters. They are single source.
        // Create App, Add source, Rollback and History
        let attributes: EditablePanelItem[] = [];
        if (props.details) {
            return getEditablePanel(
                gatherDetails(
                    0,
                    props.details,
                    attributes,
                    props.tempSource ? props.tempSource : source,
                    app,
                    setRemovedOverrides,
                    removedOverrides,
                    appParamsDeletedState,
                    setAppParamsDeletedState,
                    false
                ),
                props.details
            );
        } else {
            // For single source field, details page where we have to do the load to retrieve repo details
            return (
                <DataLoader input={app} load={application => getSingleSource(application)}>
                    {(details: models.RepoAppDetails) => {
                        attributes = [];
                        const attr = gatherDetails(
                            0,
                            details,
                            attributes,
                            source,
                            app,
                            setRemovedOverrides,
                            removedOverrides,
                            appParamsDeletedState,
                            setAppParamsDeletedState,
                            false
                        );
                        return getEditablePanel(attr, details);
                    }}
                </DataLoader>
            );
        }
    }

    // Collapse button is separate
    function getEditablePanelForSources(index: number, appSource: models.ApplicationSource): JSX.Element {
        return (collapsible && props.collapsedSources[index] === undefined) || props.collapsedSources[index] ? (
            <div
                key={'app_params_collapsed_' + index}
                className='settings-overview__redirect-panel'
                style={{marginTop: 0}}
                onClick={() => {
                    const currentState = props.collapsedSources[index] !== undefined ? props.collapsedSources[index] : true;
                    props.handleCollapse(index, !currentState);
                }}>
                <div className='editable-panel__collapsible-button'>
                    <i className={`fa fa-angle-down filter__collapse`} />
                </div>
                <div className='settings-overview__redirect-panel__content'>
                    <div className='settings-overview__redirect-panel__title'>Source {index + 1 + ': ' + appSource.repoURL}</div>
                    <div className='settings-overview__redirect-panel__description'>
                        {(appSource.path ? 'PATH=' + appSource.path : '') + (appSource.targetRevision ? (appSource.path ? ', ' : '') + 'REVISION=' + appSource.targetRevision : '')}
                    </div>
                </div>
            </div>
        ) : (
            <div key={'app_params_expanded_' + index} className={classNames('white-box', 'editable-panel')} style={{marginBottom: '18px', paddingBottom: '20px'}}>
                <div key={'app_params_panel_' + index} className='white-box__details'>
                    {collapsible && (
                        <React.Fragment>
                            <div className='editable-panel__collapsible-button'>
                                <i
                                    className={`fa fa-angle-up filter__collapse`}
                                    onClick={() => {
                                        props.handleCollapse(index, !props.collapsedSources[index]);
                                    }}
                                />
                            </div>
                        </React.Fragment>
                    )}
                    <DataLoader
                        key={'app_params_source_' + index}
                        input={app.spec.sources[index]}
                        load={src => getSourceFromAppSources(src, app.metadata.name, app.spec.project, index, 0)}>
                        {(details: models.RepoAppDetails) => getEditablePanelForOneSource(details, index, app.spec.sources[index])}
                    </DataLoader>
                </div>
            </div>
        );
    }

    function getEditablePanel(items: EditablePanelItem[], repoAppDetails: models.RepoAppDetails): any {
        return (
            <div className='application-parameters'>
                <EditablePanel
                    save={
                        props.save &&
                        (async (input: models.Application) => {
                            const updatedSrc = input.spec.source;

                            function isDefined(item: any) {
                                return item !== null && item !== undefined;
                            }
                            function isDefinedWithVersion(item: any) {
                                return item !== null && item !== undefined && item.match(/:/);
                            }
                            if (updatedSrc && updatedSrc.helm?.parameters) {
                                updatedSrc.helm.parameters = updatedSrc.helm.parameters.filter(isDefined);
                            }
                            if (updatedSrc && updatedSrc.kustomize?.images) {
                                updatedSrc.kustomize.images = updatedSrc.kustomize.images.filter(isDefinedWithVersion);
                            }

                            let params = input.spec?.source?.plugin?.parameters;
                            if (params) {
                                for (const param of params) {
                                    if (param.map && param.array) {
                                        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                                        // @ts-ignore
                                        param.map = param.array.reduce((acc, {name, value}) => {
                                            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                                            // @ts-ignore
                                            acc[name] = value;
                                            return acc;
                                        }, {});
                                        delete param.array;
                                    }
                                }
                                params = params.filter(param => !appParamsDeletedState.includes(param.name));
                                input.spec.source.plugin.parameters = params;
                            }
                            if (input.spec.source && input.spec.source.helm?.valuesObject) {
                                input.spec.source.helm.valuesObject = jsYaml.load(input.spec.source.helm.values); // Deserialize json
                                input.spec.source.helm.values = '';
                            }
                            await props.save(input, {});
                            setRemovedOverrides(new Array<boolean>());
                        })
                    }
                    values={((repoAppDetails?.plugin || app?.spec?.source?.plugin) && cloneDeep(app)) || app}
                    validate={updatedApp => {
                        const errors = {} as any;

                        for (const fieldPath of ['spec.source.directory.jsonnet.tlas', 'spec.source.directory.jsonnet.extVars']) {
                            const invalid = ((getNestedField(updatedApp, fieldPath) || []) as Array<models.JsonnetVar>).filter(item => !item.name && !item.code);
                            errors[fieldPath] = invalid.length > 0 ? 'All fields must have name' : null;
                        }

                        if (updatedApp.spec.source && updatedApp.spec.source.helm?.values) {
                            const parsedValues = jsYaml.load(updatedApp.spec.source.helm.values);
                            errors['spec.source.helm.values'] = typeof parsedValues === 'object' ? null : 'Values must be a map';
                        }

                        return errors;
                    }}
                    onModeSwitch={
                        repoAppDetails?.plugin &&
                        (() => {
                            setAppParamsDeletedState([]);
                        })
                    }
                    title={repoAppDetails?.type?.toLocaleUpperCase()}
                    items={items as EditablePanelItem[]}
                    noReadonlyMode={props.noReadonlyMode}
                    hasMultipleSources={false}
                />
            </div>
        );
    }

    function getEditablePanelForOneSource(repoAppDetails: models.RepoAppDetails, ind: number, src: models.ApplicationSource): any {
        let floatingTitle: string;
        const lowerPanelAttributes: EditablePanelItem[] = [];
        const upperPanelAttributes: EditablePanelItem[] = [];

        const upperPanel = gatherCoreSourceDetails(ind, upperPanelAttributes, appSources[ind], app);
        const lowerPanel = gatherDetails(
            ind,
            repoAppDetails,
            lowerPanelAttributes,
            appSources[ind],
            app,
            setRemovedOverrides,
            removedOverrides,
            appParamsDeletedState,
            setAppParamsDeletedState,
            true
        );

        if (repoAppDetails.type === 'Directory') {
            floatingTitle =
                'Source ' +
                (ind + 1) +
                ': TYPE=' +
                repoAppDetails.type +
                ', URL=' +
                src.repoURL +
                (repoAppDetails.path ? ', PATH=' + repoAppDetails.path : '') +
                (src.targetRevision ? ', TARGET REVISION=' + src.targetRevision : '');
        } else if (repoAppDetails.type === 'Helm') {
            floatingTitle =
                'Source ' +
                (ind + 1) +
                ': TYPE=' +
                repoAppDetails.type +
                ', URL=' +
                src.repoURL +
                (src.chart ? ', CHART=' + src.chart + ':' + src.targetRevision : '') +
                (src.path ? ', PATH=' + src.path : '') +
                (src.targetRevision ? ', REVISION=' + src.targetRevision : '');
        } else if (repoAppDetails.type === 'Kustomize') {
            floatingTitle =
                'Source ' +
                (ind + 1) +
                ': TYPE=' +
                repoAppDetails.type +
                ', URL=' +
                src.repoURL +
                (repoAppDetails.path ? ', PATH=' + repoAppDetails.path : '') +
                (src.targetRevision ? ', TARGET REVISION=' + src.targetRevision : '');
        } else if (repoAppDetails.type === 'Plugin') {
            floatingTitle =
                'Source ' +
                (ind + 1) +
                ': TYPE=' +
                repoAppDetails.type +
                ', URL=' +
                src.repoURL +
                (repoAppDetails.path ? ', PATH=' + repoAppDetails.path : '') +
                (src.targetRevision ? ', TARGET REVISION=' + src.targetRevision : '');
        }
        return (
            <ApplicationParametersSource
                index={ind}
                saveTop={props.save}
                saveBottom={
                    props.save &&
                    (async (input: models.Application) => {
                        const appSrc = input.spec.sources[ind];

                        function isDefined(item: any) {
                            return item !== null && item !== undefined;
                        }
                        function isDefinedWithVersion(item: any) {
                            return item !== null && item !== undefined && item.match(/:/);
                        }

                        if (appSrc.helm && appSrc.helm.parameters) {
                            appSrc.helm.parameters = appSrc.helm.parameters.filter(isDefined);
                        }
                        if (appSrc.kustomize && appSrc.kustomize.images) {
                            appSrc.kustomize.images = appSrc.kustomize.images.filter(isDefinedWithVersion);
                        }

                        let params = input.spec?.sources[ind]?.plugin?.parameters;
                        if (params) {
                            for (const param of params) {
                                if (param.map && param.array) {
                                    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                                    // @ts-ignore
                                    param.map = param.array.reduce((acc, {name, value}) => {
                                        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                                        // @ts-ignore
                                        acc[name] = value;
                                        return acc;
                                    }, {});
                                    delete param.array;
                                }
                            }

                            params = params.filter(param => !appParamsDeletedState.includes(param.name));
                            appSrc.plugin.parameters = params;
                        }
                        if (appSrc.helm && appSrc.helm.valuesObject) {
                            appSrc.helm.valuesObject = jsYaml.load(appSrc.helm.values); // Deserialize json
                            appSrc.helm.values = '';
                        }

                        await props.save(input, {});
                        setRemovedOverrides(new Array<boolean>());
                    })
                }
                valuesTop={(app?.spec?.sources && (repoAppDetails.plugin || app?.spec?.sources[ind]?.plugin) && cloneDeep(app)) || app}
                valuesBottom={(app?.spec?.sources && (repoAppDetails.plugin || app?.spec?.sources[ind]?.plugin) && cloneDeep(app)) || app}
                validateTop={updatedApp => {
                    const errors = [] as any;
                    const repoURL = updatedApp.spec.sources[ind].repoURL;
                    if (repoURL === null || repoURL.length === 0) {
                        errors['spec.sources[' + ind + '].repoURL'] = 'The source repo URL cannot be empty';
                    } else {
                        errors['spec.sources[' + ind + '].repoURL'] = null;
                    }
                    return errors;
                }}
                validateBottom={updatedApp => {
                    const errors = {} as any;

                    for (const fieldPath of ['spec.sources[' + ind + '].directory.jsonnet.tlas', 'spec.sources[' + ind + '].directory.jsonnet.extVars']) {
                        const invalid = ((getNestedField(updatedApp, fieldPath) || []) as Array<models.JsonnetVar>).filter(item => !item.name && !item.code);
                        errors[fieldPath] = invalid.length > 0 ? 'All fields must have name' : null;
                    }

                    if (updatedApp.spec.sources[ind].helm?.values) {
                        const parsedValues = jsYaml.load(updatedApp.spec.sources[ind].helm.values);
                        errors['spec.sources[' + ind + '].helm.values'] = typeof parsedValues === 'object' ? null : 'Values must be a map';
                    }

                    return errors;
                }}
                onModeSwitch={
                    repoAppDetails.plugin &&
                    (() => {
                        setAppParamsDeletedState([]);
                    })
                }
                titleBottom={repoAppDetails.type.toLocaleUpperCase()}
                titleTop={'SOURCE ' + (ind + 1)}
                floatingTitle={floatingTitle ? floatingTitle : null}
                itemsBottom={lowerPanel as EditablePanelItem[]}
                itemsTop={upperPanel as EditablePanelItem[]}
                noReadonlyMode={props.noReadonlyMode}
                collapsible={collapsible}
                numberOfSources={app?.spec?.sources.length}
                deleteSource={() => {
                    deleteSourceAction(app, app.spec.sources.at(ind), props.appContext);
                }}
            />
        );
    }
};

function gatherCoreSourceDetails(i: number, attributes: EditablePanelItem[], source: models.ApplicationSource, app: models.Application): EditablePanelItem[] {
    const hasMultipleSources = app.spec.sources && app.spec.sources.length > 0;
    // eslint-disable-next-line no-prototype-builtins
    const isHelm = source.hasOwnProperty('chart');
    const repoUrlField = 'spec.sources[' + i + '].repoURL';
    const sourcesPathField = 'spec.sources[' + i + '].path';
    const refField = 'spec.sources[' + i + '].ref';
    const chartField = 'spec.sources[' + i + '].chart';
    const revisionField = 'spec.sources[' + i + '].targetRevision';
    // For single source apps using the source field, these fields are shown in the Summary tab.
    if (hasMultipleSources) {
        attributes.push({
            title: 'REPO URL',
            view: <Repo url={source.repoURL} />,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field={repoUrlField} component={Text} />
        });
        if (isHelm) {
            attributes.push({
                title: 'CHART',
                view: (
                    <span>
                        {source.chart}:{source.targetRevision}
                    </span>
                ),
                edit: (formApi: FormApi) => (
                    <DataLoader input={{repoURL: source.repoURL}} load={src => services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())}>
                        {(charts: models.HelmChart[]) => (
                            <div className='row'>
                                <div className='columns small-8'>
                                    <FormField
                                        formApi={formApi}
                                        field={chartField}
                                        component={AutocompleteField}
                                        componentProps={{
                                            items: charts.map(chart => chart.name),
                                            filterSuggestions: true
                                        }}
                                    />
                                </div>
                                <DataLoader
                                    input={{charts, chart: source.chart}}
                                    load={async data => {
                                        const chartInfo = data.charts.find(chart => chart.name === data.chart);
                                        return (chartInfo && chartInfo.versions) || new Array<string>();
                                    }}>
                                    {(versions: string[]) => (
                                        <div className='columns small-4'>
                                            <FormField
                                                formApi={formApi}
                                                field={revisionField}
                                                component={AutocompleteField}
                                                componentProps={{
                                                    items: versions
                                                }}
                                            />
                                            <RevisionHelpIcon type='helm' top='0' />
                                        </div>
                                    )}
                                </DataLoader>
                            </div>
                        )}
                    </DataLoader>
                )
            });
        } else {
            attributes.push({
                title: 'TARGET REVISION',
                view: <Revision repoUrl={source.repoURL} revision={source.targetRevision || 'HEAD'} />,
                edit: (formApi: FormApi) => <RevisionFormField helpIconTop={'0'} hideLabel={true} formApi={formApi} repoURL={source.repoURL} fieldValue={revisionField} />
            });
            attributes.push({
                title: 'PATH',
                view: (
                    <Revision repoUrl={source.repoURL} revision={source.targetRevision || 'HEAD'} path={source.path} isForPath={true}>
                        {processPath(source.path)}
                    </Revision>
                ),
                edit: (formApi: FormApi) => <FormField formApi={formApi} field={sourcesPathField} component={Text} />
            });
            attributes.push({
                title: 'REF',
                view: <span>{source.ref}</span>,
                edit: (formApi: FormApi) => <FormField formApi={formApi} field={refField} component={Text} />
            });
        }
    }
    return attributes;
}

function gatherDetails(
    ind: number,
    repoDetails: models.RepoAppDetails,
    attributes: EditablePanelItem[],
    source: models.ApplicationSource,
    app: models.Application,
    setRemovedOverrides: any,
    removedOverrides: any,
    appParamsDeletedState: any[],
    setAppParamsDeletedState: any,
    isMultiSource: boolean
): EditablePanelItem[] {
    if (repoDetails.type === 'Kustomize' && repoDetails.kustomize) {
        attributes.push({
            title: 'VERSION',
            view: (source.kustomize && source.kustomize.version) || <span>default</span>,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.authService.settings()}>
                    {settings =>
                        ((settings.kustomizeVersions || []).length > 0 && (
                            <FormField
                                formApi={formApi}
                                field={isMultiSource ? 'spec.sources[' + ind + '].kustomize.version' : 'spec.source.kustomize.version'}
                                component={AutocompleteField}
                                componentProps={{items: settings.kustomizeVersions}}
                            />
                        )) || <span>default</span>
                    }
                </DataLoader>
            )
        });

        attributes.push({
            title: 'NAME PREFIX',
            view: source.kustomize && source.kustomize.namePrefix,
            edit: (formApi: FormApi) => (
                <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].kustomize.namePrefix' : 'spec.source.kustomize.namePrefix'} component={Text} />
            )
        });

        attributes.push({
            title: 'NAME SUFFIX',
            view: source.kustomize && source.kustomize.nameSuffix,
            edit: (formApi: FormApi) => (
                <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].kustomize.nameSuffix' : 'spec.source.kustomize.nameSuffix'} component={Text} />
            )
        });

        attributes.push({
            title: 'NAMESPACE',
            view: source.kustomize && source.kustomize.namespace,
            edit: (formApi: FormApi) => (
                <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].kustomize.namespace' : 'spec.source.kustomize.namespace'} component={Text} />
            )
        });

        const srcImages = ((repoDetails && repoDetails.kustomize && repoDetails.kustomize.images) || []).map(val => kustomize.parse(val));
        const images = ((source.kustomize && source.kustomize.images) || []).map(val => kustomize.parse(val));

        if (srcImages.length > 0) {
            const imagesByName = new Map<string, kustomize.Image>();
            srcImages.forEach(img => imagesByName.set(img.name, img));

            const overridesByName = new Map<string, number>();
            images.forEach((override, i) => overridesByName.set(override.name, i));

            attributes = attributes.concat(
                getParamsEditableItems(
                    app,
                    'IMAGES',
                    isMultiSource ? 'spec.sources[' + ind + '].kustomize.images' : 'spec.source.kustomize.images',
                    removedOverrides,
                    setRemovedOverrides,
                    distinct(imagesByName.keys(), overridesByName.keys()).map(name => {
                        const param = imagesByName.get(name);
                        const original = param && kustomize.format(param);
                        let overrideIndex = overridesByName.get(name);
                        if (overrideIndex === undefined) {
                            overrideIndex = -1;
                        }
                        const value = (overrideIndex > -1 && kustomize.format(images[overrideIndex])) || original;
                        return {overrideIndex, original, metadata: {name, value}};
                    }),
                    ImageTagFieldEditor
                )
            );
        }
    } else if (repoDetails.type === 'Helm' && repoDetails.helm) {
        const isValuesObject = source?.helm?.valuesObject;
        const helmValues = isValuesObject ? jsYaml.dump(source.helm.valuesObject) : source?.helm?.values;
        attributes.push({
            title: 'VALUES FILES',
            view: (source.helm && (source.helm.valueFiles || []).join(', ')) || 'No values files selected',
            edit: (formApi: FormApi) => (
                <FormField
                    formApi={formApi}
                    field={isMultiSource ? 'spec.sources[' + ind + '].helm.valueFiles' : 'spec.source.helm.valueFiles'}
                    component={TagsInputField}
                    componentProps={{
                        options: repoDetails.helm.valueFiles,
                        noTagsLabel: 'No values files selected'
                    }}
                />
            )
        });
        attributes.push({
            title: 'VALUES',
            view: source.helm && (
                <Expandable>
                    <pre>{helmValues}</pre>
                </Expandable>
            ),
            edit: (formApi: FormApi) => {
                // In case source.helm.valuesObject is set, set source.helm.values to its value
                if (source.helm) {
                    source.helm.values = helmValues;
                }

                return (
                    <div>
                        <pre>
                            <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].helm.values' : 'spec.source.helm.values'} component={TextArea} />
                        </pre>
                    </div>
                );
            }
        });
        const paramsByName = new Map<string, models.HelmParameter>();
        (repoDetails.helm.parameters || []).forEach(param => paramsByName.set(param.name, param));
        const overridesByName = new Map<string, number>();
        ((source.helm && source.helm.parameters) || []).forEach((override, i) => overridesByName.set(override.name, i));
        attributes = attributes.concat(
            getParamsEditableItems(
                app,
                'PARAMETERS',
                isMultiSource ? 'spec.sources[' + ind + '].helm.parameters' : 'spec.source.helm.parameters',
                removedOverrides,
                setRemovedOverrides,
                distinct(paramsByName.keys(), overridesByName.keys()).map(name => {
                    const param = paramsByName.get(name);
                    const original = (param && param.value) || '';
                    let overrideIndex = overridesByName.get(name);
                    if (overrideIndex === undefined) {
                        overrideIndex = -1;
                    }
                    const value = (overrideIndex > -1 && source.helm.parameters[overrideIndex].value) || original;
                    return {overrideIndex, original, metadata: {name, value}};
                })
            )
        );
        const fileParamsByName = new Map<string, models.HelmFileParameter>();
        (repoDetails.helm.fileParameters || []).forEach(param => fileParamsByName.set(param.name, param));
        const fileOverridesByName = new Map<string, number>();
        ((source.helm && source.helm.fileParameters) || []).forEach((override, i) => fileOverridesByName.set(override.name, i));
        attributes = attributes.concat(
            getParamsEditableItems(
                app,
                'PARAMETERS',
                isMultiSource ? 'spec.sources[' + ind + '].helm.parameters' : 'spec.source.helm.parameters',
                removedOverrides,
                setRemovedOverrides,
                distinct(fileParamsByName.keys(), fileOverridesByName.keys()).map(name => {
                    const param = fileParamsByName.get(name);
                    const original = (param && param.path) || '';
                    let overrideIndex = fileOverridesByName.get(name);
                    if (overrideIndex === undefined) {
                        overrideIndex = -1;
                    }
                    const value = (overrideIndex > -1 && source.helm.fileParameters[overrideIndex].path) || original;
                    return {overrideIndex, original, metadata: {name, value}};
                })
            )
        );
    } else if (repoDetails.type === 'Plugin') {
        attributes.push({
            title: 'NAME',
            view: <div style={{marginTop: 15, marginBottom: 5}}>{ValueEditor(app.spec.source?.plugin?.name, null)}</div>,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.authService.plugins()}>
                    {(plugins: Plugin[]) => (
                        <FormField
                            formApi={formApi}
                            field={isMultiSource ? 'spec.sources[' + ind + '].plugin.name' : 'spec.source.plugin.name'}
                            component={FormSelect}
                            componentProps={{options: plugins.map(p => p.name)}}
                        />
                    )}
                </DataLoader>
            )
        });
        attributes.push({
            title: 'ENV',
            view: (
                <div style={{marginTop: 15}}>
                    {(app.spec.source?.plugin?.env || []).map(val => (
                        <span key={val.name} style={{display: 'block', marginBottom: 5}}>
                            {NameValueEditor(val, null)}
                        </span>
                    ))}
                </div>
            ),
            edit: (formApi: FormApi) => (
                <FormField field={isMultiSource ? 'spec.sources[' + ind + '].plugin.env' : 'spec.source.plugin.env'} formApi={formApi} component={ArrayInputField} />
            )
        });
        const parametersSet = new Set<string>();
        if (repoDetails?.plugin?.parametersAnnouncement) {
            for (const announcement of repoDetails.plugin.parametersAnnouncement) {
                parametersSet.add(announcement.name);
            }
        }
        if (app.spec.source?.plugin?.parameters) {
            for (const appParameter of app.spec.source.plugin.parameters) {
                parametersSet.add(appParameter.name);
            }
        }

        for (const key of appParamsDeletedState) {
            parametersSet.delete(key);
        }
        parametersSet.forEach(name => {
            const announcement = repoDetails.plugin.parametersAnnouncement?.find(param => param.name === name);
            const liveParam = app.spec.source?.plugin?.parameters?.find(param => param.name === name);
            const pluginIcon =
                announcement && liveParam ? 'This parameter has been provided by plugin, but is overridden in application manifest.' : 'This parameter is provided by the plugin.';
            const isPluginPar = !!announcement;
            if ((announcement?.collectionType === undefined && liveParam?.map) || announcement?.collectionType === 'map') {
                let liveParamMap;
                if (liveParam) {
                    liveParamMap = liveParam.map ?? new Map<string, string>();
                }
                const map = concatMaps(liveParamMap ?? announcement?.map, new Map<string, string>());
                const entries = map.entries();
                const items = new Array<NameValue>();
                Array.from(entries).forEach(([key, value]) => items.push({name: key, value: `${value}`}));
                attributes.push({
                    title: announcement?.title ?? announcement?.name ?? name,
                    customTitle: (
                        <span>
                            {isPluginPar && <i className='fa solid fa-puzzle-piece' title={pluginIcon} style={{marginRight: 5}} />}
                            {announcement?.title ?? announcement?.name ?? name}
                        </span>
                    ),
                    view: (
                        <div style={{marginTop: 15, marginBottom: 5}}>
                            {items.length === 0 && <span style={{color: 'dimgray'}}>-- NO ITEMS --</span>}
                            {items.map(val => (
                                <span key={val.name} style={{display: 'block', marginBottom: 5}}>
                                    {NameValueEditor(val)}
                                </span>
                            ))}
                        </div>
                    ),
                    edit: (formApi: FormApi) => (
                        <FormField
                            field={isMultiSource ? 'spec.sources[' + ind + '].plugin.parameters' : 'spec.source.plugin.parameters'}
                            componentProps={{
                                name: announcement?.name ?? name,
                                defaultVal: announcement?.map,
                                isPluginPar,
                                setAppParamsDeletedState
                            }}
                            formApi={formApi}
                            component={MapValueField}
                        />
                    )
                });
            } else if ((announcement?.collectionType === undefined && liveParam?.array) || announcement?.collectionType === 'array') {
                let liveParamArray;
                if (liveParam) {
                    liveParamArray = liveParam?.array ?? [];
                }
                attributes.push({
                    title: announcement?.title ?? announcement?.name ?? name,
                    customTitle: (
                        <span>
                            {isPluginPar && <i className='fa-solid fa-puzzle-piece' title={pluginIcon} style={{marginRight: 5}} />}
                            {announcement?.title ?? announcement?.name ?? name}
                        </span>
                    ),
                    view: (
                        <div style={{marginTop: 15, marginBottom: 5}}>
                            {(liveParamArray ?? announcement?.array ?? []).length === 0 && <span style={{color: 'dimgray'}}>-- NO ITEMS --</span>}
                            {(liveParamArray ?? announcement?.array ?? []).map((val, index) => (
                                <span key={index} style={{display: 'block', marginBottom: 5}}>
                                    {ValueEditor(val, null)}
                                </span>
                            ))}
                        </div>
                    ),
                    edit: (formApi: FormApi) => (
                        <FormField
                            field={isMultiSource ? 'spec.sources[' + ind + '].plugin.parameters' : 'spec.source.plugin.parameters'}
                            componentProps={{
                                name: announcement?.name ?? name,
                                defaultVal: announcement?.array,
                                isPluginPar,
                                setAppParamsDeletedState
                            }}
                            formApi={formApi}
                            component={ArrayValueField}
                        />
                    )
                });
            } else if (
                (announcement?.collectionType === undefined && liveParam?.string) ||
                announcement?.collectionType === '' ||
                announcement?.collectionType === 'string' ||
                announcement?.collectionType === undefined
            ) {
                let liveParamString;
                if (liveParam) {
                    liveParamString = liveParam?.string ?? '';
                }
                attributes.push({
                    title: announcement?.title ?? announcement?.name ?? name,
                    customTitle: (
                        <span>
                            {isPluginPar && <i className='fa-solid fa-puzzle-piece' title={pluginIcon} style={{marginRight: 5}} />}
                            {announcement?.title ?? announcement?.name ?? name}
                        </span>
                    ),
                    view: (
                        <div
                            style={{
                                marginTop: 15,
                                marginBottom: 5
                            }}>
                            {ValueEditor(liveParamString ?? announcement?.string, null)}
                        </div>
                    ),
                    edit: (formApi: FormApi) => (
                        <FormField
                            field={isMultiSource ? 'spec.sources[' + ind + '].plugin.parameters' : 'spec.source.plugin.parameters'}
                            componentProps={{
                                name: announcement?.name ?? name,
                                defaultVal: announcement?.string,
                                isPluginPar,
                                setAppParamsDeletedState
                            }}
                            formApi={formApi}
                            component={StringValueField}
                        />
                    )
                });
            }
        });
    } else if (repoDetails.type === 'Directory') {
        const directory = source.directory || ({} as ApplicationSourceDirectory);
        const fieldValue = isMultiSource ? 'spec.sources[' + ind + '].directory.recurse' : 'spec.source.directory.recurse';
        attributes.push({
            title: 'DIRECTORY RECURSE',
            view: (!!directory.recurse).toString(),
            edit: (formApi: FormApi) => <FormField formApi={formApi} field={fieldValue} component={CheckboxField} />
        });
        attributes.push({
            title: 'TOP-LEVEL ARGUMENTS',
            view: ((directory?.jsonnet && directory?.jsonnet.tlas) || []).map((i, j) => (
                <label key={j}>
                    {i.name}='{i.value}' {i.code && 'code'}
                </label>
            )),
            edit: (formApi: FormApi) => (
                <FormField
                    field={isMultiSource ? 'spec.sources[' + ind + '].directory.jsonnet.tlas' : 'spec.source.directory.jsonnet.tlas'}
                    formApi={formApi}
                    component={VarsInputField}
                />
            )
        });
        attributes.push({
            title: 'EXTERNAL VARIABLES',
            view: ((directory.jsonnet && directory.jsonnet.extVars) || []).map((i, j) => (
                <label key={j}>
                    {i.name}='{i.value}' {i.code && 'code'}
                </label>
            )),
            edit: (formApi: FormApi) => (
                <FormField
                    field={isMultiSource ? 'spec.sources[' + ind + '].directory.jsonnet.extVars' : 'spec.source.directory.jsonnet.extVars'}
                    formApi={formApi}
                    component={VarsInputField}
                />
            )
        });

        attributes.push({
            title: 'INCLUDE',
            view: directory && directory.include,
            edit: (formApi: FormApi) => (
                <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].directory.include' : 'spec.source.directory.include'} component={Text} />
            )
        });

        attributes.push({
            title: 'EXCLUDE',
            view: directory && directory.exclude,
            edit: (formApi: FormApi) => (
                <FormField formApi={formApi} field={isMultiSource ? 'spec.sources[' + ind + '].directory.exclude' : 'spec.source.directory.exclude'} component={Text} />
            )
        });
    }
    return attributes;
}

// For Sources field. Get one source with index i from the list
async function getSourceFromAppSources(aSource: models.ApplicationSource, name: string, project: string, index: number, version: number) {
    const repoDetail = await services.repos.appDetails(aSource, name, project, index, version).catch(() => ({
        type: 'Directory' as models.AppSourceType,
        path: aSource.path
    }));
    return repoDetail;
}

// Delete when source field is removed
async function getSingleSource(app: models.Application) {
    if (app.spec.source) {
        const repoDetail = await services.repos.appDetails(getAppDefaultSource(app), app.metadata.name, app.spec.project, 0, 0).catch(() => ({
            type: 'Directory' as models.AppSourceType,
            path: getAppDefaultSource(app).path
        }));
        return repoDetail;
    }
    return null;
}
