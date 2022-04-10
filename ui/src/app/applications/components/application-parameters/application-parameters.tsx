import {AutocompleteField, DataLoader, FormField, FormSelect, getNestedField} from 'argo-ui';
import * as React from 'react';
import {FieldApi, FormApi, FormField as ReactFormField, Text, TextArea} from 'react-form';

import {ArrayInputField, CheckboxField, EditablePanel, EditablePanelItem, Expandable, TagsInputField} from '../../../shared/components';
import * as models from '../../../shared/models';
import {ApplicationSourceDirectory, AuthSettings} from '../../../shared/models';
import {services} from '../../../shared/services';
import {ImageTagFieldEditor} from './kustomize';
import * as kustomize from './kustomize-image';
import {VarsInputField} from './vars-input-field';

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

function overridesFirst(first: {overrideIndex: number}, second: {overrideIndex: number}) {
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
                const labelStyle = {position: 'absolute', right: 0, top: 0, zIndex: 1} as any;
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
        .sort((first, second) => {
            const firstSortBy = first.key || first.title;
            const secondSortBy = second.key || second.title;
            return firstSortBy.localeCompare(secondSortBy);
        })
        .map((item, i) => ({...item, before: (i === 0 && <p style={{marginTop: '1em'}}>{title}</p>) || null}));
}

export const ApplicationParameters = (props: {
    application: models.Application;
    details: models.RepoAppDetails;
    save?: (application: models.Application, query: {validate?: boolean}) => Promise<any>;
    noReadonlyMode?: boolean;
}) => {
    const app = props.application;
    const source = props.application.spec.source;
    const [removedOverrides, setRemovedOverrides] = React.useState(new Array<boolean>());

    let attributes: EditablePanelItem[] = [];

    if (props.details.type === 'Kustomize' && props.details.kustomize) {
        attributes.push({
            title: 'VERSION',
            view: (app.spec.source.kustomize && app.spec.source.kustomize.version) || <span>default</span>,
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
            view: app.spec.source.kustomize && app.spec.source.kustomize.namePrefix,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.kustomize.namePrefix' component={Text} />
        });

        attributes.push({
            title: 'NAME SUFFIX',
            view: app.spec.source.kustomize && app.spec.source.kustomize.nameSuffix,
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
            view: (app.spec.source.helm && (app.spec.source.helm.valueFiles || []).join(', ')) || 'No values files selected',
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
            view: app.spec.source.helm && (
                <Expandable>
                    <pre>{app.spec.source.helm.values}</pre>
                </Expandable>
            ),
            edit: (formApi: FormApi) => (
                <div>
                    <pre>
                        <FormField formApi={formApi} field='spec.source.helm.values' component={TextArea} />
                    </pre>
                    {props.details.helm.values && (
                        <div>
                            <label>values.yaml</label>
                            <Expandable>
                                <pre>{props.details.helm.values}</pre>
                            </Expandable>
                        </div>
                    )}
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
            view: app.spec.source.plugin && app.spec.source.plugin.name,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.authService.settings()}>
                    {(settings: AuthSettings) => (
                        <FormField formApi={formApi} field='spec.source.plugin.name' component={FormSelect} componentProps={{options: (settings.plugins || []).map(p => p.name)}} />
                    )}
                </DataLoader>
            )
        });
        attributes.push({
            title: 'ENV',
            view: app.spec.source.plugin && (app.spec.source.plugin.env || []).map(i => `${i.name}='${i.value}'`).join(' '),
            edit: (formApi: FormApi) => <FormField field='spec.source.plugin.env' formApi={formApi} component={ArrayInputField} />
        });
    } else if (props.details.type === 'Directory') {
        const directory = app.spec.source.directory || ({} as ApplicationSourceDirectory);
        attributes.push({
            title: 'DIRECTORY RECURSE',
            view: (!!directory.recurse).toString(),
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.directory.recurse' component={CheckboxField} />
        });
        attributes.push({
            title: 'TOP-LEVEL ARGUMENTS',
            view: ((directory.jsonnet && directory.jsonnet.tlas) || []).map((i, j) => (
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
    }

    return (
        <EditablePanel
            save={
                props.save &&
                (async (input: models.Application) => {
                    function isDefined(item: any) {
                        return item !== null && item !== undefined;
                    }
                    function isDefinedWithVersion(item: any) {
                        return item !== null && item !== undefined && item.match(/:/);
                    }

                    if (input.spec.source.helm && input.spec.source.helm.parameters) {
                        input.spec.source.helm.parameters = input.spec.source.helm.parameters.filter(isDefined);
                    }
                    if (input.spec.source.kustomize && input.spec.source.kustomize.images) {
                        input.spec.source.kustomize.images = input.spec.source.kustomize.images.filter(isDefinedWithVersion);
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
        />
    );
};
