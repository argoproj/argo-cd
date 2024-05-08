import {AutocompleteField, DataLoader, FormField, FormSelect, getNestedField} from 'argo-ui';
import * as React from 'react';
import {FieldApi, FormApi, FormField as ReactFormField, Text, TextArea} from 'react-form';
import {cloneDeep} from 'lodash-es';
import {
    ArrayInputField,
    ArrayValueField,
    CheckboxField,
    EditablePanel,
    EditablePanelItem,
    Expandable,
    MapValueField,
    NameValueEditor,
    StringValueField,
    NameValue,
    TagsInputField,
    ValueEditor
} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ApplicationSourceDirectory, Plugin} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ImageTagFieldEditor} from './kustomize';
import * as kustomize from './kustomize-image';
import {VarsInputField} from './vars-input-field';
import {concatMaps} from '../../../shared/utils';
import {getAppDefaultSource} from '../utils';
import * as jsYaml from 'js-yaml';

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
    details: models.RepoAppDetails;
    save?: (application: models.Application, query: {validate?: boolean}) => Promise<any>;
    noReadonlyMode?: boolean;
}) => {
    const app = cloneDeep(props.application);
    const source = getAppDefaultSource(app);
    const [removedOverrides, setRemovedOverrides] = React.useState(new Array<boolean>());

    let attributes: EditablePanelItem[] = [];
    const isValuesObject = source?.helm?.valuesObject;
    const helmValues = isValuesObject ? jsYaml.safeDump(source.helm.valuesObject) : source?.helm?.values;
    const [appParamsDeletedState, setAppParamsDeletedState] = React.useState([]);

    if (props.details.type === 'Kustomize' && props.details.kustomize) {
        attributes.push({
            title: 'VERSION',
            view: (source.kustomize && source.kustomize.version) || <span>default</span>,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.authService.settings()}>
                    {settings =>
                        ((settings.kustomizeVersions || []).length > 0 && (
                            <FormField formApi={formApi} field='spec.source.kustomize.version' component={AutocompleteField} componentProps={{items: settings.kustomizeVersions}} />
                        )) || <span>default</span>
                    }
                </DataLoader>
            )
        });

        attributes.push({
            title: 'NAME PREFIX',
            view: source.kustomize && source.kustomize.namePrefix,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.kustomize.namePrefix' component={Text} />
        });

        attributes.push({
            title: 'NAME SUFFIX',
            view: source.kustomize && source.kustomize.nameSuffix,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.kustomize.nameSuffix' component={Text} />
        });

        attributes.push({
            title: 'NAMESPACE',
            view: app.spec.source.kustomize && app.spec.source.kustomize.namespace,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.kustomize.namespace' component={Text} />
        });

        const srcImages = ((props.details && props.details.kustomize && props.details.kustomize.images) || []).map(val => kustomize.parse(val));
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
                    'spec.source.kustomize.images',
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
    } else if (props.details.type === 'Helm' && props.details.helm) {
        attributes.push({
            title: 'VALUES FILES',
            view: (source.helm && (source.helm.valueFiles || []).join(', ')) || 'No values files selected',
            edit: (formApi: FormApi) => (
                <FormField
                    formApi={formApi}
                    field='spec.source.helm.valueFiles'
                    component={TagsInputField}
                    componentProps={{
                        options: props.details.helm.valueFiles,
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
                            <FormField formApi={formApi} field='spec.source.helm.values' component={TextArea} />
                        </pre>
                    </div>
                );
            }
        });
        const paramsByName = new Map<string, models.HelmParameter>();
        (props.details.helm.parameters || []).forEach(param => paramsByName.set(param.name, param));
        const overridesByName = new Map<string, number>();
        ((source.helm && source.helm.parameters) || []).forEach((override, i) => overridesByName.set(override.name, i));
        attributes = attributes.concat(
            getParamsEditableItems(
                app,
                'PARAMETERS',
                'spec.source.helm.parameters',
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
        (props.details.helm.fileParameters || []).forEach(param => fileParamsByName.set(param.name, param));
        const fileOverridesByName = new Map<string, number>();
        ((source.helm && source.helm.fileParameters) || []).forEach((override, i) => fileOverridesByName.set(override.name, i));
        attributes = attributes.concat(
            getParamsEditableItems(
                app,
                'PARAMETERS',
                'spec.source.helm.parameters',
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
    } else if (props.details.type === 'Plugin') {
        attributes.push({
            title: 'NAME',
            view: <div style={{marginTop: 15, marginBottom: 5}}>{ValueEditor(app.spec.source?.plugin?.name, null)}</div>,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.authService.plugins()}>
                    {(plugins: Plugin[]) => (
                        <FormField formApi={formApi} field='spec.source.plugin.name' component={FormSelect} componentProps={{options: plugins.map(p => p.name)}} />
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
            edit: (formApi: FormApi) => <FormField field='spec.source.plugin.env' formApi={formApi} component={ArrayInputField} />
        });
        const parametersSet = new Set<string>();
        if (props.details?.plugin?.parametersAnnouncement) {
            for (const announcement of props.details.plugin.parametersAnnouncement) {
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
            const announcement = props.details.plugin.parametersAnnouncement?.find(param => param.name === name);
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
                            field='spec.source.plugin.parameters'
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
                            field='spec.source.plugin.parameters'
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
                            field='spec.source.plugin.parameters'
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
    } else if (props.details.type === 'Directory') {
        const directory = source.directory || ({} as ApplicationSourceDirectory);
        attributes.push({
            title: 'DIRECTORY RECURSE',
            view: (!!directory.recurse).toString(),
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.directory.recurse' component={CheckboxField} />
        });
        attributes.push({
            title: 'TOP-LEVEL ARGUMENTS',
            view: ((directory?.jsonnet && directory?.jsonnet.tlas) || []).map((i, j) => (
                <label key={j}>
                    {i.name}='{i.value}' {i.code && 'code'}
                </label>
            )),
            edit: (formApi: FormApi) => <FormField field='spec.source.directory.jsonnet.tlas' formApi={formApi} component={VarsInputField} />
        });
        attributes.push({
            title: 'EXTERNAL VARIABLES',
            view: ((directory.jsonnet && directory.jsonnet.extVars) || []).map((i, j) => (
                <label key={j}>
                    {i.name}='{i.value}' {i.code && 'code'}
                </label>
            )),
            edit: (formApi: FormApi) => <FormField field='spec.source.directory.jsonnet.extVars' formApi={formApi} component={VarsInputField} />
        });

        attributes.push({
            title: 'INCLUDE',
            view: directory && directory.include,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.directory.include' component={Text} />
        });

        attributes.push({
            title: 'EXCLUDE',
            view: directory && directory.exclude,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.directory.exclude' component={Text} />
        });
    }

    return (
        <EditablePanel
            save={
                props.save &&
                (async (input: models.Application) => {
                    const src = getAppDefaultSource(input);

                    function isDefined(item: any) {
                        return item !== null && item !== undefined;
                    }
                    function isDefinedWithVersion(item: any) {
                        return item !== null && item !== undefined && item.match(/:/);
                    }

                    if (src.helm && src.helm.parameters) {
                        src.helm.parameters = src.helm.parameters.filter(isDefined);
                    }
                    if (src.kustomize && src.kustomize.images) {
                        src.kustomize.images = src.kustomize.images.filter(isDefinedWithVersion);
                    }

                    let params = input.spec?.source?.plugin?.parameters;
                    if (params) {
                        for (const param of params) {
                            if (param.map && param.array) {
                                // @ts-ignore
                                param.map = param.array.reduce((acc, {name, value}) => {
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
                    if (input.spec.source.helm && input.spec.source.helm.valuesObject) {
                        input.spec.source.helm.valuesObject = jsYaml.safeLoad(input.spec.source.helm.values); // Deserialize json
                        input.spec.source.helm.values = '';
                    }
                    await props.save(input, {});
                    setRemovedOverrides(new Array<boolean>());
                })
            }
            values={((props.details.plugin || app?.spec?.source?.plugin) && cloneDeep(app)) || app}
            validate={updatedApp => {
                const errors = {} as any;

                for (const fieldPath of ['spec.source.directory.jsonnet.tlas', 'spec.source.directory.jsonnet.extVars']) {
                    const invalid = ((getNestedField(updatedApp, fieldPath) || []) as Array<models.JsonnetVar>).filter(item => !item.name && !item.code);
                    errors[fieldPath] = invalid.length > 0 ? 'All fields must have name' : null;
                }

                if (updatedApp.spec.source.helm && updatedApp.spec.source.helm.values) {
                    const parsedValues = jsYaml.safeLoad(updatedApp.spec.source.helm.values);
                    errors['spec.source.helm.values'] = typeof parsedValues === 'object' ? null : 'Values must be a map';
                }

                return errors;
            }}
            onModeSwitch={
                props.details.plugin &&
                (() => {
                    setAppParamsDeletedState([]);
                })
            }
            title={props.details.type.toLocaleUpperCase()}
            items={attributes}
            noReadonlyMode={props.noReadonlyMode}
            hasMultipleSources={app.spec.sources && app.spec.sources.length > 0}
        />
    );
};
