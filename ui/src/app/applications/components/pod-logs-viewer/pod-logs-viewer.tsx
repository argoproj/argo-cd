import {DataLoader} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useEffect, useState, useRef} from 'react';
import {bufferTime, catchError, delay, retryWhen} from 'rxjs/operators';

import {LogEntry} from '../../../shared/models';
import {services, ViewPreferences} from '../../../shared/services';

import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';

import './pod-logs-viewer.scss';
import {CopyLogsButton} from './copy-logs-button';
import {DownloadLogsButton} from './download-logs-button';
import {ContainerSelector} from './container-selector';
import {FollowToggleButton} from './follow-toggle-button';
import {ShowPreviousLogsToggleButton} from './show-previous-logs-toggle-button';
import {PodHighlightButton} from './pod-logs-highlight-button';
import {TimestampsToggleButton} from './timestamps-toggle-button';
import {DarkModeToggleButton} from './dark-mode-toggle-button';
import {FullscreenButton} from './fullscreen-button';
import {Spacer} from '../../../shared/components/spacer';
import {LogMessageFilter} from './log-message-filter';
import {SinceSecondsSelector} from './since-seconds-selector';
import {TailSelector} from './tail-selector';
import {PodNamesToggleButton} from './pod-names-toggle-button';
import {AutoScrollButton} from './auto-scroll-button';
import {WrapLinesButton} from './wrap-lines-button';
import {MatchCaseToggleButton} from './match-case-toggle-button';
import Ansi from 'ansi-to-react';
import {EMPTY} from 'rxjs';

export interface PodLogsProps {
    namespace: string;
    applicationNamespace: string;
    applicationName: string;
    podName?: string;
    containerName: string;
    group?: string;
    kind?: string;
    name?: string;
    timestamp?: string;
    containerGroups?: any[];
    onClickContainer?: (group: any, i: number, tab: string) => void;
    fullscreen?: boolean;
}

export interface PodLogsQueryProps {
    viewPodNames?: boolean;
    viewTimestamps?: boolean;
    follow?: boolean;
    showPreviousLogs?: boolean;
    filterText?: string;
    tail?: number;
    matchCase?: boolean;
    sinceSeconds?: number;
}

// ansi colors, see https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
const blue = '\u001b[34m';
const magenta = '\u001b[35m';
const colors = [blue, magenta];
const reset = '\u001b[0m';
const whiteOnYellow = '\u001b[1m\u001b[43;1m\u001b[37m';

// Default colors using argo-ui theme variables
const POD_COLORS_LIGHT = ['var(--pod-background-light)'];
const POD_COLORS_DARK = ['var(--pod-background-dark)'];

const getPodColors = (isDark: boolean) => {
    const envColors = (window as any).env?.POD_COLORS?.[isDark ? 'dark' : 'light'];
    return envColors || (isDark ? POD_COLORS_DARK : POD_COLORS_LIGHT);
};

function getPodBackgroundColor(podName: string, darkMode: boolean) {
    const colors = getPodColors(darkMode);
    return colors[0];
}

// ansi color for pod name
function podColor(podName: string, isDarkMode: boolean, isSelected: boolean) {
    if (!isSelected) {
        return '';
    }
    return isDarkMode ? colors[1] : colors[0];
}

// https://2ality.com/2012/09/empty-regexp.html
const matchNothing = /.^/;

export const PodsLogsViewer = (props: PodLogsProps) => {
    const {containerName, onClickContainer, timestamp, containerGroups, applicationName, applicationNamespace, namespace, podName, group, kind, name} = props;
    const queryParams = new URLSearchParams(location.search);
    const [selectedPod, setSelectedPod] = useState<string | null>(null);
    const [viewPodNames, setViewPodNames] = useState(queryParams.get('viewPodNames') === 'true');
    const [follow, setFollow] = useState(queryParams.get('follow') !== 'false');
    const [viewTimestamps, setViewTimestamps] = useState(queryParams.get('viewTimestamps') === 'true');
    const [previous, setPreviousLogs] = useState(queryParams.get('showPreviousLogs') === 'true');
    const [tail, setTail] = useState<number>(parseInt(queryParams.get('tail'), 10) || 1000);
    const [matchCase, setMatchCase] = useState(queryParams.get('matchCase') === 'true');
    const [sinceSeconds, setSinceSeconds] = useState(parseInt(queryParams.get('sinceSeconds'), 10) || 0);
    const [filter, setFilter] = useState(queryParams.get('filterText') || '');
    const [highlight, setHighlight] = useState<RegExp>(matchNothing);
    const [scrollToBottom, setScrollToBottom] = useState(true);
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const logsContainerRef = useRef(null);
    const uniquePods = Array.from(new Set(logs.map(log => log.podName)));
    const [errorMessage, setErrorMessage] = useState<string | null>(null);

    const setWithQueryParams = <T extends (val: any) => void>(key: string, cb: T) => {
        return (val => {
            cb(val);
            queryParams.set(key, val.toString());
            history.replaceState(null, '', `${location.pathname}?${queryParams}`);
        }) as T;
    };

    const setViewPodNamesWithQueryParams = setWithQueryParams('viewPodNames', setViewPodNames);
    const setViewTimestampsWithQueryParams = setWithQueryParams('viewTimestamps', setViewTimestamps);
    const setFollowWithQueryParams = setWithQueryParams('follow', setFollow);
    const setPreviousLogsWithQueryParams = setWithQueryParams('showPreviousLogs', setPreviousLogs);
    const setTailWithQueryParams = setWithQueryParams('tail', setTail);
    const setFilterWithQueryParams = setWithQueryParams('filterText', setFilter);
    const setMatchCaseWithQueryParams = setWithQueryParams('matchCase', setMatchCase);

    const onToggleViewPodNames = (val: boolean) => {
        setViewPodNamesWithQueryParams(val);
        if (val) {
            setViewTimestampsWithQueryParams(false);
        }
    };

    useEffect(() => {
        // https://stackoverflow.com/questions/3561493/is-there-a-regexp-escape-function-in-javascript
        // matchNothing this is chosen instead of empty regexp, because that would match everything and break colored logs
        // eslint-disable-next-line no-useless-escape
        setHighlight(filter === '' ? matchNothing : new RegExp(filter.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&'), 'g' + (matchCase ? '' : 'i')));
    }, [filter, matchCase]);

    if (!containerName || containerName === '') {
        return <div>Pod does not have container with name {containerName}</div>;
    }

    useEffect(() => setScrollToBottom(true), [follow]);

    useEffect(() => {
        if (scrollToBottom) {
            const element = logsContainerRef.current;
            if (element) {
                element.scrollTop = element.scrollHeight;
            }
        }
    }, [logs, scrollToBottom]);

    useEffect(() => {
        setLogs([]);
        const logsSource = services.applications
            .getContainerLogs({
                applicationName,
                appNamespace: applicationNamespace,
                namespace,
                podName,
                resource: {group, kind, name},
                containerName,
                tail,
                follow,
                sinceSeconds,
                filter,
                previous,
                matchCase
            })
            .pipe(
                bufferTime(100),
                catchError((error: any) => {
                    const errorBody = JSON.parse(error.body);
                    if (errorBody.error && errorBody.error.message) {
                        if (errorBody.error.message.includes('max pods to view logs are reached')) {
                            setErrorMessage('Max pods to view logs are reached. Please provide more granular query.');
                            return EMPTY; // Non-retryable condition, stop the stream and display the error message.
                        }
                    }
                }),
                retryWhen(errors => errors.pipe(delay(500)))
            )
            .subscribe(log => {
                if (log.length) {
                    setLogs(previousLogs => previousLogs.concat(log));
                }
            });

        return () => logsSource.unsubscribe();
    }, [applicationName, applicationNamespace, namespace, podName, group, kind, name, containerName, tail, follow, sinceSeconds, filter, previous, matchCase]);

    const handleScroll = (event: React.WheelEvent<HTMLDivElement>) => {
        if (event.deltaY < 0) setScrollToBottom(false);
    };

    const renderLog = (log: LogEntry, lineNum: number, darkMode: boolean) => {
        const podNameContent = viewPodNames
            ? (lineNum === 0 || logs[lineNum - 1].podName !== log.podName
                  ? `${podColor(log.podName, darkMode, selectedPod === log.podName)}${log.podName}${reset}`
                  : ' '.repeat(log.podName.length)) + ' '
            : '';

        // show the timestamp if requested, pad with spaces to align
        const timestampContent = viewTimestamps ? (lineNum === 0 || logs[lineNum - 1].timeStamp !== log.timeStamp ? log.timeStampStr : '').padEnd(30) + ' ' : '';

        // show the log content without colors, only highlight search terms
        const logContent = log.content?.replace(highlight, (substring: string) => whiteOnYellow + substring + reset);

        return {podNameContent, timestampContent, logContent};
    };

    const logsContent = (width: number, height: number, isWrapped: boolean, prefs: ViewPreferences) => (
        <div
            ref={logsContainerRef}
            onScroll={handleScroll}
            style={{
                width,
                height,
                overflow: 'scroll',
                minWidth: '100%'
            }}>
            <div
                style={{
                    width: '100%',
                    minWidth: 'fit-content'
                }}>
                {logs.map((log, lineNum) => {
                    const {podNameContent, timestampContent, logContent} = renderLog(log, lineNum, prefs.appDetails.darkMode);
                    return (
                        <div
                            key={lineNum}
                            style={{
                                whiteSpace: isWrapped ? 'normal' : 'pre',
                                lineHeight: '1.5rem',
                                backgroundColor: selectedPod === log.podName ? getPodBackgroundColor(log.podName, prefs.appDetails.darkMode) : 'transparent',
                                padding: '1px 8px',
                                width: '100vw',
                                marginLeft: '-8px',
                                marginRight: '-8px'
                            }}
                            className='noscroll'>
                            {viewPodNames && (lineNum === 0 || logs[lineNum - 1].podName !== log.podName) && (
                                <span onClick={() => setSelectedPod(selectedPod === log.podName ? null : log.podName)} style={{cursor: 'pointer'}} className='pod-name-link'>
                                    <Ansi>{podNameContent}</Ansi>
                                </span>
                            )}
                            {viewPodNames && !(lineNum === 0 || logs[lineNum - 1].podName !== log.podName) && (
                                <span>
                                    <Ansi>{podNameContent}</Ansi>
                                </span>
                            )}
                            <Ansi>{timestampContent + logContent}</Ansi>
                        </div>
                    );
                })}
            </div>
        </div>
    );

    const preferenceLoader = React.useCallback(() => services.viewPreferences.getPreferences(), []);
    return (
        <DataLoader load={preferenceLoader}>
            {(prefs: ViewPreferences) => {
                return (
                    <React.Fragment>
                        <div className='pod-logs-viewer__settings'>
                            <span>
                                <FollowToggleButton follow={follow} setFollow={setFollowWithQueryParams} />
                                {follow && <AutoScrollButton scrollToBottom={scrollToBottom} setScrollToBottom={setScrollToBottom} />}
                                <ShowPreviousLogsToggleButton setPreviousLogs={setPreviousLogsWithQueryParams} showPreviousLogs={previous} />
                                <Spacer />
                                <PodHighlightButton selectedPod={selectedPod} setSelectedPod={setSelectedPod} pods={uniquePods} darkMode={prefs.appDetails.darkMode} />
                                <Spacer />
                                <ContainerSelector containerGroups={containerGroups} containerName={containerName} onClickContainer={onClickContainer} />
                                <Spacer />
                                {!follow && (
                                    <>
                                        <SinceSecondsSelector sinceSeconds={sinceSeconds} setSinceSeconds={n => setSinceSeconds(n)} />
                                        <TailSelector tail={tail} setTail={setTailWithQueryParams} />
                                    </>
                                )}
                                <LogMessageFilter filterText={filter} setFilterText={setFilterWithQueryParams} />
                            </span>
                            <Spacer />
                            <span>
                                <MatchCaseToggleButton matchCase={matchCase} setMatchCase={setMatchCaseWithQueryParams} />
                                <WrapLinesButton prefs={prefs} />
                                <PodNamesToggleButton viewPodNames={viewPodNames} setViewPodNames={onToggleViewPodNames} />
                                <TimestampsToggleButton setViewTimestamps={setViewTimestampsWithQueryParams} viewTimestamps={viewTimestamps} timestamp={timestamp} />
                                <DarkModeToggleButton prefs={prefs} />
                            </span>
                            <Spacer />
                            <span>
                                <CopyLogsButton logs={logs} />
                                <DownloadLogsButton {...props} />
                                <FullscreenButton
                                    {...props}
                                    viewPodNames={viewPodNames}
                                    viewTimestamps={viewTimestamps}
                                    follow={follow}
                                    showPreviousLogs={previous}
                                    filterText={filter}
                                    matchCase={matchCase}
                                    tail={tail}
                                    sinceSeconds={sinceSeconds}
                                />
                            </span>
                        </div>
                        <div className={classNames('pod-logs-viewer', {'pod-logs-viewer--inverted': prefs.appDetails.darkMode})} onWheel={handleScroll}>
                            {errorMessage ? (
                                <div>{errorMessage}</div>
                            ) : (
                                <AutoSizer>{({width, height}: {width: number; height: number}) => logsContent(width, height, prefs.appDetails.wrapLines, prefs)}</AutoSizer>
                            )}
                        </div>
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
