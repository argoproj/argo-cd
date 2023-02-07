import {AutocompleteField, DataLoader, FormField, FormSelect, getNestedField} from 'argo-ui';
import * as React from 'react';
import {FieldApi, FormApi, FormField as ReactFormField, Text, TextArea} from 'react-form';

import {ArrayInputField, CheckboxField, EditablePanel, EditablePanelItem, Expandable, TagsInputField} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ApplicationSourceDirectory, Plugin} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ImageTagFieldEditor} from './kustomize';
import * as kustomize from './kustomize-image';
import {VarsInputField} from './vars-input-field';
import {concatMaps} from '../../../shared/utils';
import {getAppDefaultSource} from '../utils';

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
    const app = props.application;
    const source = getAppDefaultSource(app);
    const [removedOverrides, setRemovedOverrides] = React.useState(new Array<boolean>());

    let attributes: EditablePanelItem[] = [];

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
                    <pre>{source.helm.values}</pre>
                </Expandable>
            ),
            edit: (formApi: FormApi) => (
                <div>
                    <pre>
                        <FormField formApi={formApi} field='spec.source.helm.values' component={TextArea} />
                    </pre>
                </div>
            )
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
            view: source.plugin && source.plugin.name,
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
            view: source.plugin && (source.plugin.env || []).map(i => `${i.name}='${i.value}'`).join(' '),
            edit: (formApi: FormApi) => <FormField field='spec.source.plugin.env' formApi={formApi} component={ArrayInputField} />
        });
        if (props.details.plugin.parametersAnnouncement) {
            for (const announcement of props.details.plugin.parametersAnnouncement) {
                const liveParam = app.spec.source.plugin.parameters?.find(param => param.name === announcement.name);
                if (announcement.collectionType === undefined || announcement.collectionType === '' || announcement.collectionType === 'string') {
                    attributes.push({
                        title: announcement.title ?? announcement.name,
                        view: liveParam?.string || announcement.string,
                        edit: () => liveParam?.string || announcement.string
                    });
                } else if (announcement.collectionType === 'array') {
                    attributes.push({
                        title: announcement.title ?? announcement.name,
                        view: (liveParam?.array || announcement.array || []).join(' '),
                        edit: () => (liveParam?.array || announcement.array || []).join(' ')
                    });
                } else if (announcement.collectionType === 'map') {
                    const entries = concatMaps(announcement.map, liveParam?.map).entries();
                    attributes.push({
                        title: announcement.title ?? announcement.name,
                        view: Array.from(entries)
                            .map(([key, value]) => `${key}='${value}'`)
                            .join(' '),
                        edit: () =>
                            Array.from(entries)
                                .map(([key, value]) => `${key}='${value}'`)
                                .join(' ')
                    });
                }
            }
        }
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
                    await props.save(input, {});
                    setRemovedOverrides(new Array<boolean>());
                })
            }
            values={app}
            validate={updatedApp => {
                const errors = {} as any;

                for (const fieldPath of ['spec.source.directory.jsonnet.tlas', 'spec.source.directory.jsonnet.extVars']) {
                    const invalid = ((getNestedField(updatedApp, fieldPath) || []) as Array<models.JsonnetVar>).filter(item => !item.name && !item.code);
                    errors[fieldPath] = invalid.length > 0 ? 'All fields must have name' : null;
                }

                return errors;
            }}
            title={props.details.type.toLocaleUpperCase()}
            items={attributes}
            noReadonlyMode={props.noReadonlyMode}
            hasMultipleSources={app.spec.sources && app.spec.sources.length > 0}
        />
    );
};
