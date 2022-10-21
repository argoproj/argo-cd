import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useRef, useState} from 'react';
import {bufferTime, delay, filter as rxfilter, map, retryWhen, scan} from 'rxjs/operators';
import Ansi from 'ansi-to-react';
import * as models from '../../../shared/models';
import {services, ViewPreferences} from '../../../shared/services';

import {BASE_COLORS} from '../utils';

import './pod-logs-viewer.scss';
import {CopyLogsButton} from './copy-logs-button';
import {DownloadLogsButton} from './download-logs-button';
import {ContainerSelector} from './container-selector';
import {FollowToggleButton} from './follow-toggle-button';
import {WrapLinesToggleButton} from './wrap-lines-toggle-button';
import {LogLoader} from './log-loader';
import {ShowPreviousLogsToggleButton} from './show-previous-logs-toggle-button';
import {TimestampsToggleButton} from './timestamps-toggle-button';
import {DarkModeToggleButton} from './dark-mode-toggle-button';
import {FullscreenButton} from './fullscreen-button';
import {ButtonGroup} from '../../../shared/components/button-group';
import {ToggleButton} from '../../../shared/components/toggle-button';
import { Spacer } from '../../../shared/components/spacer';

const maxLines = 100;
export interface PodLogsProps {
    namespace: string;
    applicationNamespace: string;
    applicationName: string;
    podName?: string;
    containerName: string;
    group?: string;
    kind?: string;
    name?: string;
    page: {number: number; untilTimes: string[]};
    timestamp?: string;
    setPage: (pageData: {number: number; untilTimes: string[]}) => void;
    containerGroups?: any[];
    onClickContainer?: (group: any, i: number, tab: string) => any;
}

export const PodsLogsViewer = (props: PodLogsProps & {fullscreen?: boolean}) => {
    const {containerName, onClickContainer} = props;
    if (!containerName || containerName === '') {
        return <div>Pod does not have container with name {props.containerName}</div>;
    }

    const [selectedLine, setSelectedLine] = useState(-1);
    const bottom = React.useRef<HTMLInputElement>(null);
    const top = React.useRef<HTMLInputElement>(null);
    const page = props.page;
    const setPage = props.setPage;
    const [viewPodNames, setViewPodNames] = useState(false);
    const [viewTimestamps, setViewTimestamps] = useState(false);
    const [showPreviousLogs, setPreviousLogs] = useState(false);

    interface FilterData {
        literal: string;
        inverse: boolean;
    }

    const [filterText, setFilterText] = useState('');
    const [filter, setFilter] = useState({inverse: false, literal: ''} as FilterData);

    const formatFilter = (f: FilterData): string => {
        return f.literal && `${f.inverse ? '!' : ''}${f.literal}`;
    };

    const [filterQuery, setFilterQuery] = React.useState(formatFilter(filter));
    React.useEffect(() => {
        setFilterQuery(formatFilter(filter));
        if (loader) {
            loader.reload();
        }
    }, [filter]);

    React.useEffect(() => {
        const to = setTimeout(() => {
            setFilter({...filter, literal: filterText});
        }, 500);
        return () => clearTimeout(to);
    }, [filterText]);

    const loaderRef = useRef();

    const loader: LogLoader = loaderRef.current;
    const grep = filter.literal.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&'); //https://stackoverflow.com/questions/3561493/is-there-a-regexp-escape-function-in-javascript

    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {(prefs: ViewPreferences) => (
                <React.Fragment>
                    <div className='pod-logs-viewer__settings'>
                        <ButtonGroup>
                            <ContainerSelector containerGroups={props.containerGroups} containerName={containerName} onClickContainer={onClickContainer} />
                        </ButtonGroup>
                        <Spacer/>
                        <ButtonGroup>
                            <ShowPreviousLogsToggleButton loader={loader} setPreviousLogs={setPreviousLogs} showPreviousLogs={showPreviousLogs} />
                            <TimestampsToggleButton
                                setViewPodNames={setViewPodNames}
                                viewPodNames={viewPodNames}
                                setViewTimestamps={setViewTimestamps}
                                viewTimestamps={viewTimestamps}
                                timestamp={props.timestamp}
                            />
                            <WrapLinesToggleButton prefs={prefs} />
                            <FollowToggleButton page={page} setPage={setPage} prefs={prefs} loader={loader} />
                            <DarkModeToggleButton prefs={prefs} />
                        </ButtonGroup>
                        <Spacer/>
                        <ButtonGroup>
                            <CopyLogsButton loader={loader} />
                            <DownloadLogsButton {...props} />
                            <FullscreenButton {...props} />
                        </ButtonGroup>

                        <div className='pod-logs-viewer__filter'>
                            <ToggleButton
                                toggled={filter.inverse}
                                onToggle={() => setFilter({...filter, inverse: !filter.inverse})}
                                title='Show lines that do not match'
                                icon='exclamation'
                            />
                            <input
                                type='text'
                                placeholder={`Filter ${filter.inverse ? 'out' : ''} string`}
                                className='argo-field'
                                value={filterText}
                                onChange={e => setFilterText(e.target.value)}
                                style={{padding: 0}}
                            />
                        </div>
                    </div>
                    <DataLoader
                        ref={loaderRef}
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
                                    props.applicationNamespace,
                                    props.namespace,
                                    props.podName,
                                    {group: props.group, kind: props.kind, name: props.name},
                                    props.containerName,
                                    maxLines * (page.number + 1),
                                    prefs.appDetails.followLogs && page.number === 0,
                                    page.untilTimes[page.untilTimes.length - 1],
                                    filterQuery,
                                    showPreviousLogs
                                )
                                // show only current page lines
                                .pipe(
                                    scan((lines, logEntry) => {
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
                                )
                                // accumulate log changes and render only once every 100ms to reduce CPU usage
                                .pipe(bufferTime(100))
                                .pipe(rxfilter(batch => batch.length > 0))
                                .pipe(map(batch => batch[batch.length - 1]));
                            if (prefs.appDetails.followLogs) {
                                logsSource = logsSource.pipe(retryWhen(errors => errors.pipe(delay(500))));
                            }
                            return logsSource;
                        }}>
                        {(logs:any[]) => {
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
                                        'pod-logs-viewer--pod-name-visible': viewPodNames,
                                        'pod-logs-viewer--pod-timestamp-visible': viewTimestamps
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
                                                onClick={() => {
                                                    setViewPodNames(!viewPodNames);
                                                    if (viewTimestamps) {
                                                        setViewTimestamps(false);
                                                    }
                                                }}
                                            />
                                        </Tooltip>
                                    )}
                                    <pre style={{height: '95%', whiteSpace: prefs.appDetails.wrapLines ? 'normal' : 'pre'}}>
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
                                                                        <Tooltip content='Copy'>
                                                                            <span>
                                                                                <i className='fa fa-clipboard' />
                                                                            </span>
                                                                        </Tooltip>
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
                                                    {viewTimestamps && (
                                                        <div className='pod-logs-viewer__line__timestamp'>
                                                            {(i === 0 || logs[i - 1].timeStamp !== logs[i].timeStamp) && (
                                                                <React.Fragment>
                                                                    <Tooltip content={logs[i].timeStampStr}>
                                                                        <span>{logs[i].timeStampStr}</span>
                                                                    </Tooltip>
                                                                </React.Fragment>
                                                            )}
                                                        </div>
                                                    )}
                                                    <div className='pod-logs-viewer__line__number'>{lineNum}</div>
                                                    <div className={`pod-logs-viewer__line ${selectedLine === i ? 'pod-logs-viewer__line--selected' : ''}`}>
                                                        <Ansi>{l.replace(new RegExp(grep, 'g'), (y:string) => '\u001b[1m\u001b[43;1m\u001b[37m' + y + '\u001b[0m')}</Ansi>
                                                    </div>
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
            <>
                <>
                    Lines {info?.firstLine} to {info?.lastLine}&nbsp;
                </>
                <i title='Top' className='fa fa-arrow-up' onClick={actions.top} />
                <i title='Bottom' className='fa fa-arrow-down' onClick={actions.bottom} />
            </>
            <div style={{marginLeft: 'auto', marginRight: 'auto'}} />
            <>
                <>
                    Page {info?.curPage + 1}
                    &nbsp;
                </>
                <i title='First page' className={`fa fa-backward-fast $ ${actions.begin ? '' : 'disabled'}`} onClick={actions.begin} />
                <i title='Previous page' className={`fa fa-backward-step ${info?.canPageBack ? '' : 'disabled'}`} onClick={actions.left} />
                <i title='Next page' className={`fa fa-forward-step ${info?.curPage > 0 ? '' : 'disabled'}`} onClick={actions.right} />
                <i title='Last page' className={`fa fa-forward-fast ${info?.curPage > 1 ? '' : 'disabled'}`} onClick={actions.end} />
            </>
        </div>
    );
};
