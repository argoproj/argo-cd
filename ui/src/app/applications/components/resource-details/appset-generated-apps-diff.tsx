import {Tabs} from 'argo-ui';
import * as jsYaml from 'js-yaml';
import * as React from 'react';
import {useEffect, useState} from 'react';

import {MonacoEditor} from '../../../shared/components';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationResourcesDiff} from '../application-resources-diff/application-resources-diff';
import './resource-details.scss';

export interface AppSetGeneratedAppsDiffProps {
    currentAppSet: models.ApplicationSet;
    proposedAppSet: models.ApplicationSet;
    trigger: number;
    rbacProject?: string;
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

export const AppSetGeneratedAppsDiff = (props: AppSetGeneratedAppsDiffProps) => {
    const {currentAppSet, proposedAppSet, trigger, rbacProject} = props;
    const [running, setRunning] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [permissionDenied, setPermissionDenied] = useState(false);
    const [generated, setGenerated] = useState<models.Application[] | null>(null);
    const [currentApps, setCurrentApps] = useState<models.Application[]>([]);
    const [diffStates, setDiffStates] = useState<models.ResourceDiff[] | null>(null);
    const [diffWarning, setDiffWarning] = useState<string | null>(null);
    const [resultTab, setResultTab] = useState<string>('diff');

    useEffect(() => {
        if (trigger === 0) {
            return;
        }
        let cancelled = false;
        const run = async () => {
            setError(null);
            setPermissionDenied(false);
            setDiffWarning(null);
            setRunning(true);
            try {
                const [proposedApps, currentAppsResult] = await Promise.all([
                    services.applications.appSetGenerate(proposedAppSet),
                    services.applications.appSetGenerate(currentAppSet).catch(e => {
                        if (!cancelled) {
                            setDiffWarning(`Could not generate current appset output for diff: ${extractErrorMessage(e)}`);
                        }
                        return [] as models.Application[];
                    })
                ]);
                if (cancelled) return;

                const currentAppsList = currentAppsResult as models.Application[];
                setGenerated(proposedApps);
                setCurrentApps(currentAppsList);

                const defaultNs = currentAppSet.metadata?.namespace || proposedAppSet.metadata?.namespace || '';
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
                if (cancelled) return;
                if (isPermissionDeniedError(e)) {
                    setPermissionDenied(true);
                }
                setError(extractErrorMessage(e));
                setGenerated(null);
                setDiffStates(null);
                setCurrentApps([]);
            } finally {
                if (!cancelled) setRunning(false);
            }
        };
        run();
        return () => {
            cancelled = true;
        };
    }, [trigger, currentAppSet, proposedAppSet]);

    if (running) {
        return (
            <div className='white-box' style={{marginTop: '15px'}}>
                <div className='white-box__details'>
                    <i className='fa fa-circle-notch fa-spin' style={{marginRight: '6px'}} />
                    Previewing...
                </div>
            </div>
        );
    }

    if (error) {
        return (
            <div className='white-box' style={{marginTop: '15px'}}>
                <div className='white-box__details'>
                    {permissionDenied ? (
                        <>
                            <p>PERMISSION DENIED</p>
                            <div>
                                You need <code>applicationsets, create</code> permission on project <code>{rbacProject || 'unknown'}</code> to preview generated Applications.
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
        );
    }

    if (!generated) {
        return null;
    }

    return (
        <div style={{marginTop: '15px'}}>
            {diffWarning && (
                <div className='white-box' style={{marginBottom: '15px'}}>
                    <div className='white-box__details'>{diffWarning}</div>
                </div>
            )}
            <div className='appset-generated-apps-diff__manifest'>
                <Tabs
                    key={resultTab}
                    selectedTabKey={resultTab}
                    onTabSelected={setResultTab}
                    tabs={[
                        {
                            title: 'LIVE APPS',
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
                            title: 'DESIRED APPS',
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
    );
};
