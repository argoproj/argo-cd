import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useState} from 'react';
import {Link} from 'react-router-dom';
import {Observable} from 'rxjs';

import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

import {BASE_COLORS} from '../utils';

import './pod-logs-viewer.scss';

const maxLines = 100;
export interface PodLogsProps {
    namespace: string;
    applicationName: string;
    podName?: string;
    containerName: string;
    group?: string;
    kind?: string;
    name?: string;
    page: {number: number; untilTimes: string[]};
    setPage: (pageData: {number: number; untilTimes: string[]}) => void;
}

export const PodsLogsViewer = (props: PodLogsProps & {fullscreen?: boolean}) => {
    if (!props.containerName || props.containerName === '') {
        return <div>Pod does not have container with name {props.containerName}</div>;
    }

    let loader: DataLoader<models.LogEntry[], string>;
    const [copy, setCopy] = useState('');
    const [selectedLine, setSelectedLine] = useState(-1);
    const bottom = React.useRef<HTMLInputElement>(null);
    const top = React.useRef<HTMLInputElement>(null);
    const page = props.page;
    const setPage = props.setPage;
    const [viewPodNames, setViewPodNames] = useState(false);

    interface FilterData {
        literal: string;
        inverse: boolean;
    }
    const [filter, setFilter] = useState({inverse: false, literal: ''} as FilterData);

    const filterQuery = () => {
        return filter.literal && `${filter.inverse ? '!' : ''}${filter.literal}`;
    };
    const fullscreenURL =
        `/applications/${props.applicationName}/${props.namespace}/${props.containerName}/logs?` +
        `podName=${props.podName}&group=${props.group}&kind=${props.kind}&name=${props.name}`;
    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {prefs => (
                <React.Fragment>
                    <div className='pod-logs-viewer__settings'>
                        <button
                            className='argo-button argo-button--base'
                            style={{width: '100px'}}
                            onClick={async () => {
                                try {
                                    await navigator.clipboard.writeText(
                                        loader
                                            .getData()
                                            .map(item => item.content)
                                            .join('\n')
                                    );
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
                        </button>
                        <button
                            className={classNames(`argo-button argo-button--base${prefs.appDetails.followLogs && page.number === 0 ? '' : '-o'}`, {
                                disabled: page.number > 0
                            })}
                            style={{width: '110px'}}
                            onClick={() => {
                                if (page.number > 0) {
                                    return;
                                }
                                const follow = !prefs.appDetails.followLogs;
                                services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, followLogs: follow}});
                                if (follow) {
                                    setPage({number: 0, untilTimes: []});
                                }
                                loader.reload();
                            }}>
                            FOLLOW {prefs.appDetails.followLogs && <i className='fa fa-check' />}
                        </button>
                        <button
                            className='argo-button argo-button--base-o'
                            onClick={() => {
                                const inverted = prefs.appDetails.darkMode;
                                services.viewPreferences.updatePreferences({...prefs, appDetails: {...prefs.appDetails, darkMode: !inverted}});
                            }}>
                            {prefs.appDetails.darkMode ? <i className='fa fa-sun' /> : <i className='fa fa-moon' />}
                        </button>
                        {!props.fullscreen && (
                            <Link to={fullscreenURL} target='_blank' className='argo-button argo-button--base'>
                                <i className='fa fa-external-link-alt' />
                            </Link>
                        )}
                        <div className='pod-logs-viewer__filter'>
                            <button
                                className={`argo-button argo-button--base${filter.inverse ? '' : '-o'}`}
                                onClick={() => setFilter({...filter, inverse: !filter.inverse})}
                                style={{marginRight: '10px'}}>
                                !
                            </button>
                            <input
                                ref={input => {
                                    if (input) {
                                        Observable.fromEvent(input, 'keyup')
                                            .debounceTime(500)
                                            .subscribe(() => {
                                                if (loader) {
                                                    loader.reload();
                                                }
                                            });
                                    }
                                }}
                                type='text'
                                placeholder='Filter string'
                                className='argo-field'
                                value={filter.literal}
                                onChange={e => setFilter({...filter, literal: e.target.value})}
                                style={{padding: 0}}
                            />
                        </div>
                    </div>
                    <DataLoader
                        ref={l => (loader = l)}
                        loadingRenderer={() => (
                            <div className={`pod-logs-viewer ${prefs.appDetails.darkMode ? 'pod-logs-viewer--inverted' : ''}`}>
                                {logNavigators({}, prefs.appDetails.darkMode, null)}
                                <pre style={{height: '95%', textAlign: 'center'}}>{!prefs.appDetails.followLogs && 'Loading...'}</pre>
                            </div>
                        )}
                        input={props.containerName}
                        load={() => {
                            let logsSource = services.applications
                                .getContainerLogs(
                                    props.applicationName,
                                    props.namespace,
                                    props.podName,
                                    {group: props.group, kind: props.kind, name: props.name},
                                    props.containerName,
                                    maxLines * (page.number + 1),
                                    prefs.appDetails.followLogs && page.number === 0,
                                    page.untilTimes[page.untilTimes.length - 1],
                                    filterQuery()
                                )
                                // show only current page lines
                                .scan((lines, logEntry) => {
                                    // first equal true means retry attempt so we should clear accumulated log entries
                                    if (logEntry.first) {
                                        lines = [logEntry];
                                    } else {
                                        lines.push(logEntry);
                                    }
                                    if (lines.length > maxLines) {
                                        lines.splice(0, lines.length - maxLines);
                                    }
                                    return lines;
                                }, new Array<models.LogEntry>())
                                // accumulate log changes and render only once every 100ms to reduce CPU usage
                                .bufferTime(100)
                                .filter(batch => batch.length > 0)
                                .map(batch => batch[batch.length - 1]);
                            if (prefs.appDetails.followLogs) {
                                logsSource = logsSource.retryWhen(errors => errors.delay(500));
                            }
                            return logsSource;
                        }}>
                        {logs => {
                            logs = logs || [];
                            setTimeout(() => {
                                if (page.number === 0 && prefs.appDetails.followLogs && bottom.current) {
                                    bottom.current.scrollIntoView({behavior: 'smooth'});
                                }
                            });
                            const pods = Array.from(new Set(logs.map(log => log.podName)));
                            const podColors = pods.reduce((colors, pod, i) => colors.set(pod, BASE_COLORS[i % BASE_COLORS.length]), new Map<string, string>());
                            const lines = logs.map(item => item.content);
                            const firstLine = maxLines * page.number + 1;
                            const lastLine = maxLines * page.number + lines.length;
                            const canPageBack = lines.length === maxLines;
                            return (
                                <div
                                    className={classNames('pod-logs-viewer', {
                                        'pod-logs-viewer--inverted': prefs.appDetails.darkMode,
                                        'pod-logs-viewer--pod-name-visible': viewPodNames
                                    })}>
                                    {logNavigators(
                                        {
                                            left: () => {
                                                if (!canPageBack) {
                                                    return;
                                                }
                                                setPage({number: page.number + 1, untilTimes: page.untilTimes.concat(logs[0].timeStampStr)});
                                                loader.reload();
                                            },
                                            bottom: () => {
                                                bottom.current.scrollIntoView({
                                                    behavior: 'smooth'
                                                });
                                            },
                                            top: () => {
                                                top.current.scrollIntoView({
                                                    behavior: 'smooth'
                                                });
                                            },
                                            right: () => {
                                                if (page.number > 0) {
                                                    setPage({number: page.number - 1, untilTimes: page.untilTimes.slice(0, page.untilTimes.length - 1)});
                                                    loader.reload();
                                                }
                                            },
                                            end: () => {
                                                setPage({number: 0, untilTimes: []});
                                                loader.reload();
                                            }
                                        },
                                        prefs.appDetails.darkMode,
                                        {
                                            firstLine,
                                            lastLine,
                                            curPage: page.number,
                                            canPageBack
                                        }
                                    )}
                                    {!props.podName && (
                                        <Tooltip content={viewPodNames ? 'Hide pod names' : 'Show pod names'}>
                                            <i
                                                className={classNames('fa pod-logs-viewer__pod-name-toggle', {'fa-chevron-left': viewPodNames, 'fa-chevron-right': !viewPodNames})}
                                                onClick={() => setViewPodNames(!viewPodNames)}
                                            />
                                        </Tooltip>
                                    )}
                                    <pre style={{height: '95%'}}>
                                        <div ref={top} style={{height: '1px'}} />
                                        {lines.map((l, i) => {
                                            const lineNum = lastLine - i;
                                            return (
                                                <div
                                                    key={lineNum}
                                                    style={{display: 'flex', cursor: 'pointer'}}
                                                    onClick={() => {
                                                        setSelectedLine(selectedLine === i ? -1 : i);
                                                    }}>
                                                    <div className={`pod-logs-viewer__line__menu ${selectedLine === i ? 'pod-logs-viewer__line__menu--visible' : ''}`}>
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
                                                                        await navigator.clipboard.writeText(JSON.stringify(lineNum));
                                                                    }
                                                                }
                                                            ]}
                                                        />
                                                    </div>
                                                    {!props.podName && (
                                                        <div className='pod-logs-viewer__line__pod' style={{color: podColors.get(logs[i].podName)}}>
                                                            {(i === 0 || logs[i - 1].podName !== logs[i].podName) && (
                                                                <React.Fragment>
                                                                    <Tooltip content={logs[i].podName}>
                                                                        <span>{logs[i].podName}</span>
                                                                    </Tooltip>
                                                                    <Tooltip content={logs[i].podName}>
                                                                        <i className='fa fa-circle' />
                                                                    </Tooltip>
                                                                </React.Fragment>
                                                            )}
                                                        </div>
                                                    )}
                                                    <div className='pod-logs-viewer__line__number'>{lineNum}</div>
                                                    <div className={`pod-logs-viewer__line ${selectedLine === i ? 'pod-logs-viewer__line--selected' : ''}`}>{l}</div>
                                                </div>
                                            );
                                        })}
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
    top?: () => void;
}

interface PageInfo {
    firstLine: number;
    lastLine: number;
    curPage: number;
    canPageBack: boolean;
}

const logNavigators = (actions: NavActions, darkMode: boolean, info?: PageInfo) => {
    return (
        <div className={`pod-logs-viewer__menu ${darkMode ? 'pod-logs-viewer__menu--inverted' : ''}`}>
            {actions.begin && <i className='fa fa-angle-double-left' onClick={actions.begin || (() => null)} />}
            <i className={`fa fa-angle-left ${info && info.canPageBack ? '' : 'disabled'}`} onClick={actions.left || (() => null)} />
            <i className='fa fa-angle-down' onClick={actions.bottom || (() => null)} />
            <i className='fa fa-angle-up' onClick={actions.top || (() => null)} />
            <div style={{marginLeft: 'auto', marginRight: 'auto'}}>
                {info && (
                    <React.Fragment>
                        Page {info.curPage + 1} (Lines {info.firstLine} to {info.lastLine})
                    </React.Fragment>
                )}
            </div>
            <i className={`fa fa-angle-right ${info && info.curPage > 0 ? '' : 'disabled'}`} onClick={(info && info.curPage > 0 && actions.right) || null} />
            <i className={`fa fa-angle-double-right ${info && info.curPage > 1 ? '' : 'disabled'}`} onClick={(info && info.curPage > 1 && actions.end) || null} />
        </div>
    );
};
