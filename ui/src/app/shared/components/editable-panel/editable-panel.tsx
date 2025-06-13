import {ErrorNotification, NotificationType} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
import {helpTip} from '../../../applications/components/utils';
import {Consumer} from '../../context';
import {Spinner} from '../spinner';

import './editable-panel.scss';

export interface EditablePanelItem {
    title: string;
    customTitle?: string | React.ReactNode;
    key?: string;
    before?: React.ReactNode;
    view: string | React.ReactNode;
    edit?: (formApi: FormApi) => React.ReactNode;
    titleEdit?: (formApi: FormApi) => React.ReactNode;
}

export interface EditablePanelProps<T> {
    title?: string | React.ReactNode;
    titleCollapsed?: string | React.ReactNode;
    floatingTitle?: string | React.ReactNode;
    values: T;
    validate?: (values: T) => any;
    save?: (input: T, query: {validate?: boolean}) => Promise<any>;
    items: EditablePanelItem[];
    onModeSwitch?: () => any;
    noReadonlyMode?: boolean;
    view?: string | React.ReactNode;
    edit?: (formApi: FormApi) => React.ReactNode;
    hasMultipleSources?: boolean;
    collapsible?: boolean;
    collapsed?: boolean;
    collapsedDescription?: string;
}

export const EditablePanel = React.memo(
    <T extends {} = {}>({
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
    }: EditablePanelProps<T>) => {
        const [isEditing, setIsEditing] = React.useState<boolean>(!!noReadonlyMode);
        const [isSaving, setIsSaving] = React.useState<boolean>(false);
        const [isCollapsed, setIsCollapsed] = React.useState<boolean>(collapsedProp);
        const formApiRef = React.useRef<FormApi | null>(null);
        const initialValuesRef = React.useRef<T>(values);

        React.useEffect(() => {
            setIsCollapsed(collapsedProp);
        }, [collapsedProp]);

        React.useEffect(() => {
            if (formApiRef.current && JSON.stringify(initialValuesRef.current) !== JSON.stringify(values)) {
                if (noReadonlyMode) {
                    formApiRef.current.setAllValues(values);
                }
                initialValuesRef.current = values;
            } else if (JSON.stringify(initialValuesRef.current) !== JSON.stringify(values)) {
                initialValuesRef.current = values;
            }
        }, [values, noReadonlyMode]);

        const onModeSwitch = () => {
            if (onModeSwitchProp) {
                onModeSwitchProp();
            }
        };

        const handleSave = async (input: T) => {
            if (!save) return;
            try {
                setIsSaving(true);
                await save(input, {});
                setIsEditing(false);
                setIsSaving(false);
                onModeSwitch();
            } catch (e) {
                console.error('Save failed:', e);
                throw e;
            } finally {
                if (setIsSaving) {
                    setIsSaving(false);
                }
            }
        };

        const handleCancel = () => {
            setIsEditing(false);
            onModeSwitch();
        };

        return (
            <Consumer>
                {ctx =>
                    collapsible && isCollapsed ? (
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
                                                disabled={hasMultipleSources || isSaving}
                                                className='argo-button argo-button--base'>
                                                {hasMultipleSources &&
                                                    helpTip('Sources are not editable for applications with multiple sources. You can edit them in the "Manifest" tab.')}{' '}
                                                Edit
                                            </button>
                                        )}
                                        {isEditing && (
                                            <React.Fragment>
                                                <button
                                                    disabled={isSaving}
                                                    onClick={() => !isSaving && formApiRef.current?.submitForm(null)}
                                                    className='argo-button argo-button--base'>
                                                    <Spinner show={isSaving} style={{marginRight: '5px'}} />
                                                    Save
                                                </button>{' '}
                                                <button onClick={handleCancel} disabled={isSaving} className='argo-button argo-button--base-o'>
                                                    Cancel
                                                </button>
                                            </React.Fragment>
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
                                    <React.Fragment>
                                        {view}
                                        {items
                                            .filter(item => item.view)
                                            .map(item => (
                                                <React.Fragment key={item.key || item.title}>
                                                    {item.before}
                                                    <div className='row white-box__details-row'>
                                                        <div className='columns small-3'>{item.customTitle || item.title}</div>
                                                        <div className='columns small-9'>{item.view}</div>
                                                    </div>
                                                </React.Fragment>
                                            ))}
                                    </React.Fragment>
                                ) : (
                                    <Form
                                        getApi={api => (formApiRef.current = api)}
                                        formDidUpdate={async form => {
                                            if (noReadonlyMode && save) {
                                                await save(form.values as any, {});
                                            }
                                        }}
                                        onSubmit={async (input: T) => {
                                            try {
                                                await handleSave(input);
                                            } catch (e) {
                                                ctx.notifications.show({
                                                    content: <ErrorNotification title='Unable to save changes' e={e} />,
                                                    type: NotificationType.Error
                                                });
                                            }
                                        }}
                                        defaultValues={values}
                                        validateError={validate}>
                                        {api => (
                                            <React.Fragment>
                                                {editProp && editProp(api)}
                                                {items.map(item => (
                                                    <React.Fragment key={item.key || item.title}>
                                                        {item.before}
                                                        <div className='row white-box__details-row'>
                                                            <div className='columns small-3'>{(item.titleEdit && item.titleEdit(api)) || item.customTitle || item.title}</div>
                                                            <div className='columns small-9'>{(item.edit && item.edit(api)) || item.view}</div>
                                                        </div>
                                                    </React.Fragment>
                                                ))}
                                            </React.Fragment>
                                        )}
                                    </Form>
                                )}
                            </div>
                        </div>
                    )
                }
            </Consumer>
        );
    }
);
