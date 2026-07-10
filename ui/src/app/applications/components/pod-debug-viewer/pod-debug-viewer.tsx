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
    const fitAddon = new FitAddon();
    let terminal: Terminal;
    let webSocket: WebSocket;
    let connSubject = new ReplaySubject<ShellFrame>(100);
    let incommingMessage = new Subject<ShellFrame>();
    const unsubscribe = new Subject<void>();
    let connected = false;

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
        if (connected) {
            webSocket.send(JSON.stringify({operation: 'stdin', data: str, rows: terminal.rows, cols: terminal.cols}));
        }
    };

    const onTerminalResize = () => {
        if (connected) {
            webSocket.send(
                JSON.stringify({
                    operation: 'resize',
                    cols: terminal.cols,
                    rows: terminal.rows
                })
            );
        }
    };

    const onConnectionMessage = (e: MessageEvent) => {
        const msg = JSON.parse(e.data);
        if (!msg?.Code) {
            connSubject.next(msg);
        } else {
            onConnectionClose();
            setupConnection();
        }
    };

    const onConnectionOpen = () => {
        connected = true;
        onTerminalResize();
        terminal.focus();
    };

    const onConnectionClose = () => {
        if (!connected) return;
        if (webSocket) webSocket.close();
        connected = false;
    };

    const handleConnectionMessage = (frame: ShellFrame) => {
        terminal.write(frame.data);
        incommingMessage.next(frame);
    };

    const disconnect = () => {
        if (webSocket) {
            webSocket.close();
        }
        if (connSubject) {
            connSubject.complete();
            connSubject = new ReplaySubject<ShellFrame>(100);
        }
        if (terminal) {
            terminal.dispose();
        }
        incommingMessage.complete();
        incommingMessage = new Subject<ShellFrame>();
    };

    function initTerminal(node: HTMLElement) {
        if (connSubject) {
            connSubject.complete();
            connSubject = new ReplaySubject<ShellFrame>(100);
        }
        if (terminal) {
            terminal.dispose();
        }

        terminal = new Terminal({
            convertEol: true,
            fontFamily: 'Menlo, Monaco, Courier New, monospace',
            bellStyle: 'sound',
            fontSize: 14,
            fontWeight: 400,
            cursorBlink: true
        });
        terminal.options = {
            theme: {background: '#333'}
        };
        terminal.loadAddon(fitAddon);
        terminal.open(node);
        fitAddon.fit();

        connSubject.pipe(takeUntil(unsubscribe)).subscribe(frame => {
            handleConnectionMessage(frame);
        });

        terminal.onResize(onTerminalResize);
        terminal.onData(onTerminalSendString);
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
        webSocket = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${url}/debug?${params.toString()}`);
        webSocket.onopen = onConnectionOpen;
        webSocket.onclose = onConnectionClose;
        webSocket.onerror = e => {
            showErrorMsg('Debug Connection Error', e);
            onConnectionClose();
        };
        webSocket.onmessage = onConnectionMessage;
    }

    const setTerminalRef = useCallback(
        node => {
            if (terminal && connected) {
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
                if (fitAddon) {
                    fitAddon.fit();
                }
            });
        return () => {
            resizeHandler.unsubscribe();
            unsubscribe.next();
            unsubscribe.complete();
            if (webSocket) webSocket.close();
            if (connSubject) connSubject.complete();
            if (terminal) terminal.dispose();
            incommingMessage.complete();
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
