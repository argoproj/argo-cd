import {DataLoader} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useEffect, useState, useRef} from 'react';
import {bufferTime, delay, retryWhen} from 'rxjs/operators';

import {LogEntry} from '../../../shared/models';
import {services, ViewPreferences} from '../../../shared/services';

import AutoSizer from 'react-virtualized/dist/commonjs/AutoSizer';

import './pod-logs-viewer.scss';
import {CopyLogsButton} from './copy-logs-button';
import {DownloadLogsButton} from './download-logs-button';
import {ContainerSelector} from './container-selector';
import {FollowToggleButton} from './follow-toggle-button';
import {ShowPreviousLogsToggleButton} from './show-previous-logs-toggle-button';
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
import Ansi from 'ansi-to-react';

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
}

// ansi colors, see https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
const red = '\u001b[31m';
const green = '\u001b[32m';
const yellow = '\u001b[33m';
const blue = '\u001b[34m';
const magenta = '\u001b[35m';
const cyan = '\u001b[36m';
const colors = [red, green, yellow, blue, magenta, cyan];
const reset = '\u001b[0m';
const whiteOnYellow = '\u001b[1m\u001b[43;1m\u001b[37m';

// cheap string hash function
function stringHashCode(str: string) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        // tslint:disable-next-line:no-bitwise
        hash = str.charCodeAt(i) + ((hash << 5) - hash);
    }
    return hash;
}

// ansi color for pod name
function podColor(podName: string) {
    return colors[stringHashCode(podName) % colors.length];
}

// https://2ality.com/2012/09/empty-regexp.html
const matchNothing = /.^/;

export const PodsLogsViewer = (props: PodLogsProps) => {
    const {containerName, onClickContainer, timestamp, containerGroups, applicationName, applicationNamespace, namespace, podName, group, kind, name} = props;
    const queryParams = new URLSearchParams(location.search);
    const [viewPodNames, setViewPodNames] = useState(queryParams.get('viewPodNames') === 'true');
    const [follow, setFollow] = useState(queryParams.get('follow') !== 'false');
    const [viewTimestamps, setViewTimestamps] = useState(queryParams.get('viewTimestamps') === 'true');
    const [previous, setPreviousLogs] = useState(queryParams.get('showPreviousLogs') === 'true');
    const [tail, setTail] = useState<number>(parseInt(queryParams.get('tail'), 10) || 1000);
    const [sinceSeconds, setSinceSeconds] = useState(0);
    const [filter, setFilter] = useState(queryParams.get('filterText') || '');
    const [highlight, setHighlight] = useState<RegExp>(matchNothing);
    const [scrollToBottom, setScrollToBottom] = useState(true);
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const logsContainerRef = useRef(null);

    useEffect(() => {
        if (viewPodNames) {
            setViewTimestamps(false);
        }
    }, [viewPodNames]);

    useEffect(() => {
        // https://stackoverflow.com/questions/3561493/is-there-a-regexp-escape-function-in-javascript
        // matchNothing this is chosen instead of empty regexp, because that would match everything and break colored logs
        setHighlight(filter === '' ? matchNothing : new RegExp(filter.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&'), 'g'));
    }, [filter]);

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
                previous
            }) // accumulate log changes and render only once every 100ms to reduce CPU usage
            .pipe(bufferTime(100))
            .pipe(retryWhen(errors => errors.pipe(delay(500))))
            .subscribe(log => setLogs(previousLogs => previousLogs.concat(log)));

        return () => logsSource.unsubscribe();
    }, [applicationName, applicationNamespace, namespace, podName, group, kind, name, containerName, tail, follow, sinceSeconds, filter, previous]);

    const handleScroll = (event: React.WheelEvent<HTMLDivElement>) => {
        if (event.deltaY < 0) setScrollToBottom(false);
    };

    const renderLog = (log: LogEntry, lineNum: number) =>
        // show the pod name if there are multiple pods, pad with spaces to align
        (viewPodNames ? (lineNum === 0 || logs[lineNum - 1].podName !== log.podName ? podColor(podName) + log.podName + reset : ' '.repeat(log.podName.length)) + ' ' : '') +
        // show the timestamp if requested, pad with spaces to align
        (viewTimestamps ? (lineNum === 0 || (logs[lineNum - 1].timeStamp !== log.timeStamp ? log.timeStampStr : '').padEnd(30)) + ' ' : '') +
        // show the log content, highlight the filter text
        log.content?.replace(highlight, (substring: string) => whiteOnYellow + substring + reset);
    const logsContent = (width: number, height: number, isWrapped: boolean) => (
        <div ref={logsContainerRef} onScroll={handleScroll} style={{width, height, overflow: 'scroll'}}>
            {logs.map((log, lineNum) => (
                <pre key={lineNum} style={{whiteSpace: isWrapped ? 'normal' : 'pre'}} className='noscroll'>
                    <Ansi>{renderLog(log, lineNum)}</Ansi>
                </pre>
            ))}
        </div>
    );

    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {(prefs: ViewPreferences) => {
                return (
                    <React.Fragment>
                        <div className='pod-logs-viewer__settings'>
                            <span>
                                <FollowToggleButton follow={follow} setFollow={setFollow} />
                                {follow && <AutoScrollButton scrollToBottom={scrollToBottom} setScrollToBottom={setScrollToBottom} />}
                                <ShowPreviousLogsToggleButton setPreviousLogs={setPreviousLogs} showPreviousLogs={previous} />
                                <Spacer />
                                <ContainerSelector containerGroups={containerGroups} containerName={containerName} onClickContainer={onClickContainer} />
                                <Spacer />
                                {!follow && (
                                    <>
                                        <SinceSecondsSelector sinceSeconds={sinceSeconds} setSinceSeconds={n => setSinceSeconds(n)} />
                                        <TailSelector tail={tail} setTail={setTail} />
                                    </>
                                )}
                                <LogMessageFilter filterText={filter} setFilterText={setFilter} />
                            </span>
                            <Spacer />
                            <span>
                                <WrapLinesButton prefs={prefs} />
                                <PodNamesToggleButton viewPodNames={viewPodNames} setViewPodNames={setViewPodNames} />
                                <TimestampsToggleButton setViewTimestamps={setViewTimestamps} viewTimestamps={viewTimestamps} timestamp={timestamp} />
                                <DarkModeToggleButton prefs={prefs} />
                            </span>
                            <Spacer />
                            <span>
                                <CopyLogsButton logs={logs} />
                                <DownloadLogsButton {...props} />
                                <FullscreenButton {...props} />
                            </span>
                        </div>
                        <div className={classNames('pod-logs-viewer', {'pod-logs-viewer--inverted': prefs.appDetails.darkMode})} onWheel={handleScroll}>
                            <AutoSizer>{({width, height}: {width: number; height: number}) => logsContent(width, height, prefs.appDetails.wrapLines)}</AutoSizer>
                        </div>
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
