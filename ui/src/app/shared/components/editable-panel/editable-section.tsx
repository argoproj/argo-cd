import {ErrorNotification, NotificationType} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
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

interface EditableSectionState {
    isEditing: boolean;
    isSaving: boolean;
}

// Similar to editable-panel but it should be part of a white-box, editable-panel HOC and it can be reused one after another
export class EditableSection<T = {}> extends React.Component<EditableSectionProps<T>, EditableSectionState> {
    private formApi: FormApi;

    constructor(props: EditableSectionProps<T>) {
        super(props);
        this.state = {isEditing: !!props.noReadonlyMode, isSaving: false};
    }

    public UNSAFE_componentWillReceiveProps(nextProps: EditableSectionProps<T>) {
        if (this.formApi && JSON.stringify(this.props.values) !== JSON.stringify(nextProps.values)) {
            if (nextProps.noReadonlyMode) {
                this.formApi.setAllValues(nextProps.values);
            }
        }
    }

    public render() {
        return (
            <div key={this.props.uniqueId}>
                {!this.props.noReadonlyMode && this.props.save && (
                    <div
                        key={this.props.uniqueId + '__panel__buttons'}
                        className={this.props.isTopSection ? 'editable-panel__buttons' : 'row white-box__details-row editable-panel__buttons-relative'}
                        style={{
                            top: this.props.isTopSection ? '25px' : '',
                            right: this.props.isTopSection ? (this.props.collapsible ? '4.5em' : '3.5em') : this.props.collapsible ? '1.0em' : '0em'
                        }}>
                        {!this.state.isEditing && (
                            <div className='editable-panel__buttons-relative-button'>
                                <button
                                    key={'edit_button_' + this.props.uniqueId}
                                    onClick={() => {
                                        this.setState({isEditing: true});
                                        this.props.updateButtons(true);
                                        this.props.onModeSwitch();
                                    }}
                                    disabled={this.props.disabledState}
                                    className='argo-button argo-button--base'>
                                    Edit
                                </button>{' '}
                                {this.props.isTopSection && this.props.deleteSource && (
                                    <button
                                        key={'delete_button_' + this.props.uniqueId}
                                        onClick={() => {
                                            this.props.deleteSource();
                                        }}
                                        disabled={this.props.disabledDelete}
                                        className='argo-button argo-button--base'>
                                        {helpTip('Delete the source from the sources field')}
                                        <span style={{marginRight: '8px'}} />
                                        Delete
                                    </button>
                                )}
                            </div>
                        )}
                        {this.state.isEditing && (
                            <div key={'buttons_' + this.props.uniqueId} className={!this.props.isTopSection ? 'editable-panel__buttons-relative-button' : ''}>
                                <React.Fragment key={'fragment_' + this.props.uniqueId}>
                                    <button
                                        key={'save_button_' + this.props.uniqueId}
                                        disabled={this.state.isSaving}
                                        onClick={() => !this.state.isSaving && this.formApi.submitForm(null)}
                                        className='argo-button argo-button--base'>
                                        <Spinner show={this.state.isSaving} style={{marginRight: '5px'}} />
                                        Save
                                    </button>{' '}
                                    <button
                                        key={'cancel_button_' + this.props.uniqueId}
                                        onClick={() => {
                                            this.setState({isEditing: false});
                                            this.props.updateButtons(false);
                                            this.props.onModeSwitch();
                                        }}
                                        className='argo-button argo-button--base-o'>
                                        Cancel
                                    </button>
                                </React.Fragment>
                            </div>
                        )}
                    </div>
                )}

                {this.props.title && (
                    <div className='row white-box__details-row'>
                        <p>{this.props.title}</p>
                    </div>
                )}

                {(!this.state.isEditing && (
                    <React.Fragment key={'read_' + this.props.uniqueId}>
                        {this.props.view}
                        {this.props.items
                            .filter(item => item.view)
                            .map(item => (
                                <React.Fragment key={'read_' + this.props.uniqueId + '_' + (item.key || item.title)}>
                                    {item.before}
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-3'>{item.customTitle || item.title}</div>
                                        <div className='columns small-9'>{item.view}</div>
                                    </div>
                                </React.Fragment>
                            ))}
                    </React.Fragment>
                )) || (
                    <Form
                        getApi={api => (this.formApi = api)}
                        formDidUpdate={async form => {
                            if (this.props.noReadonlyMode && this.props.save) {
                                await this.props.save(form.values as any, {});
                            }
                        }}
                        onSubmit={async input => {
                            try {
                                this.setState({isSaving: true});
                                await this.props.save(input as any, {});
                                this.setState({isEditing: false, isSaving: false});
                                this.props.onModeSwitch();
                            } catch (e) {
                                this.props.ctx.notifications.show({
                                    content: <ErrorNotification title='Unable to save changes' e={e} />,
                                    type: NotificationType.Error
                                });
                            } finally {
                                this.setState({isSaving: false});
                            }
                        }}
                        defaultValues={this.props.values}
                        validateError={this.props.validate}>
                        {api => (
                            <React.Fragment key={'edit_' + this.props.uniqueId}>
                                {this.props.edit && this.props.edit(api)}
                                {this.props.items?.map(item => (
                                    <React.Fragment key={'edit_' + this.props.uniqueId + '_' + (item.key || item.title)}>
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
        );
    }
}
