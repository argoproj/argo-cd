import {Terminal} from 'xterm';
import {FitAddon} from 'xterm-addon-fit';
import * as models from '../../../shared/models';
import * as React from 'react';
import './pod-debug-viewer.scss';
import 'xterm/css/xterm.css';
import {useEffect, useState} from 'react';
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
    containerName: string;
    onClickContainer?: (group: any, i: number, tab: string) => any;
}

export interface DebugFrame {
    operation: string;
    data?: string;
    rows?: number;
    cols?: number;
}


export const PodDebugViewer: React.FC<PodDebugViewerProps> = ({
    selectedNode,
    applicationName,
    applicationNamespace,
    projectName,
    podState,
    containerName,
    onClickContainer
}) => {
    const terminalRef = React.useRef(null);
    const appContext = React.useContext(Context);
    const fitAddon = React.useRef(new FitAddon());

    // Debug-specific state
    const [debugImage, setDebugImage] = useState('busybox:1.28');
    const [debugCommand, setDebugCommand] = useState('sh');
    const [shareProcesses, setShareProcesses] = useState(true);
    const [isConnected, setIsConnected] = useState(false);
    const [showConfig, setShowConfig] = useState(true);

    const terminalInstance = React.useRef<Terminal | null>(null);
    const webSocketRef = React.useRef<WebSocket | null>(null);
    const connSubject = React.useRef(new ReplaySubject<DebugFrame>(100));
    const unsubscribe = React.useRef(new Subject<void>());

    function showErrorMsg(msg: string, err: any) {
        appContext.notifications.show({
            content: <ErrorNotification title={msg} e={err} />,
            type: NotificationType.Error
        });
    }

    const onTerminalSendString = (str: string) => {
        if (isConnected && webSocketRef.current && terminalInstance.current) {
            webSocketRef.current.send(JSON.stringify({
                operation: 'stdin',
                data: str,
                rows: terminalInstance.current.rows,
                cols: terminalInstance.current.cols
            }));
        }
    };

    const onTerminalResize = () => {
        if (isConnected && webSocketRef.current && terminalInstance.current) {
            webSocketRef.current.send(
                JSON.stringify({
                    operation: 'resize',
                    cols: terminalInstance.current.cols,
                    rows: terminalInstance.current.rows
                })
            );
        }
    };

    const onConnectionMessage = (e: MessageEvent) => {
        const msg = JSON.parse(e.data);
        console.log('Debug WebSocket message received:', msg);
        if (!msg?.Code) {
            connSubject.current.next(msg);
        } else {
            onConnectionClose();
            setupConnection();
        }
    };

    const initializeTerminal = () => {
        console.log('Initializing terminal, terminalRef.current:', terminalRef.current);
        if (terminalRef.current && !terminalInstance.current) {
            console.log('Creating new terminal instance');
            terminalInstance.current = new Terminal({
                cursorBlink: true,
                convertEol: true,
                fontFamily: 'Menlo, Monaco, Courier New, monospace',
                fontSize: 14,
                theme: {
                    background: '#1e1e1e',
                    foreground: '#d4d4d4'
                }
            });

            terminalInstance.current.loadAddon(fitAddon.current);
            terminalInstance.current.open(terminalRef.current);
            fitAddon.current.fit();

            terminalInstance.current.onData((data) => {
                console.log('Terminal input data:', data);
                onTerminalSendString(data);
            });
            terminalInstance.current.onResize(onTerminalResize);

            // Handle resize events
            fromEvent(window, 'resize')
                .pipe(debounceTime(100), takeUntil(unsubscribe.current))
                .subscribe(() => {
                    if (terminalInstance.current && fitAddon.current) {
                        fitAddon.current.fit();
                        onTerminalResize();
                    }
                });

            console.log('Terminal initialized successfully');
        }
    };

    const onConnectionOpen = () => {
        console.log('Debug WebSocket connection opened');
        setIsConnected(true);
        setShowConfig(false);

        if (terminalInstance.current) {
            console.log('Terminal found, resizing and focusing');
            onTerminalResize();
            terminalInstance.current.focus();
        } else {
            console.log('Terminal instance not found');
        }
    };

    const onConnectionClose = () => {
        setIsConnected(false);
        if (webSocketRef.current) {
            webSocketRef.current.close();
        }
    };

    const onConnectionError = (error: Event) => {
        showErrorMsg('Connection error', error);
        onConnectionClose();
    };

    const setupConnection = () => {
        if (webSocketRef.current) {
            webSocketRef.current.close();
        }

        // Set up message handling before connection
        connSubject.current.pipe(takeUntil(unsubscribe.current)).subscribe(msg => {
            console.log('Processing message for terminal:', msg);
            if (msg.operation === 'stdout' && msg.data && terminalInstance.current) {
                console.log('Writing to terminal:', msg.data);
                terminalInstance.current.write(msg.data);
            }
        });

        const url = `${location.host}${appContext.baseHref}`.replace(/\/$/, '');

        // kubectl debug WebSocket endpoint (following same pattern as terminal)
        const wsUrl = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${url}/debug` +
            `?pod=${selectedNode.name}&appName=${applicationName}` +
            `&projectName=${projectName}&namespace=${selectedNode.namespace}` +
            `&image=${encodeURIComponent(debugImage)}&command=${encodeURIComponent(debugCommand)}` +
            `&shareProcesses=${shareProcesses}`;

        webSocketRef.current = new WebSocket(wsUrl);
        webSocketRef.current.addEventListener('open', onConnectionOpen);
        webSocketRef.current.addEventListener('message', onConnectionMessage);
        webSocketRef.current.addEventListener('close', onConnectionClose);
        webSocketRef.current.addEventListener('error', onConnectionError);
    };

    const startDebugSession = () => {
        if (!debugImage.trim()) {
            showErrorMsg('Debug image is required', new Error('Please specify a debug image'));
            return;
        }

        // Initialize terminal first
        initializeTerminal();

        setupConnection();
    };

    const stopDebugSession = () => {
        onConnectionClose();
        setShowConfig(true);
        if (terminalInstance.current) {
            terminalInstance.current.dispose();
            terminalInstance.current = null;
        }
    };

    useEffect(() => {
        return () => {
            unsubscribe.current.next();
            unsubscribe.current.complete();
            onConnectionClose();
            if (terminalInstance.current) {
                terminalInstance.current.dispose();
            }
        };
    }, []);

    React.useEffect(() => {
        if (terminalRef.current && !terminalInstance.current) {
            console.log('Initializing terminal after container is rendered');
            initializeTerminal();
        }

        return () => {
            if (terminalInstance.current) {
                console.log('Cleaning up terminal event listeners');
                terminalInstance.current.dispose();
                terminalInstance.current = null;
            }
        };
    }, [terminalRef.current]);

    console.log('Rendering PodDebugViewer component');

    if (showConfig && !isConnected) {
        return (
            <div className="pod-debug-viewer">
                <div className="debug-config">
                    <div className="debug-config__header">
                        <h3>🐛 Debug Pod Configuration</h3>
                        <p>Configure kubectl debug session for pod: <strong>{selectedNode.name}</strong></p>
                    </div>

                    <div className="debug-config__form">
                        <div className="form-field">
                            <label htmlFor="debug-image">Debug Image *</label>
                            <input
                                id="debug-image"
                                type="text"
                                value={debugImage}
                                onChange={(e) => setDebugImage(e.target.value)}
                                placeholder="e.g., busybox:1.28, ubuntu:20.04, nicolaka/netshoot"
                                className="debug-input"
                            />
                            <small>Container image to use for debugging</small>
                        </div>

                        <div className="form-field">
                            <label htmlFor="debug-command">Command</label>
                            <input
                                id="debug-command"
                                type="text"
                                value={debugCommand}
                                onChange={(e) => setDebugCommand(e.target.value)}
                                placeholder="e.g., sh, bash, /bin/bash"
                                className="debug-input"
                            />
                            <small>Command to run in debug container</small>
                        </div>

                        <div className="form-field">
                            <label className="checkbox-label">
                                <input
                                    type="checkbox"
                                    checked={shareProcesses}
                                    onChange={(e) => setShareProcesses(e.target.checked)}
                                />
                                Share process namespace with target container
                            </label>
                            <small>Allows debugging tools to see processes from the target container</small>
                        </div>

                        <div className="debug-config__actions">
                            <button
                                className="debug-button debug-button--primary"
                                onClick={startDebugSession}
                                disabled={!debugImage.trim()}
                            >
                                Start Debug Session
                            </button>
                        </div>

                        <div className="debug-config__info">
                            <h4>Popular Debug Images:</h4>
                            <div className="debug-images">
                                <button
                                    className="image-suggestion"
                                    onClick={() => setDebugImage('busybox:1.28')}
                                >
                                    busybox:1.28
                                </button>
                                <button
                                    className="image-suggestion"
                                    onClick={() => setDebugImage('ubuntu:20.04')}
                                >
                                    ubuntu:20.04
                                </button>
                                <button
                                    className="image-suggestion"
                                    onClick={() => setDebugImage('nicolaka/netshoot')}
                                >
                                    nicolaka/netshoot
                                </button>
                                <button
                                    className="image-suggestion"
                                    onClick={() => setDebugImage('alpine:latest')}
                                >
                                    alpine:latest
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="pod-debug-viewer">
            <div className="debug-header">
                <div className="debug-session-info">
                    <span className="debug-status">
                        Debug Session: {selectedNode.name} (<span className="debug-image-tag">{debugImage}</span>)
                    </span>
                </div>
                <div className="debug-actions">
                    <button
                        className="debug-button debug-button--secondary"
                        onClick={() => setShowConfig(true)}
                    >
                        Reconfigure
                    </button>
                    <button
                        className="debug-button debug-button--danger"
                        onClick={stopDebugSession}
                    >
                        Stop Debug
                    </button>
                </div>
            </div>

            <div className="debug-terminal-container">
                <div ref={terminalRef} className="debug-terminal" />
                {console.log('Terminal container rendered', terminalRef.current)}
                {!isConnected && (
                    <div className="debug-connecting">
                        <div className="connecting-spinner"></div>
                        <p>Connecting to debug session...</p>
                    </div>
                )}
            </div>
        </div>
    );
};
