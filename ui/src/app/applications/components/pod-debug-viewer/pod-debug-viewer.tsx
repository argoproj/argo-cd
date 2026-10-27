import {Terminal} from 'xterm';
import {FitAddon} from 'xterm-addon-fit';
import * as models from '../../../shared/models';
import * as React from 'react';
import './pod-debug-viewer.scss';
import 'xterm/css/xterm.css';
import {useCallback, useEffect, useState} from 'react';
import {debounceTime, takeUntil} from 'rxjs/operators';
import {fromEvent, ReplaySubject, Subject} from 'rxjs';
import {Context} from '../../../shared/context';
import {ErrorNotification, NotificationType} from 'argo-ui';

export interface PodDebugViewerProps {
    applicationName: string;
    applicationNamespace: string;
    projectName: string;
    selectedNode: models.ResourceNode;
    podState: models.State;
    debugImages: string[];
}

export interface ShellFrame {
    operation: string;
    data?: string;
    rows?: number;
    cols?: number;
}

export const PodDebugViewer: React.FC<PodDebugViewerProps> = ({selectedNode, applicationName, applicationNamespace, projectName, podState, debugImages}) => {
    const terminalRef = React.useRef(null);
    const appContext = React.useContext(Context);
    const fitAddonRef = React.useRef(new FitAddon());
    const terminalRefObj = React.useRef<Terminal>(null);
    const webSocketRef = React.useRef<WebSocket>(null);
    const connSubjectRef = React.useRef(new ReplaySubject<ShellFrame>(100));
    const incommingMessageRef = React.useRef(new Subject<ShellFrame>());
    const unsubscribeRef = React.useRef(new Subject<void>());
    const connectedRef = React.useRef(false);

    const [selectedImage, setSelectedImage] = useState<string>(debugImages?.[0] || '');
    const [selectedTargetContainer, setSelectedTargetContainer] = useState<string>('');
    const [sessionStarted, setSessionStarted] = useState<boolean>(false);

    const containers = (podState?.spec?.containers || []).map((c: any) => c.name);

    function showErrorMsg(msg: string, err: any) {
        appContext.notifications.show({
            content: <ErrorNotification title={msg} e={err} />,
            type: NotificationType.Error
        });
    }

    const onTerminalSendString = (str: string) => {
        if (connectedRef.current) {
            webSocketRef.current.send(JSON.stringify({operation: 'stdin', data: str, rows: terminalRefObj.current.rows, cols: terminalRefObj.current.cols}));
        }
    };

    const onTerminalResize = () => {
        if (connectedRef.current) {
            webSocketRef.current.send(
                JSON.stringify({
                    operation: 'resize',
                    cols: terminalRefObj.current.cols,
                    rows: terminalRefObj.current.rows
                })
            );
        }
    };

    const onConnectionMessage = (e: MessageEvent) => {
        const msg = JSON.parse(e.data);
        if (!msg?.Code) {
            connSubjectRef.current.next(msg);
        } else {
            onConnectionClose();
            disconnect();
            setSessionStarted(false);
            showErrorMsg('Debug session ended', new Error('The session token was refreshed. Start a new debug session.'));
        }
    };

    const onConnectionOpen = () => {
        connectedRef.current = true;
        onTerminalResize();
        terminalRefObj.current.focus();
    };

    const onConnectionClose = () => {
        if (!connectedRef.current) return;
        if (webSocketRef.current) webSocketRef.current.close();
        connectedRef.current = false;
    };

    const handleConnectionMessage = (frame: ShellFrame) => {
        terminalRefObj.current.write(frame.data);
        incommingMessageRef.current.next(frame);
    };

    const disconnect = () => {
        if (webSocketRef.current) {
            webSocketRef.current.close();
        }
        if (connSubjectRef.current) {
            connSubjectRef.current.complete();
            connSubjectRef.current = new ReplaySubject<ShellFrame>(100);
        }
        if (terminalRefObj.current) {
            terminalRefObj.current.dispose();
        }
        incommingMessageRef.current.complete();
        incommingMessageRef.current = new Subject<ShellFrame>();
    };

    function initTerminal(node: HTMLElement) {
        if (connSubjectRef.current) {
            connSubjectRef.current.complete();
            connSubjectRef.current = new ReplaySubject<ShellFrame>(100);
        }
        if (terminalRefObj.current) {
            terminalRefObj.current.dispose();
        }

        terminalRefObj.current = new Terminal({
            convertEol: true,
            fontFamily: 'Menlo, Monaco, Courier New, monospace',
            bellStyle: 'sound',
            fontSize: 14,
            fontWeight: 400,
            cursorBlink: true
        });
        terminalRefObj.current.options = {
            theme: {background: '#333'}
        };
        terminalRefObj.current.loadAddon(fitAddonRef.current);
        terminalRefObj.current.open(node);
        fitAddonRef.current.fit();

        connSubjectRef.current.pipe(takeUntil(unsubscribeRef.current)).subscribe(frame => {
            handleConnectionMessage(frame);
        });

        terminalRefObj.current.onResize(onTerminalResize);
        terminalRefObj.current.onData(onTerminalSendString);
    }

    function setupConnection() {
        const {name = '', namespace = ''} = selectedNode || {};
        const url = `${location.host}${appContext.baseHref}`.replace(/\/$/, '');
        const params = new URLSearchParams({
            pod: name,
            appName: applicationName,
            appNamespace: applicationNamespace,
            projectName,
            namespace,
            image: selectedImage,
            ...(selectedTargetContainer ? {targetContainer: selectedTargetContainer} : {})
        });
        webSocketRef.current = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${url}/debug?${params.toString()}`);
        webSocketRef.current.onopen = onConnectionOpen;
        webSocketRef.current.onclose = onConnectionClose;
        webSocketRef.current.onerror = e => {
            showErrorMsg('Debug Connection Error', e);
            onConnectionClose();
        };
        webSocketRef.current.onmessage = onConnectionMessage;
    }

    const setTerminalRef = useCallback(
        (node: HTMLElement) => {
            if (terminalRefObj.current && connectedRef.current) {
                disconnect();
            }
            if (node) {
                initTerminal(node);
                setupConnection();
            }
            terminalRef.current = node;
        },
        [selectedImage, selectedTargetContainer, sessionStarted]
    );

    useEffect(() => {
        const resizeHandler = fromEvent(window, 'resize')
            .pipe(debounceTime(1000))
            .subscribe(() => {
                if (fitAddonRef.current) {
                    fitAddonRef.current.fit();
                }
            });
        return () => {
            resizeHandler.unsubscribe();
            unsubscribeRef.current.next();
            unsubscribeRef.current.complete();
            unsubscribeRef.current = new Subject<void>();
            if (webSocketRef.current) webSocketRef.current.close();
            if (connSubjectRef.current) connSubjectRef.current.complete();
            if (terminalRefObj.current) terminalRefObj.current.dispose();
            incommingMessageRef.current.complete();
        };
    }, [selectedImage, selectedTargetContainer, sessionStarted]);

    return (
        <div className='pod-debug-viewer__container'>
            {!sessionStarted ? (
                <div className='pod-debug-viewer__controls' style={{flexDirection: 'column', alignItems: 'flex-start', padding: '1.5rem'}}>
                    <h4 style={{marginBottom: '1rem'}}>Start Debug Session</h4>
                    <div style={{marginBottom: '1rem'}}>
                        <label>Debug Image:</label>
                        <select value={selectedImage} onChange={e => setSelectedImage(e.target.value)} style={{marginLeft: '0.5rem', minWidth: '200px'}}>
                            {(debugImages || []).map(img => (
                                <option key={img} value={img}>
                                    {img}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div style={{marginBottom: '1rem'}}>
                        <label>Target Container (optional):</label>
                        <select value={selectedTargetContainer} onChange={e => setSelectedTargetContainer(e.target.value)} style={{marginLeft: '0.5rem', minWidth: '200px'}}>
                            <option value=''>-- None (share PID namespace) --</option>
                            {containers.map((name: string) => (
                                <option key={name} value={name}>
                                    {name}
                                </option>
                            ))}
                        </select>
                    </div>
                    <button
                        className='argo-button argo-button--base'
                        onClick={() => {
                            if (selectedImage) {
                                setSessionStarted(true);
                            }
                        }}
                        disabled={!selectedImage}>
                        <i className='fa fa-bug' /> Start Debug Session
                    </button>
                </div>
            ) : (
                <div>
                    <div className='pod-debug-viewer__controls'>
                        <span style={{fontSize: '12px'}}>
                            <i className='fa fa-bug' /> Debugging with <strong>{selectedImage}</strong>
                            {selectedTargetContainer && (
                                <>
                                    {' '}
                                    → <strong>{selectedTargetContainer}</strong>
                                </>
                            )}
                        </span>
                        <button
                            className='argo-button argo-button--base-o'
                            style={{marginLeft: 'auto', fontSize: '11px'}}
                            onClick={() => {
                                disconnect();
                                setSessionStarted(false);
                            }}>
                            <i className='fa fa-times' /> End Session
                        </button>
                    </div>
                    <div ref={setTerminalRef} className='pod-debug-viewer' />
                </div>
            )}
        </div>
    );
};
