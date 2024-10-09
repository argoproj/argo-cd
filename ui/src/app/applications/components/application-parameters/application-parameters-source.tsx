import * as classNames from 'classnames';
import * as React from 'react';
import {FormApi} from 'react-form';
import {EditablePanelItem} from '../../../shared/components';
import {EditableSection} from '../../../shared/components/editable-panel/editable-section';
import {Consumer} from '../../../shared/context';
import '../../../shared/components/editable-panel/editable-panel.scss';

export interface ApplicationParametersPanelProps<T> {
    floatingTitle?: string | React.ReactNode;
    titleTop?: string | React.ReactNode;
    titleBottom?: string | React.ReactNode;
    index: number;
    valuesTop?: T;
    valuesBottom?: T;
    validateTop?: (values: T) => any;
    validateBottom?: (values: T) => any;
    saveTop?: (input: T, query: {validate?: boolean}) => Promise<any>;
    saveBottom?: (input: T, query: {validate?: boolean}) => Promise<any>;
    itemsTop?: EditablePanelItem[];
    itemsBottom?: EditablePanelItem[];
    onModeSwitch?: () => any;
    viewTop?: string | React.ReactNode;
    viewBottom?: string | React.ReactNode;
    editTop?: (formApi: FormApi) => React.ReactNode;
    editBottom?: (formApi: FormApi) => React.ReactNode;
    numberOfSources?: number;
    noReadonlyMode?: boolean;
    collapsible?: boolean;
    deleteSource: () => void;
}

interface ApplicationParametersPanelState {
    editTop: boolean;
    editBottom: boolean;
    savingTop: boolean;
    savingBottom: boolean;
}

// Currently two editable sections, but can be modified to support N panels in general.  This should be part of a white-box, editable-panel.
export class ApplicationParametersSource<T = {}> extends React.Component<ApplicationParametersPanelProps<T>, ApplicationParametersPanelState> {
    constructor(props: ApplicationParametersPanelProps<T>) {
        super(props);
        this.state = {editTop: !!props.noReadonlyMode, editBottom: !!props.noReadonlyMode, savingTop: false, savingBottom: false};
    }

    public render() {
        return (
            <Consumer>
                {ctx => (
                    <div className={classNames({'editable-panel--disabled': this.state.savingTop})}>
                        {this.props.floatingTitle && <div className='white-box--additional-top-space editable-panel__sticky-title'>{this.props.floatingTitle}</div>}
                        <React.Fragment>
                            <EditableSection
                                uniqueId={'top_' + this.props.index}
                                title={this.props.titleTop}
                                view={this.props.viewTop}
                                values={this.props.valuesTop}
                                items={this.props.itemsTop}
                                validate={this.props.validateTop}
                                save={this.props.saveTop}
                                onModeSwitch={() => this.onModeSwitch()}
                                noReadonlyMode={this.props.noReadonlyMode}
                                edit={this.props.editTop}
                                collapsible={this.props.collapsible}
                                ctx={ctx}
                                isTopSection={true}
                                disabledState={this.state.editTop || this.state.editTop === null}
                                disabledDelete={this.props.numberOfSources <= 1}
                                updateButtons={editClicked => {
                                    this.setState({editBottom: editClicked});
                                }}
                                deleteSource={this.props.deleteSource}
                            />
                        </React.Fragment>
                        {this.props.itemsTop && (
                            <React.Fragment>
                                <div className='row white-box__details-row'>
                                    <p>&nbsp;</p>
                                </div>
                                <div className='white-box--no-padding editable-panel__divider' />
                            </React.Fragment>
                        )}
                        <React.Fragment>
                            <EditableSection
                                uniqueId={'bottom_' + this.props.index}
                                title={this.props.titleBottom}
                                view={this.props.viewBottom}
                                values={this.props.valuesBottom}
                                items={this.props.itemsBottom}
                                validate={this.props.validateBottom}
                                save={this.props.saveBottom}
                                onModeSwitch={() => this.onModeSwitch()}
                                noReadonlyMode={this.props.noReadonlyMode}
                                edit={this.props.editBottom}
                                collapsible={this.props.collapsible}
                                ctx={ctx}
                                isTopSection={false}
                                disabledState={this.state.editBottom || this.state.editBottom === null}
                                updateButtons={editClicked => {
                                    this.setState({editTop: editClicked});
                                }}
                            />
                        </React.Fragment>
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
