import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import {MonacoEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

interface AppSetPreviewTabProps {
    appSet: models.ApplicationSet;
}

const VOLATILE_METADATA_FIELDS = ['managedFields', 'uid', 'resourceVersion', 'generation', 'creationTimestamp', 'selfLink'];
const NOISY_ANNOTATIONS = ['kubectl.kubernetes.io/last-applied-configuration'];

function cleanManifest(obj: any): any {
    const copy: any = JSON.parse(JSON.stringify(obj));
    if (copy?.metadata) {
        for (const field of VOLATILE_METADATA_FIELDS) {
            delete copy.metadata[field];
        }
        if (copy.metadata.annotations) {
            for (const ann of NOISY_ANNOTATIONS) {
                delete copy.metadata.annotations[ann];
            }
            if (Object.keys(copy.metadata.annotations).length === 0) {
                delete copy.metadata.annotations;
            }
        }
    }
    delete copy.status;
    return {
        apiVersion: copy.apiVersion,
        kind: copy.kind,
        metadata: copy.metadata,
        spec: copy.spec
    };
}

function extractErrorMessage(err: any): string {
    if (err?.response?.body?.message) {
        return err.response.body.message;
    }
    if (err?.response?.body?.error) {
        return err.response.body.error;
    }
    if (err?.message) {
        return err.message;
    }
    return 'Preview failed';
}

function isPermissionDeniedError(err: any): boolean {
    if (err?.response?.status === 403) {
        return true;
    }
    const msg = extractErrorMessage(err).toLowerCase();
    return msg.includes('permission denied') || msg.includes('permissiondenied');
}

const GeneratedAppCard = ({app}: {app: models.Application}) => {
    const [expanded, setExpanded] = useState(false);
    const yaml = React.useMemo(() => jsYaml.dump(cleanManifest(app)), [app]);
    const name = app.metadata?.name || '(unnamed)';
    const namespace = app.metadata?.namespace || '';

    return (
        <div className='white-box' style={{marginTop: '10px'}}>
            <div className='white-box__details'>
                <div style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between'}}>
                    <strong>{namespace ? `${namespace}/${name}` : name}</strong>
                    <button className='argo-button argo-button--base-o' onClick={() => setExpanded(e => !e)}>
                        {expanded ? 'Hide YAML' : 'Show YAML'}
                    </button>
                </div>
                {expanded && (
                    <div style={{marginTop: '10px'}}>
                        <MonacoEditor
                            vScrollBar={false}
                            editor={{
                                input: {text: yaml, language: 'yaml'},
                                options: {readOnly: true, minimap: {enabled: false}, wordWrap: 'on'}
                            }}
                        />
                    </div>
                )}
            </div>
        </div>
    );
};

export const AppSetPreviewTab = (props: AppSetPreviewTabProps) => {
    const {appSet} = props;
    const initialYaml = React.useMemo(() => jsYaml.dump(cleanManifest(appSet)), [appSet]);
    const modelRef = useRef<monacoEditor.editor.ITextModel | null>(null);
    const resultRef = useRef<HTMLDivElement | null>(null);
    const [editing, setEditing] = useState(false);
    const [previewing, setPreviewing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [permissionDenied, setPermissionDenied] = useState(false);
    const [generated, setGenerated] = useState<models.Application[] | null>(null);
    const [editorKey, setEditorKey] = useState(0);

    useEffect(() => {
        if ((generated || error) && resultRef.current) {
            resultRef.current.scrollIntoView({behavior: 'smooth', block: 'start'});
        }
    }, [generated, error]);

    const readEditorYaml = (): string => {
        if (modelRef.current) {
            return modelRef.current.getLinesContent().join('\n');
        }
        return initialYaml;
    };

    const onPreview = async () => {
        setError(null);
        setPermissionDenied(false);
        let parsed: models.ApplicationSet;
        try {
            parsed = jsYaml.load(readEditorYaml()) as models.ApplicationSet;
        } catch (e: any) {
            setError(`Invalid YAML: ${e.message || e}`);
            return;
        }
        if (!parsed || !parsed.metadata) {
            setError('Invalid YAML: missing ApplicationSet metadata');
            return;
        }
        setPreviewing(true);
        try {
            const apps = await services.applications.appSetGenerate(parsed);
            setGenerated(apps);
        } catch (e: any) {
            if (isPermissionDeniedError(e)) {
                setPermissionDenied(true);
            }
            setError(extractErrorMessage(e));
            setGenerated(null);
        } finally {
            setPreviewing(false);
        }
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
            <div className='white-box'>
                <div className='white-box__details'>
                    <div style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between'}}>
                        <p style={{margin: 0}}>APPLICATIONSET MANIFEST{editing && ' (edits are not saved)'}</p>
                        <div>
                            {editing ? (
                                <>
                                    <button className='argo-button argo-button--base' onClick={onPreview} disabled={previewing}>
                                        {previewing ? 'Previewing...' : 'Preview'}
                                    </button>{' '}
                                    <button className='argo-button argo-button--base-o' onClick={onCancelEdit} disabled={previewing}>
                                        Cancel
                                    </button>
                                </>
                            ) : (
                                <>
                                    <button className='argo-button argo-button--base' onClick={onPreview} disabled={previewing}>
                                        {previewing ? 'Previewing...' : 'Preview'}
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

            {error && (
                <div ref={resultRef} className='white-box' style={{marginTop: '15px'}}>
                    <div className='white-box__details'>
                        {permissionDenied ? (
                            <>
                                <p>PERMISSION DENIED</p>
                                <div>
                                    You need <code>applicationsets, create</code> permission on project <code>{project}</code> to preview generated Applications.
                                </div>
                            </>
                        ) : (
                            <>
                                <p>PREVIEW FAILED</p>
                                <pre style={{whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: '400px', overflow: 'auto'}}>{error}</pre>
                            </>
                        )}
                    </div>
                </div>
            )}

            {generated && !error && (
                <div ref={resultRef} style={{marginTop: '15px'}}>
                    <div className='white-box'>
                        <div className='white-box__details'>
                            <p>GENERATED APPLICATIONS</p>
                            <div>
                                {generated.length} application{generated.length === 1 ? '' : 's'} would be generated by this ApplicationSet.
                            </div>
                        </div>
                    </div>
                    {generated.length === 0 ? (
                        <div className='white-box' style={{marginTop: '15px'}}>
                            <div className='white-box__details'>No Applications would be generated by this ApplicationSet.</div>
                        </div>
                    ) : (
                        generated.map(app => <GeneratedAppCard key={`${app.metadata?.namespace}/${app.metadata?.name}`} app={app} />)
                    )}
                </div>
            )}
        </div>
    );
};
