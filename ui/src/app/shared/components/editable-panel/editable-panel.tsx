import {ErrorNotification, NotificationType} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {Form, FormApi} from 'react-form';

import {Consumer} from '../../context';
import {Spinner} from '../spinner';

export interface EditablePanelItem {
    title: string;
    key?: string;
    before?: React.ReactNode;
    view: string | React.ReactNode;
    edit?: (formApi: FormApi) => React.ReactNode;
    titleEdit?: (formApi: FormApi) => React.ReactNode;
}

export interface EditablePanelProps<T> {
    title?: string | React.ReactNode;
    values: T;
    validate?: (values: T) => any;
    save?: (input: T) => Promise<any>;
    items: EditablePanelItem[];
    onModeSwitch?: () => any;
    noReadonlyMode?: boolean;
    view?: string | React.ReactNode;
    edit?: (formApi: FormApi) => React.ReactNode;
}

interface EditablePanelState {
    edit: boolean;
    saving: boolean;
}

require('./editable-panel.scss');

export class EditablePanel<T = {}> extends React.Component<EditablePanelProps<T>, EditablePanelState> {
    private formApi: FormApi;

    constructor(props: EditablePanelProps<T>) {
        super(props);
        this.state = {edit: !!props.noReadonlyMode, saving: false};
    }

    public UNSAFE_componentWillReceiveProps(nextProps: EditablePanelProps<T>) {
        if (this.formApi && JSON.stringify(this.props.values) !== JSON.stringify(nextProps.values)) {
            if (!!nextProps.noReadonlyMode) {
                this.formApi.setAllValues(nextProps.values);
            }
        }
    }

    public render() {
        return (
            <Consumer>
                {ctx => (
                    <div className={classNames('white-box editable-panel', {'editable-panel--disabled': this.state.saving})}>
                        <div className='white-box__details'>
                            {!this.props.noReadonlyMode && this.props.save && (
                                <div className='editable-panel__buttons'>
                                    {!this.state.edit && (
                                        <button
                                            onClick={() => {
                                                this.setState({edit: true});
                                                this.onModeSwitch();
                                            }}
                                            className='argo-button argo-button--base'>
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
                            {this.props.title && <p>{this.props.title}</p>}
                            {(!this.state.edit && (
                                <React.Fragment>
                                    {this.props.view}
                                    {this.props.items.map(item => (
                                        <React.Fragment key={item.key || item.title}>
                                            {item.before}
                                            <div className='row white-box__details-row'>
                                                <div className='columns small-3'>{item.title}</div>
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
                                            await this.props.save(form.values as any);
                                        }
                                    }}
                                    onSubmit={async input => {
                                        try {
                                            this.setState({saving: true});
                                            await this.props.save(input as any);
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
                                                        <div className='columns small-3'>{(item.titleEdit && item.titleEdit(api)) || item.title}</div>
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
                )}
            </Consumer>
        );
    }

    private onModeSwitch() {
        if (this.props.onModeSwitch) {
            this.props.onModeSwitch();
        }
    }
}
