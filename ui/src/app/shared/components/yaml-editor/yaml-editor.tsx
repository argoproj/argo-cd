import {ErrorNotification, NotificationType} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';

import {Consumer} from '../../context';
import {MonacoEditor} from '../monaco-editor';
import {safeYamlDump} from '../../utils/yaml-performance';

const jsonMergePatch = require('json-merge-patch');
require('./yaml-editor.scss');

export class YamlEditor<T> extends React.Component<
    {
        input: T;
        hideModeButtons?: boolean;
        initialEditMode?: boolean;
        vScrollbar?: boolean;
        enableWordWrap?: boolean;
        onSave?: (patch: string, patchType: string) => Promise<any>;
        onCancel?: () => any;
        minHeight?: number;
    },
    {
        editing: boolean;
    }
> {
    private model: monacoEditor.editor.ITextModel;

    constructor(props: any) {
        super(props);
        this.state = {editing: props.initialEditMode};
    }

    public render() {
        const props = this.props;
        const {yaml, info} = props.input ? safeYamlDump(props.input) : {yaml: '', info: null};

        return (
            <div className='yaml-editor'>
                {info?.warningMessage && (
                    <div className='yaml-editor__warning'>
                        <i className='fa fa-exclamation-triangle' />
                        <span>{info.warningMessage}</span>
                    </div>
                )}

                {!props.hideModeButtons && (
                    <div className='yaml-editor__buttons'>
                        {(this.state.editing && (
                            <Consumer>
                                {ctx => (
                                    <React.Fragment>
                                        <button
                                            onClick={async () => {
                                                try {
                                                    const updated = jsYaml.load(this.model.getLinesContent().join('\n'));
                                                    const patch = jsonMergePatch.generate(props.input, updated);
                                                    try {
                                                        const unmounted = await this.props.onSave(JSON.stringify(patch || {}), 'application/merge-patch+json');
                                                        if (unmounted !== true) {
                                                            this.setState({editing: false});
                                                        }
                                                    } catch (e) {
                                                        ctx.notifications.show({
                                                            content: (
                                                                <div className='yaml-editor__error'>
                                                                    <ErrorNotification title='Unable to save changes' e={e} />
                                                                </div>
                                                            ),
                                                            type: NotificationType.Error
                                                        });
                                                    }
                                                } catch (e) {
                                                    ctx.notifications.show({
                                                        content: <ErrorNotification title='Unable to validate changes' e={e} />,
                                                        type: NotificationType.Error
                                                    });
                                                }
                                            }}
                                            className='argo-button argo-button--base'>
                                            Save
                                        </button>{' '}
                                        <button
                                            onClick={() => {
                                                const {yaml: freshYaml} = safeYamlDump(props.input);
                                                this.model.setValue(freshYaml);
                                                this.setState({editing: !this.state.editing});
                                                if (props.onCancel) {
                                                    props.onCancel();
                                                }
                                            }}
                                            className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button>
                                    </React.Fragment>
                                )}
                            </Consumer>
                        )) || (
                            <button onClick={() => this.setState({editing: true})} className='argo-button argo-button--base'>
                                Edit
                            </button>
                        )}
                    </div>
                )}
                <MonacoEditor
                    minHeight={props.minHeight}
                    vScrollBar={props.vScrollbar}
                    editor={{
                        input: {text: yaml, language: 'yaml'},
                        options: {
                            readOnly: !this.state.editing,
                            minimap: {enabled: false},
                            wordWrap: props.enableWordWrap ? 'on' : 'off',
                            // Performance optimizations for large files
                            renderWhitespace: info?.isLarge ? 'none' : 'selection',
                            renderLineHighlight: info?.isLarge ? 'none' : 'line',
                            occurrencesHighlight: info?.isLarge ? false : true,
                            selectionHighlight: info?.isLarge ? false : true,
                            folding: info?.isLarge ? false : true,
                            foldingHighlight: info?.isLarge ? false : true
                        },
                        getApi: api => {
                            this.model = api.getModel() as monacoEditor.editor.ITextModel;
                        }
                    }}
                />
            </div>
        );
    }
}
