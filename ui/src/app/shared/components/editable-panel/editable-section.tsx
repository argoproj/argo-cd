import {ErrorNotification, NotificationType} from 'argo-ui';
import React, {useState, useRef, useEffect, Fragment, useCallback} from 'react';
import type {FormApi, FormState} from 'react-form';
import {Form} from 'react-form';
import {ContextApis} from '../../context';
import {EditablePanelItem} from './editable-panel';
import {Spinner} from '../spinner';
import {helpTip} from '../../../applications/components/utils';

export interface EditableSectionProps<T> {
    title?: string | React.ReactNode;
    uniqueId: string;
    values: T;
    validate?: (values: T) => any;
    save?: (input: T, query: {validate?: boolean}) => Promise<any>;
    items: EditablePanelItem[];
    onModeSwitch?: () => any;
    noReadonlyMode?: boolean;
    view?: string | React.ReactNode;
    edit?: (formApi: FormApi) => React.ReactNode;
    collapsible?: boolean;
    ctx: ContextApis;
    isTopSection?: boolean;
    disabledState?: boolean;
    disabledDelete?: boolean;
    updateButtons?: (pressed: boolean) => void;
    deleteSource?: () => void;
}

// Similar to editable-panel but it should be part of a white-box, editable-panel HOC and it can be reused one after another
function EditableSection<T extends {} = {}>({
    title,
    uniqueId,
    noReadonlyMode,
    save,
    isTopSection,
    collapsible,
    values,
    onModeSwitch,
    updateButtons,
    disabledState,
    deleteSource,
    disabledDelete,
    view,
    items,
    ctx,
    validate,
    edit
}: EditableSectionProps<T>) {
    const [isEditing, setIsEditing] = useState<boolean>(!!noReadonlyMode);
    const [isSaving, setIsSaving] = useState<boolean>(false);
    const formApiRef = useRef<FormApi | null>(null);
    const prevValuesRef = useRef<T>(values);

    useEffect(() => {
        if (formApiRef.current && JSON.stringify(prevValuesRef.current) !== JSON.stringify(values) && noReadonlyMode) {
            formApiRef.current.setAllValues(values);
        }
        prevValuesRef.current = values;
    }, [values, noReadonlyMode]);

    const onFormDidUpdate = useCallback(
        async (form: FormState) => {
            if (noReadonlyMode && save) {
                await save(form.values as any, {});
            }
        },
        [noReadonlyMode, save]
    );

    const onFormSubmit = useCallback<<K extends T>(data: K) => Promise<void>>(
        async input => {
            try {
                setIsSaving(true);
                await save(input as any, {});
                setIsEditing(false);
                onModeSwitch?.();
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title='Unable to save changes' e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setIsSaving(false);
            }
        },
        [save, ctx.notifications, onModeSwitch]
    );

    return (
        <div key={uniqueId}>
            {!noReadonlyMode && save && (
                <div
                    key={uniqueId + '__panel__buttons'}
                    className={isTopSection ? 'editable-panel__buttons' : 'row white-box__details-row editable-panel__buttons-relative'}
                    style={{
                        top: isTopSection ? '25px' : '',
                        right: isTopSection ? (collapsible ? '4.5em' : '3.5em') : collapsible ? '1.0em' : '0em'
                    }}>
                    {!isEditing ? (
                        <div className='editable-panel__buttons-relative-button'>
                            <button
                                key={'edit_button_' + uniqueId}
                                onClick={() => {
                                    setIsEditing(true);
                                    updateButtons?.(true);
                                    onModeSwitch?.();
                                }}
                                disabled={disabledState}
                                className='argo-button argo-button--base'>
                                Edit
                            </button>{' '}
                            {isTopSection && deleteSource && (
                                <button
                                    key={'delete_button_' + uniqueId}
                                    onClick={() => {
                                        deleteSource();
                                    }}
                                    disabled={disabledDelete}
                                    className='argo-button argo-button--base'>
                                    {helpTip('Delete the source from the sources field')}
                                    <span style={{marginRight: '8px'}} />
                                    Delete
                                </button>
                            )}
                        </div>
                    ) : (
                        <div key={'buttons_' + uniqueId} className={!isTopSection ? 'editable-panel__buttons-relative-button' : ''}>
                            <Fragment key={'fragment_' + uniqueId}>
                                <button
                                    key={'save_button_' + uniqueId}
                                    disabled={isSaving}
                                    onClick={() => !isSaving && formApiRef.current?.submitForm(null)}
                                    className='argo-button argo-button--base'>
                                    <Spinner show={isSaving} style={{marginRight: '5px'}} />
                                    Save
                                </button>{' '}
                                <button
                                    key={'cancel_button_' + uniqueId}
                                    onClick={() => {
                                        setIsEditing(false);
                                        updateButtons?.(false);
                                        onModeSwitch?.();
                                    }}
                                    className='argo-button argo-button--base-o'>
                                    Cancel
                                </button>
                            </Fragment>
                        </div>
                    )}
                </div>
            )}
            {title && (
                <div className='row white-box__details-row'>
                    <p>{title}</p>
                </div>
            )}
            {(!isEditing && (
                <Fragment key={'read_' + uniqueId}>
                    {view}
                    {items
                        .filter(item => item.view)
                        .map(item => (
                            <Fragment key={'read_' + uniqueId + '_' + (item.key || item.title)}>
                                {item.before}
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>{item.customTitle || item.title}</div>
                                    <div className='columns small-9'>{item.view}</div>
                                </div>
                            </Fragment>
                        ))}
                </Fragment>
            )) || (
                <Form getApi={api => (formApiRef.current = api)} formDidUpdate={onFormDidUpdate} onSubmit={onFormSubmit} defaultValues={values} validateError={validate}>
                    {api => (
                        <Fragment key={'edit_' + uniqueId}>
                            {edit && edit(api)}
                            {items.map(item => (
                                <Fragment key={'edit_' + uniqueId + '_' + (item.key || item.title)}>
                                    {item.before}
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>{(item.titleEdit && item.titleEdit(api)) || item.customTitle || item.title}</div>
                                        <div className='columns small-9'>{(item.edit && item.edit(api)) || item.view}</div>
                                    </div>
                                </Fragment>
                            ))}
                        </Fragment>
                    )}
                </Form>
            )}
        </div>
    );
}

export {EditableSection};
