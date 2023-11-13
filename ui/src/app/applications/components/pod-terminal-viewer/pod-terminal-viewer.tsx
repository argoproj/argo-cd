import {Terminal} from 'xterm';
import {FitAddon} from 'xterm-addon-fit';
import * as models from '../../../shared/models';
import * as React from 'react';
import './pod-terminal-viewer.scss';
import 'xterm/css/xterm.css';
import {useCallback, useEffect} from 'react';
import {debounceTime, takeUntil} from 'rxjs/operators';
import {fromEvent, ReplaySubject, Subject} from 'rxjs';
import {Context} from '../../../shared/context';
import {ErrorNotification, NotificationType} from 'argo-ui';
export interface PodTerminalViewerProps {
    applicationName: string;
    applicationNamespace: string;
    projectName: string;
    selectedNode: models.ResourceNode;
    podState: models.State;
    containerName: string;
    onClickContainer?: (group: any, i: number, tab: string) => any;
}
export interface ShellFrame {
    operation: string;
    data?: string;
    rows?: number;
    cols?: number;
}

export const PodTerminalViewer: React.FC<PodTerminalViewerProps> = ({
    selectedNode,
    applicationName,
    applicationNamespace,
    projectName,
    podState,
    containerName,
    onClickContainer
}) => {
    const terminalRef = React.useRef(null);
    const appContext = React.useContext(Context); // used to show toast
    const fitAddon = new FitAddon();
    let terminal: Terminal;
    let webSocket: WebSocket;
    const keyEvent = new ReplaySubject<KeyboardEvent>(2);
    let connSubject = new ReplaySubject<ShellFrame>(100);
    let incommingMessage = new Subject<ShellFrame>();
    const unsubscribe = new Subject<void>();
    let connected = false;

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
            // Do reconnect due to refresh token event
            onConnectionClose();
            setupConnection();
        }
    };

    const onConnectionOpen = () => {
        connected = true;
        onTerminalResize(); // fit the screen first time
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
            theme: {
                background: '#333'
            }
        };
        terminal.loadAddon(fitAddon);
        terminal.open(node);
        fitAddon.fit();

        connSubject.pipe(takeUntil(unsubscribe)).subscribe(frame => {
            handleConnectionMessage(frame);
        });

        terminal.onResize(onTerminalResize);
        terminal.onKey(key => {
            keyEvent.next(key.domEvent);
        });
        terminal.onData(onTerminalSendString);
    }

    function setupConnection() {
        const {name = '', namespace = ''} = selectedNode || {};
        const url = `${location.host}${appContext.baseHref}`.replace(/\/$/, '');
        webSocket = new WebSocket(
            `${
                location.protocol === 'https:' ? 'wss' : 'ws'
            }://${url}/terminal?pod=${name}&container=${containerName}&appName=${applicationName}&appNamespace=${applicationNamespace}&projectName=${projectName}&namespace=${namespace}`
        );
        webSocket.onopen = onConnectionOpen;
        webSocket.onclose = onConnectionClose;
        webSocket.onerror = e => {
            showErrorMsg('Terminal Connection Error', e);
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

            // Save a reference to the node
            terminalRef.current = node;
        },
        [containerName]
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
            resizeHandler.unsubscribe(); // unsubscribe resize callback
            unsubscribe.next();
            unsubscribe.complete();

            // clear connection and close terminal
            if (webSocket) {
                webSocket.close();
            }

            if (connSubject) {
                connSubject.complete();
            }

            if (terminal) {
                terminal.dispose();
            }

            incommingMessage.complete();
        };
    }, [containerName]);

    const containerGroups = [
        {
            offset: 0,
            title: 'CONTAINERS',
            containers: podState.spec.containers || []
        },
        {
            offset: (podState.spec.containers || []).length,
            title: 'INIT CONTAINERS',
            containers: podState.spec.initContainers || []
        }
    ];

    return (
        <div className='row'>
            <div className='columns small-3 medium-2'>
                {containerGroups.map(group => (
                    <div key={group.title} style={{marginBottom: '1em'}}>
                        {group.containers.length > 0 && <p>{group.title}</p>}
                        {group.containers.map((container: any, i: number) => (
                            <div
                                className='application-details__container'
                                key={container.name}
                                onClick={() => {
                                    if (container.name !== containerName) {
                                        disconnect();
                                        onClickContainer(group, i, 'exec');
                                    }
                                }}>
                                {container.name === containerName && <i className='fa fa-angle-right negative-space-arrow' />}
                                <span title={container.name}>{container.name}</span>
                            </div>
                        ))}
                    </div>
                ))}
            </div>
            <div className='columns small-9 medium-10'>
                <div ref={setTerminalRef} className='pod-terminal-viewer' />
            </div>
        </div>
    );
};
