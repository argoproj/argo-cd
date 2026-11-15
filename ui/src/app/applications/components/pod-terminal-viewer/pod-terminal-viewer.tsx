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
import {Tooltip} from 'argo-ui/v2';
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

const TooltipWrapper = (props: {content: React.ReactNode | string; disabled?: boolean; inverted?: boolean} & React.PropsWithRef<any>) => {
    return !props.disabled ? (
        <Tooltip content={props.content} inverted={props.inverted}>
            {props.children}
        </Tooltip>
    ) : (
        props.children
    );
};

// Closes a socket that the component is done with. Its handlers are detached first: the browser
// still fires close/error events for a socket closed by us (asynchronously, and as an error when it
// was still connecting), and those must neither show an error toast nor touch whichever socket the
// component is using by then.
function discardSocket(webSocket: WebSocket) {
    webSocket.onopen = null;
    webSocket.onmessage = null;
    webSocket.onerror = null;
    webSocket.onclose = null;
    webSocket.close();
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
    const fitAddonRef = React.useRef(new FitAddon());
    const terminalRefObj = React.useRef<Terminal>(null);
    const webSocketRef = React.useRef<WebSocket>(null);
    const keyEventRef = React.useRef(new ReplaySubject<KeyboardEvent>(2));
    const connSubjectRef = React.useRef(new ReplaySubject<ShellFrame>(100));
    const incommingMessageRef = React.useRef(new Subject<ShellFrame>());
    const unsubscribeRef = React.useRef(new Subject<void>());
    const connectedRef = React.useRef(false);

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
            // Do reconnect due to refresh token event. The old socket is discarded with its handlers
            // detached, so its deferred close event cannot close the replacement opened below.
            discardSocket(webSocketRef.current);
            connectedRef.current = false;
            setupConnection();
        }
    };

    const onConnectionOpen = () => {
        connectedRef.current = true;
        onTerminalResize(); // fit the screen first time
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
            discardSocket(webSocketRef.current);
            webSocketRef.current = null;
        }
        connectedRef.current = false;

        if (connSubjectRef.current) {
            connSubjectRef.current.complete();
            connSubjectRef.current = new ReplaySubject<ShellFrame>(100);
        }

        if (terminalRefObj.current) {
            terminalRefObj.current.dispose();
            terminalRefObj.current = null;
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
            theme: {
                background: '#333'
            }
        };
        terminalRefObj.current.loadAddon(fitAddonRef.current);
        terminalRefObj.current.open(node);
        fitAddonRef.current.fit();

        // One teardown subject per terminal: a completed Subject ignores next(), so the previous
        // terminal's subject (completed by the effect cleanup below) cannot be reused.
        unsubscribeRef.current = new Subject<void>();
        connSubjectRef.current.pipe(takeUntil(unsubscribeRef.current)).subscribe(frame => {
            handleConnectionMessage(frame);
        });

        terminalRefObj.current.onResize(onTerminalResize);
        terminalRefObj.current.onKey(key => {
            keyEventRef.current.next(key.domEvent);
        });
        terminalRefObj.current.onData(onTerminalSendString);
    }

    function setupConnection() {
        const {name = '', namespace = ''} = selectedNode || {};
        const url = `${location.host}${appContext.baseHref}`.replace(/\/$/, '');
        webSocketRef.current = new WebSocket(
            `${
                location.protocol === 'https:' ? 'wss' : 'ws'
            }://${url}/terminal?pod=${name}&container=${containerName}&appName=${applicationName}&appNamespace=${applicationNamespace}&projectName=${projectName}&namespace=${namespace}`
        );
        webSocketRef.current.onopen = onConnectionOpen;
        webSocketRef.current.onclose = onConnectionClose;
        webSocketRef.current.onerror = e => {
            showErrorMsg('Terminal Connection Error', e);
            onConnectionClose();
        };
        webSocketRef.current.onmessage = onConnectionMessage;
    }

    const setTerminalRef = useCallback(
        (node: HTMLElement) => {
            // React detaches the previous callback ref (node === null) before it attaches this one,
            // so the session torn down here is always the previous container's, whether or not its
            // socket had finished connecting.
            if (terminalRefObj.current) {
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
        // React attaches callback refs before it runs passive effects, so the refs now hold the
        // terminal, socket and subjects that setTerminalRef created for this containerName. Capture
        // them: when this effect's cleanup runs on a container switch, setTerminalRef has already been
        // re-attached and the refs point at the NEXT container's objects, which must stay untouched.
        // This effect and setTerminalRef must keep the same dependency list ([containerName]) so that
        // the captured instances are the ones created for this containerName.
        const terminal = terminalRefObj.current;
        const webSocket = webSocketRef.current;
        const connSubject = connSubjectRef.current;
        const incommingMessage = incommingMessageRef.current;
        const unsubscribe = unsubscribeRef.current;

        const resizeHandler = fromEvent(window, 'resize')
            .pipe(debounceTime(1000))
            .subscribe(() => {
                if (fitAddonRef.current) {
                    fitAddonRef.current.fit();
                }
            });
        return () => {
            resizeHandler.unsubscribe(); // unsubscribe resize callback
            unsubscribe.next();
            unsubscribe.complete();

            // clear connection and close terminal (a no-op when setTerminalRef already did it)
            if (webSocket) {
                discardSocket(webSocket);
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

    const isContainerRunning = (container: any): boolean => {
        const containerStatus =
            podState.status?.containerStatuses?.find((status: {name: string}) => status.name === container.name) ||
            podState.status?.initContainerStatuses?.find((status: {name: string}) => status.name === container.name);
        return containerStatus?.state?.running != null;
    };

    return (
        <div className='row pod-terminal-viewer__container'>
            <div className='columns small-3 medium-2'>
                {containerGroups.map(group => (
                    <div key={group.title} style={{marginBottom: '1em'}}>
                        {group.containers.length > 0 && <p>{group.title}</p>}
                        {group.containers.map((container: any, i: number) => {
                            const running = isContainerRunning(container);
                            return (
                                <TooltipWrapper key={container.name} content={!running ? 'Container is not running' : ''} disabled={running}>
                                    <div
                                        className={`application-details__container pod-terminal-viewer__tab ${!running ? 'pod-terminal-viewer__tab--disabled' : ''}`}
                                        onClick={() => {
                                            if (!running) {
                                                return;
                                            }
                                            if (container.name !== containerName) {
                                                disconnect();
                                                onClickContainer(group, i, 'exec');
                                            }
                                        }}
                                        title={!running ? 'Container is not running' : container.name}>
                                        {container.name === containerName && <i className='pod-terminal-viewer__icon fa fa-angle-right negative-space-arrow' />}
                                        <span>{container.name}</span>
                                    </div>
                                </TooltipWrapper>
                            );
                        })}
                    </div>
                ))}
            </div>
            <div className='columns small-9 medium-10'>
                <div ref={setTerminalRef} className='pod-terminal-viewer' />
            </div>
        </div>
    );
};
