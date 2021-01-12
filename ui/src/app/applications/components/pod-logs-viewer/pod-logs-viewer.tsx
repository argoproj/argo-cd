import {DataLoader, DropDownMenu} from 'argo-ui';
import * as React from 'react';

import {useState} from 'react';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import './pod-logs-viewer.scss';

const maxLines = 100;

export const PodsLogsViewer = (props: {applicationName: string; pod: models.ResourceNode & any; containerIndex: number}) => {
    const containers = (props.pod.spec.initContainers || []).concat(props.pod.spec.containers || []);
    const container = containers[props.containerIndex];
    if (!container) {
        return <div>Pod does not have container with index {props.containerIndex}</div>;
    }
    const containerStatuses = ((props.pod.status && props.pod.status.containerStatuses) || []).concat((props.pod.status && props.pod.status.initContainerStatuses) || []);
    const containerStatus = containerStatuses.find((status: any) => status.name === container.name);
    const isRunning = !!(containerStatus && containerStatus.state && containerStatus && containerStatus.state.running);
    let loader: DataLoader;
    const [copy, setCopy] = useState('');
    const [selectedLine, setSelectedLine] = useState(-1);
    const [lines, setLines] = useState([]);
    const bottom = React.useRef<HTMLInputElement>(null);
    const [page, setPage] = useState(0);
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {prefs => (
                <React.Fragment>
                    <div className='pod-logs__settings'>
                        <div
                            className='argo-button argo-button--base'
                            onClick={async () => {
                                try {
                                    await navigator.clipboard.writeText(lines.join('\n'));
                                    setCopy('success');
                                } catch (err) {
                                    setCopy('failure');
                                }
                                setTimeout(() => {
                                    setCopy('');
                                }, 750);
                            }}>
                            {copy === 'success' && (
                                <React.Fragment>
                                    COPIED <i className='fa fa-check' />
                                </React.Fragment>
                            )}
                            {copy === 'failure' && (
                                <React.Fragment>
                                    COPY FAILED <i className='fa fa-times' />
                                </React.Fragment>
                            )}
                            {copy === '' && (
                                <React.Fragment>
                                    COPY <i className='fa fa-clipboard' />
                                </React.Fragment>
                            )}
                        </div>
                        <div
                            className={`argo-button argo-button--base${prefs.appDetails.followLogs ? '' : '-o'}`}
                            onClick={() => {
                                const follow = !prefs.appDetails.followLogs;
                                services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, followLogs: follow}});
                                if (follow) {
                                    setPage(0);
                                }
                                loader.reload();
                            }}>
                            FOLLOW {prefs.appDetails.followLogs && <i className='fa fa-check' />}
                        </div>
                        <div
                            className='argo-button argo-button--base-o'
                            onClick={() => {
                                const inverted = prefs.appDetails.darkMode;
                                services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, darkMode: !inverted}});
                            }}>
                            {prefs.appDetails.darkMode ? <i className='fa fa-sun' /> : <i className='fa fa-moon' />}
                        </div>
                    </div>
                    <DataLoader
                        ref={l => (loader = l)}
                        loadingRenderer={() => (
                            <div className={`pod-logs ${prefs.appDetails.darkMode ? 'pod-logs--inverted' : ''}`}>
                                {logNavigators({}, prefs.appDetails.darkMode, null)}
                                <pre style={{height: '95%', textAlign: 'center'}}>Loading...</pre>
                            </div>
                        )}
                        load={() => {
                            setLines([]);
                            return services.applications.getContainerLogs(
                                props.applicationName,
                                props.pod.metadata.namespace,
                                props.pod.metadata.name,
                                container.name,
                                maxLines * (page + 1),
                                prefs.appDetails.followLogs,
                                maxLines
                            );
                        }}>
                        {log => {
                            if (isRunning && !(!prefs.appDetails.followLogs && lines.length >= maxLines)) {
                                const tmp = lines;
                                tmp.push(log.content);
                                setLines(tmp);
                            }
                            const firstLine = maxLines * page + 1;
                            const lastLine = maxLines * (page + 1);
                            return (
                                <div className={`pod-logs ${prefs.appDetails.darkMode ? 'pod-logs--inverted' : ''}`}>
                                    {logNavigators(
                                        {
                                            left: () => {
                                                setPage(page + 1);
                                                services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, followLogs: false}});
                                                loader.reload();
                                            },
                                            bottom: () => {
                                                bottom.current.scrollIntoView({
                                                    behavior: 'smooth'
                                                });
                                            },
                                            right: () => {
                                                if (page > 0) {
                                                    setPage(page - 1);
                                                    loader.reload();
                                                }
                                            },
                                            end: () => {
                                                setPage(0);
                                                loader.reload();
                                            }
                                        },
                                        prefs.appDetails.darkMode,
                                        {
                                            firstLine,
                                            lastLine,
                                            curPage: page
                                        }
                                    )}
                                    <pre style={{height: '95%'}}>
                                        {lines.map((l, i) => (
                                            <div
                                                style={{display: 'flex', cursor: 'pointer'}}
                                                onClick={() => {
                                                    setSelectedLine(selectedLine === i ? -1 : i);
                                                }}>
                                                <div className={`pod-logs__line__menu ${selectedLine === i ? 'pod-logs__line__menu--visible' : ''}`}>
                                                    <DropDownMenu
                                                        anchor={() => <i className='fas fa-ellipsis-h' />}
                                                        items={[
                                                            {
                                                                title: (
                                                                    <span>
                                                                        <i className='fa fa-clipboard' /> Copy
                                                                    </span>
                                                                ),
                                                                action: async () => {
                                                                    await navigator.clipboard.writeText(l);
                                                                }
                                                            },
                                                            {
                                                                title: (
                                                                    <span>
                                                                        <i className='fa fa-list-ol' /> Copy Line Number
                                                                    </span>
                                                                ),
                                                                action: async () => {
                                                                    await navigator.clipboard.writeText(JSON.stringify(i + 1));
                                                                }
                                                            }
                                                        ]}
                                                    />
                                                </div>
                                                <div className='pod-logs__line__number'>{firstLine + i}</div>
                                                <div className={`pod-logs__line ${selectedLine === i ? 'pod-logs__line--selected' : ''}`}>{l}</div>
                                            </div>
                                        ))}
                                        <div ref={bottom} style={{height: '1px'}} />
                                    </pre>
                                </div>
                            );
                        }}
                    </DataLoader>
                </React.Fragment>
            )}
        </DataLoader>
    );
};

interface NavActions {
    left?: () => void;
    right?: () => void;
    begin?: () => void;
    end?: () => void;
    bottom?: () => void;
}

interface PageInfo {
    firstLine: number;
    lastLine: number;
    curPage: number;
}

const logNavigators = (actions: NavActions, darkMode: boolean, info?: PageInfo) => {
    return (
        <div className={`pod-logs__menu ${darkMode ? 'pod-logs__menu--inverted' : ''}`}>
            {actions.begin && <i className='fa fa-angle-double-left' onClick={actions.begin} />}
            <i className='fa fa-angle-left' onClick={actions.left} />
            <i className='fa fa-angle-down' onClick={actions.bottom} />

            <div style={{marginLeft: 'auto', marginRight: 'auto'}}>
                {info && (
                    <React.Fragment>
                        Page {info.curPage + 1} (Lines {info.firstLine} to {info.lastLine})
                    </React.Fragment>
                )}
            </div>
            <i className={`fa fa-angle-right ${info && info.curPage > 0 ? '' : 'disabled'}`} onClick={info && info.curPage > 0 && actions.right} />
            <i className={`fa fa-angle-double-right ${info && info.curPage > 1 ? '' : 'disabled'}`} onClick={info && info.curPage > 1 && actions.end} />
        </div>
    );
};
