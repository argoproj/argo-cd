import {ErrorNotification, NotificationType} from 'argo-ui';
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
    key?: string;
    before?: ReactNode;
    view: string | ReactNode;
    edit?: (formApi: FormApi) => ReactNode;
    titleEdit?: (formApi: FormApi) => ReactNode;
}

export interface EditablePanelProps<T> {
    title?: string | ReactNode;
    titleCollapsed?: string | ReactNode;
    floatingTitle?: string | ReactNode;
    values: T;
    validate?: (values: T) => any;
    save?: (input: T, query: {validate?: boolean}) => Promise<any>;
    items: EditablePanelItem[];
    onModeSwitch?: () => any;
    noReadonlyMode?: boolean;
    view?: string | ReactNode;
    edit?: (formApi: FormApi) => ReactNode;
    hasMultipleSources?: boolean;
    collapsible?: boolean;
    collapsed?: boolean;
    collapsedDescription?: string;
    registered?: boolean;
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
    collapsedDescription,
    registered
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
                        {registered !== false && !noReadonlyMode && save && (
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
                                    .filter(item => item.view)
                                    .map(item => (
                                        <Fragment key={item.key || item.title}>
                                            {item.before}
                                            <div className='row white-box__details-row'>
                                                <div className='columns small-3'>{item.customTitle || item.title}</div>
                                                <div className='columns small-9'>{item.view}</div>
                                            </div>
                                        </Fragment>
                                    ))}
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
                                        {items.map(item => (
                                            <Fragment key={item.key || item.title}>
                                                {item.before}
                                                <div className='row white-box__details-row'>
                                                    <div className='columns small-3'>{(item.titleEdit && item.titleEdit(api)) || item.customTitle || item.title}</div>
                                                    <div className='columns small-9'>{(item.edit && item.edit(api)) || item.view}</div>
                                                </div>
                                            </Fragment>
                                        ))}
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
