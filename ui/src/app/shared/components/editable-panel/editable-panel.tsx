import {ErrorNotification, HelpIcon, NotificationType} from 'argo-ui';
import * as classNames from 'classnames';
import React, {type ReactNode, useCallback, useContext, useEffect, useRef, useState, Fragment} from 'react';
import {Form, type FormApi} from 'react-form';
import {helpTip} from '../../../applications/components/utils';
import {Context} from '../../context';
import {Spinner} from '../spinner';

import './editable-panel.scss';

export interface EditablePanelItem {
    title: string;
    customTitle?: string | ReactNode;
    hint?: string;
    key?: string;
    before?: ReactNode;
    view: string | ReactNode;
    edit?: (formApi: FormApi) => ReactNode;
    titleEdit?: (formApi: FormApi) => ReactNode;
}

export interface EditablePanelSubsection {
    sectionName: string;
    items: EditablePanelItem[];
}

export type EditablePanelContent = EditablePanelItem | EditablePanelSubsection;

function isSubsection(content: EditablePanelContent): content is EditablePanelSubsection {
    return (content as EditablePanelSubsection).sectionName !== undefined;
}

export interface EditablePanelProps<T> {
    title?: string | ReactNode;
    titleCollapsed?: string | ReactNode;
    floatingTitle?: string | ReactNode;
    values: T;
    validate?: (values: T) => any;
    save?: (input: T, query: {validate?: boolean}) => Promise<any>;
    items: EditablePanelContent[];
    onModeSwitch?: () => any;
    noReadonlyMode?: boolean;
    view?: string | ReactNode;
    edit?: (formApi: FormApi) => ReactNode;
    hasMultipleSources?: boolean;
    collapsible?: boolean;
    collapsed?: boolean;
    collapsedDescription?: string;
}

function EditablePanel<T extends {} = {}>({
    title,
    titleCollapsed,
    floatingTitle,
    values,
    validate,
    save,
    items,
    onModeSwitch: onModeSwitchProp,
    noReadonlyMode,
    view,
    edit: editProp,
    hasMultipleSources,
    collapsible,
    collapsed: collapsedProp = false,
    collapsedDescription
}: EditablePanelProps<T>) {
    const [isEditing, setIsEditing] = useState<boolean>(!!noReadonlyMode);
    const [isSaving, setIsSaving] = useState<boolean>(false);
    const [isCollapsed, setIsCollapsed] = useState<boolean>(collapsedProp);
    const ctx = useContext(Context);
    const formApiRef = useRef<FormApi | null>(null);
    const initialValuesRef = useRef<T>(values);

    useEffect(() => {
        setIsCollapsed(collapsedProp);
    }, [collapsedProp]);

    useEffect(() => {
        const initialValuesString = JSON.stringify(initialValuesRef.current);
        const valuesString = JSON.stringify(values);

        if (formApiRef.current && initialValuesString !== valuesString) {
            if (noReadonlyMode) {
                formApiRef.current.setAllValues(values);
            }
            initialValuesRef.current = values;
        } else if (initialValuesString !== valuesString) {
            initialValuesRef.current = values;
        }
    }, [values, noReadonlyMode]);

    const onModeSwitch = useCallback(() => {
        if (onModeSwitchProp) {
            onModeSwitchProp();
        }
    }, [onModeSwitchProp]);

    const handleSubmit = useCallback<<K extends T>(data: K) => Promise<void>>(
        async input => {
            if (!save) return;
            try {
                setIsSaving(true);
                await save(input, {});
                setIsEditing(false);
                setIsSaving(false);
                onModeSwitch();
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to save changes' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setIsSaving(false);
            }
        },
        [save, onModeSwitch, ctx.notifications]
    );

    const handleCancel = useCallback(() => {
        setIsEditing(false);
        onModeSwitch();
    }, [onModeSwitch]);

    const renderItem = (item: EditablePanelItem, api?: FormApi, isIndented = false) => (
        <Fragment key={item.key || item.title}>
            {item.before}
            <div className='row white-box__details-row'>
                <div className='columns small-3' style={isIndented ? {paddingLeft: '2em'} : undefined}>
                    {api && item.titleEdit ? item.titleEdit(api) : item.customTitle || item.title}
                </div>
                <div className='columns small-9'>{api && item.edit ? item.edit(api) : item.view}</div>
            </div>
        </Fragment>
    );

    const renderContent = (content: EditablePanelContent, api?: FormApi) => {
        if (isSubsection(content)) {
            const itemsToRender = api ? content.items : content.items.filter(item => item.view);
            return (
                <div key={content.sectionName} className='editable-panel__subsection'>
                    <div className='row white-box__details-row'>
                        <div className='columns small-12' style={{fontWeight: 'bold', fontSize: '14px', marginTop: '15px', marginBottom: '5px', textTransform: 'uppercase'}}>
                            {content.sectionName}
                        </div>
                    </div>
                    {content.items.map(item => renderItem(item, api, true))}
                </div>
            );
        }
        return renderItem(content, api, false);
    };

    return (
        <>
            {collapsible && isCollapsed ? (
                <div className='settings-overview__redirect-panel' style={{marginTop: 0}} onClick={() => setIsCollapsed(!isCollapsed)}>
                    <div className='settings-overview__redirect-panel__content'>
                        <div className='settings-overview__redirect-panel__title'>{titleCollapsed || title}</div>
                        <div className='settings-overview__redirect-panel__description'>{collapsedDescription}</div>
                    </div>
                    <div className='settings-overview__redirect-panel__arrow'>
                        <i className='fa fa-angle-down' />
                    </div>
                </div>
            ) : (
                <div className={classNames('white-box editable-panel', {'editable-panel--disabled': isSaving})}>
                    {floatingTitle && <div className='white-box--additional-top-space editable-panel__sticky-title'>{floatingTitle}</div>}
                    <div className='white-box__details'>
                        {!noReadonlyMode && save && (
                            <div className='editable-panel__buttons' style={{right: collapsible ? '5em' : '3em'}}>
                                {!isEditing && (
                                    <button
                                        onClick={() => {
                                            setIsEditing(true);
                                            onModeSwitch();
                                        }}
                                        disabled={hasMultipleSources}
                                        className='argo-button argo-button--base'>
                                        {hasMultipleSources && helpTip('Sources are not editable for applications with multiple sources. You can edit them in the "Manifest" tab.')}{' '}
                                        Edit
                                    </button>
                                )}
                                {isEditing && (
                                    <>
                                        <button disabled={isSaving} onClick={() => !isSaving && formApiRef.current?.submitForm(null)} className='argo-button argo-button--base'>
                                            <Spinner show={isSaving} style={{marginRight: '5px'}} />
                                            Save
                                        </button>{' '}
                                        <button onClick={handleCancel} className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button>
                                    </>
                                )}
                            </div>
                        )}
                        {collapsible && (
                            <div className='editable-panel__collapsible-button'>
                                <i className={`fa fa-angle-${isCollapsed ? 'down' : 'up'} filter__collapse`} onClick={() => setIsCollapsed(!isCollapsed)} />
                            </div>
                        )}
                        {title && <p>{title}</p>}
                        {!isEditing ? (
                            <>
                                {view}
                                {items
                                    .filter(content => {
                                        if (isSubsection(content)) {
                                            return content.items.some(item => item.view);
                                        }
                                        return content.view;
                                    })
                                    .map(content => renderContent(content))}
                            </>
                        ) : (
                            <Form
                                getApi={api => (formApiRef.current = api)}
                                formDidUpdate={async form => {
                                    if (noReadonlyMode && save) {
                                        await save(form.values as any, {});
                                    }
                                }}
                                onSubmit={handleSubmit}
                                defaultValues={values}
                                validateError={validate}>
                                {api => (
                                    <>
                                        {editProp && editProp(api)}
                                        {items.map(content => renderContent(content, api))}
                                    </>
                                )}
                            </Form>
                        )}
                    </div>
                </div>
            )}
        </>
    );
}

export {EditablePanel};
