import {Tabs} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as monacoEditor from 'monaco-editor';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';

import {MonacoEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';

interface AppSetPreviewTabProps {
    appSet: models.ApplicationSet;
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

const AppManifestCard = ({app}: {app: models.Application}) => {
    const yaml = React.useMemo(() => jsYaml.dump(app.spec || {}), [app]);
    const name = app.metadata?.name || '(unnamed)';
    const namespace = app.metadata?.namespace || '';
    return (
        <div className='white-box' style={{marginTop: '10px'}}>
            <div className='white-box__details'>
                <div style={{marginBottom: '8px'}}>
                    <strong>{namespace ? `${namespace}/${name}` : name}</strong>
                </div>
                <MonacoEditor
                    vScrollBar={false}
                    editor={{
                        input: {text: yaml, language: 'yaml'},
                        options: {readOnly: true, minimap: {enabled: false}, wordWrap: 'on', scrollBeyondLastLine: false}
                    }}
                />
            </div>
        </div>
    );
};

export const AppSetPreviewTab = (props: AppSetPreviewTabProps) => {
    const {appSet} = props;
    const initialYaml = React.useMemo(() => jsYaml.dump(appSet.spec || {}), [appSet]);
    const modelRef = useRef<monacoEditor.editor.ITextModel | null>(null);
    const resultRef = useRef<HTMLDivElement | null>(null);
    const [editing, setEditing] = useState(false);
    const [previewing, setPreviewing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [permissionDenied, setPermissionDenied] = useState(false);
    const [generated, setGenerated] = useState<models.Application[] | null>(null);
    const [currentApps, setCurrentApps] = useState<models.Application[]>([]);
    const [diffStates, setDiffStates] = useState<models.ResourceDiff[] | null>(null);
    const [diffWarning, setDiffWarning] = useState<string | null>(null);
    const [editorKey, setEditorKey] = useState(0);
    const [resultTab, setResultTab] = useState<string>('diff');

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
        setDiffWarning(null);
        let parsedSpec: any;
        try {
            parsedSpec = jsYaml.load(readEditorYaml());
        } catch (e: any) {
            setError(`Invalid YAML: ${e.message || e}`);
            return;
        }
        if (!parsedSpec || typeof parsedSpec !== 'object') {
            setError('Invalid YAML: spec must be an object');
            return;
        }
        const parsed: models.ApplicationSet = {...appSet, spec: parsedSpec};
        setPreviewing(true);
        try {
            const [proposedApps, currentAppsResult] = await Promise.all([
                services.applications.appSetGenerate(parsed),
                services.applications.appSetGenerate(appSet).catch(e => {
                    setDiffWarning(`Could not generate current appset output for diff: ${extractErrorMessage(e)}`);
                    return [] as models.Application[];
                })
            ]);
            setGenerated(proposedApps);

            const currentAppsList = currentAppsResult as models.Application[];
            setCurrentApps(currentAppsList);
            const defaultNs = appSet.metadata?.namespace || '';
            const keyOf = (app: models.Application) => `${app.metadata?.namespace || defaultNs}/${app.metadata?.name}`;
            const currentByKey = new Map<string, models.Application>();
            for (const app of currentAppsList) {
                currentByKey.set(keyOf(app), app);
            }
            const proposedByKey = new Map<string, models.Application>();
            for (const app of proposedApps) {
                proposedByKey.set(keyOf(app), app);
            }
            const allKeys: string[] = [];
            const seen = new Set<string>();
            for (const k of [...Array.from(proposedByKey.keys()), ...Array.from(currentByKey.keys())]) {
                if (!seen.has(k)) {
                    seen.add(k);
                    allKeys.push(k);
                }
            }
            const states: models.ResourceDiff[] = allKeys.map(key => {
                const current = currentByKey.get(key) || null;
                const proposed = proposedByKey.get(key) || null;
                const sep = key.indexOf('/');
                const ns = key.slice(0, sep);
                const name = key.slice(sep + 1);
                return {
                    group: 'argoproj.io',
                    kind: 'Application',
                    namespace: ns,
                    name,
                    hook: false,
                    normalizedLiveState: current,
                    predictedLiveState: proposed,
                    liveState: current,
                    targetState: proposed
                } as models.ResourceDiff;
            });
            setDiffStates(states);
        } catch (e: any) {
            if (isPermissionDeniedError(e)) {
                setPermissionDenied(true);
            }
            setError(extractErrorMessage(e));
            setGenerated(null);
            setDiffStates(null);
            setCurrentApps([]);
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
                    {diffWarning && (
                        <div className='white-box' style={{marginTop: '15px'}}>
                            <div className='white-box__details'>{diffWarning}</div>
                        </div>
                    )}
                    <div style={{marginTop: '15px'}}>
                        <Tabs
                            key={resultTab}
                            navTransparent={true}
                            selectedTabKey={resultTab}
                            onTabSelected={setResultTab}
                            tabs={[
                                {
                                    title: 'CURRENT',
                                    key: 'current',
                                    content:
                                        currentApps.length === 0 ? (
                                            <div className='white-box'>
                                                <div className='white-box__details'>No Applications are currently generated by this ApplicationSet.</div>
                                            </div>
                                        ) : (
                                            <div>
                                                {currentApps.map(app => (
                                                    <AppManifestCard key={`${app.metadata?.namespace}/${app.metadata?.name}`} app={app} />
                                                ))}
                                            </div>
                                        )
                                },
                                {
                                    title: 'DIFF',
                                    key: 'diff',
                                    content:
                                        diffStates && diffStates.length > 0 ? (
                                            <ApplicationResourcesDiff states={diffStates} />
                                        ) : (
                                            <div className='white-box'>
                                                <div className='white-box__details'>No changes — proposed output matches current output.</div>
                                            </div>
                                        )
                                },
                                {
                                    title: 'PREVIEW',
                                    key: 'preview',
                                    content:
                                        generated.length === 0 ? (
                                            <div className='white-box'>
                                                <div className='white-box__details'>No Applications would be generated by this ApplicationSet.</div>
                                            </div>
                                        ) : (
                                            <div>
                                                {generated.map(app => (
                                                    <AppManifestCard key={`${app.metadata?.namespace}/${app.metadata?.name}`} app={app} />
                                                ))}
                                            </div>
                                        )
                                }
                            ]}
                        />
                    </div>
                </div>
            )}
        </div>
    );
};
