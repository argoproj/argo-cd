import {ErrorNotification, NotificationType} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
import {helpTip} from '../../../applications/components/utils';
import {Consumer} from '../../context';
import {Spinner} from '../spinner';

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

interface EditablePanelState {
    edit: boolean;
    saving: boolean;
    collapsed: boolean;
}

require('./editable-panel.scss');

export class EditablePanel<T = {}> extends React.Component<EditablePanelProps<T>, EditablePanelState> {
    private formApi: FormApi;

    constructor(props: EditablePanelProps<T>) {
        super(props);
        this.state = {edit: !!props.noReadonlyMode, saving: false, collapsed: this.props.collapsed};
    }

    public UNSAFE_componentWillReceiveProps(nextProps: EditablePanelProps<T>) {
        if (this.formApi && JSON.stringify(this.props.values) !== JSON.stringify(nextProps.values)) {
            if (nextProps.noReadonlyMode) {
                this.formApi.setAllValues(nextProps.values);
            }
        }
    }

    public render() {
        return (
            <Consumer>
                {ctx =>
                    this.props.collapsible && this.state.collapsed ? (
                        <div className='settings-overview__redirect-panel' style={{marginTop: 0}} onClick={() => this.setState({collapsed: !this.state.collapsed})}>
                            <div className='settings-overview__redirect-panel__content'>
                                <div className='settings-overview__redirect-panel__title'>{this.props.titleCollapsed ? this.props.titleCollapsed : this.props.title}</div>
                                <div className='settings-overview__redirect-panel__description'>{this.props.collapsedDescription}</div>
                            </div>
                            <div className='settings-overview__redirect-panel__arrow'>
                                <i className='fa fa-angle-down' />
                            </div>
                        </div>
                    ) : (
                        <div className={classNames('white-box editable-panel', {'editable-panel--disabled': this.state.saving})}>
                            {this.props.floatingTitle && <div className='white-box--additional-top-space editable-panel__sticky-title'>{this.props.floatingTitle}</div>}
                            <div className='white-box__details'>
                                {!this.props.noReadonlyMode && this.props.save && (
                                    <div className='editable-panel__buttons' style={{right: this.props.collapsible ? '5em' : '3em'}}>
                                        {!this.state.edit && (
                                            <button
                                                onClick={() => {
                                                    this.setState({edit: true});
                                                    this.onModeSwitch();
                                                }}
                                                disabled={this.props.hasMultipleSources}
                                                className='argo-button argo-button--base'>
                                                {this.props.hasMultipleSources &&
                                                    helpTip('Sources are not editable for applications with multiple sources. You can edit them in the "Manifest" tab.')}{' '}
                                                Edit
                                            </button>
                                        )}
                                        {this.state.edit && (
                                            <React.Fragment>
                                                <button
                                                    disabled={this.state.saving}
                                                    onClick={() => !this.state.saving && this.formApi.submitForm(null)}
                                                    className='argo-button argo-button--base'>
                                                    <Spinner show={this.state.saving} style={{marginRight: '5px'}} />
                                                    Save
                                                </button>{' '}
                                                <button
                                                    onClick={() => {
                                                        this.setState({edit: false});
                                                        this.onModeSwitch();
                                                    }}
                                                    className='argo-button argo-button--base-o'>
                                                    Cancel
                                                </button>
                                            </React.Fragment>
                                        )}
                                    </div>
                                )}
                                {this.props.collapsible && (
                                    <React.Fragment>
                                        <div className='editable-panel__collapsible-button'>
                                            <i
                                                className={`fa fa-angle-${this.state.collapsed ? 'down' : 'up'} filter__collapse`}
                                                onClick={() => {
                                                    this.setState({collapsed: !this.state.collapsed});
                                                }}
                                            />
                                        </div>
                                    </React.Fragment>
                                )}
                                {this.props.title && <p>{this.props.title}</p>}
                                {(!this.state.edit && (
                                    <React.Fragment>
                                        {this.props.view}
                                        {this.props.items
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
                                                this.setState({saving: true});
                                                await this.props.save(input as any, {});
                                                this.setState({edit: false, saving: false});
                                                this.onModeSwitch();
                                            } catch (e) {
                                                ctx.notifications.show({
                                                    content: <ErrorNotification title='Unable to save changes' e={e} />,
                                                    type: NotificationType.Error
                                                });
                                            } finally {
                                                this.setState({saving: false});
                                            }
                                        }}
                                        defaultValues={this.props.values}
                                        validateError={this.props.validate}>
                                        {api => (
                                            <React.Fragment>
                                                {this.props.edit && this.props.edit(api)}
                                                {this.props.items.map(item => (
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

    private onModeSwitch() {
        if (this.props.onModeSwitch) {
            this.props.onModeSwitch();
        }
    }
}
