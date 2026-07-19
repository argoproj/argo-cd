import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import {MonacoEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {AppSetGeneratedAppsDiff} from './appset-generated-apps-diff';

interface AppSetPreviewTabProps {
    appSet: models.ApplicationSet;
}

export const AppSetPreviewTab = (props: AppSetPreviewTabProps) => {
    const {appSet} = props;
    const initialYaml = React.useMemo(() => jsYaml.dump(appSet.spec || {}), [appSet]);
    const modelRef = useRef<monacoEditor.editor.ITextModel | null>(null);
    const resultRef = useRef<HTMLDivElement | null>(null);
    const [editing, setEditing] = useState(false);
    const [parseError, setParseError] = useState<string | null>(null);
    const [editorKey, setEditorKey] = useState(0);
    const [proposedAppSet, setProposedAppSet] = useState<models.ApplicationSet | null>(null);
    const [trigger, setTrigger] = useState(0);

    useEffect(() => {
        if ((proposedAppSet || parseError) && resultRef.current) {
            resultRef.current.scrollIntoView({behavior: 'smooth', block: 'start'});
        }
    }, [proposedAppSet, parseError]);

    const readEditorYaml = (): string => {
        if (modelRef.current) {
            return modelRef.current.getLinesContent().join('\n');
        }
        return initialYaml;
    };

    const onPreview = () => {
        setParseError(null);
        let parsedSpec: any;
        try {
            parsedSpec = jsYaml.load(readEditorYaml());
        } catch (e: any) {
            setParseError(`Invalid YAML: ${e.message || e}`);
            setProposedAppSet(null);
            return;
        }
        if (!parsedSpec || typeof parsedSpec !== 'object') {
            setParseError('Invalid YAML: spec must be an object');
            setProposedAppSet(null);
            return;
        }
        setProposedAppSet({...appSet, spec: parsedSpec});
        setTrigger(t => t + 1);
    };

    const onEdit = () => {
        setEditing(true);
    };

    const onCancelEdit = () => {
        setEditing(false);
        setEditorKey(k => k + 1);
    };

    const spec = appSet.spec as any;
    const project = spec?.template?.spec?.project || 'unknown';

    return (
        <div className='applicationset-preview' style={{padding: '15px'}}>
            <div style={{marginBottom: '15px', fontSize: '13px', color: '#6d7f8b'}}>
                <i className='fa fa-info-circle' style={{marginRight: '6px'}} />
                Preview what Applications your ApplicationSet will generate. The diff compares your proposed ApplicationSet's output to the current ApplicationSet's output.
            </div>
            <div className='white-box'>
                <div className='white-box__details'>
                    <div style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between'}}>
                        <p style={{margin: 0}}>APPLICATIONSET MANIFEST{editing && ' (edits are not saved)'}</p>
                        <div>
                            {editing ? (
                                <>
                                    <button className='argo-button argo-button--base' onClick={onPreview}>
                                        Preview
                                    </button>{' '}
                                    <button className='argo-button argo-button--base-o' onClick={onCancelEdit}>
                                        Cancel
                                    </button>
                                </>
                            ) : (
                                <>
                                    <button className='argo-button argo-button--base' onClick={onPreview}>
                                        Preview
                                    </button>{' '}
                                    <button className='argo-button argo-button--base-o' onClick={onEdit}>
                                        Edit
                                    </button>
                                </>
                            )}
                        </div>
                    </div>
                    <div style={{marginTop: '10px'}}>
                        <MonacoEditor
                            key={editorKey}
                            minHeight={800}
                            vScrollBar={false}
                            editor={{
                                input: {text: initialYaml, language: 'yaml'},
                                options: {readOnly: !editing, minimap: {enabled: false}, wordWrap: 'on'},
                                getApi: api => {
                                    modelRef.current = api.getModel() as monacoEditor.editor.ITextModel;
                                }
                            }}
                        />
                    </div>
                </div>
            </div>

            {parseError && (
                <div ref={resultRef} className='white-box' style={{marginTop: '15px'}}>
                    <div className='white-box__details'>
                        <p>PREVIEW FAILED</p>
                        <pre style={{whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: '400px', overflow: 'auto'}}>{parseError}</pre>
                    </div>
                </div>
            )}

            {proposedAppSet && !parseError && (
                <div ref={resultRef}>
                    <AppSetGeneratedAppsDiff currentAppSet={appSet} proposedAppSet={proposedAppSet} trigger={trigger} rbacProject={project} />
                </div>
            )}
        </div>
    );
};
