import {ErrorNotification, NotificationType} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';

import {Consumer} from '../../context';
import {MonacoEditor} from '../monaco-editor';

const jsonMergePatch = require('json-merge-patch');
require('./yaml-editor.scss');

export class YamlEditor<T> extends React.Component<
    {
        input: T;
        hideModeButtons?: boolean;
        initialEditMode?: boolean;
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
        const yaml = props.input ? jsYaml.safeDump(props.input) : '';

        return (
            <div className='yaml-editor'>
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
                                                    const unmounted = await this.props.onSave(JSON.stringify(patch || {}), 'application/merge-patch+json');
                                                    if (unmounted !== true) {
                                                        this.setState({editing: false});
                                                    }
                                                } catch (e) {
                                                    ctx.notifications.show({
                                                        content: <ErrorNotification title='Unable to save changes' e={e} />,
                                                        type: NotificationType.Error
                                                    });
                                                }
                                            }}
                                            className='argo-button argo-button--base'>
                                            Save
                                        </button>{' '}
                                        <button
                                            onClick={() => {
                                                this.model.setValue(jsYaml.safeDump(props.input));
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
                    editor={{
                        input: {text: yaml, language: 'yaml'},
                        options: {readOnly: !this.state.editing, minimap: {enabled: false}},
                        getApi: api => {
                            this.model = api.getModel() as monacoEditor.editor.ITextModel;
                        }
                    }}
                />
            </div>
        );
    }
}
