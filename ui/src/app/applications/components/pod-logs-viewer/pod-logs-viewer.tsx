import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';
import {bufferTime, delay, filter as rxfilter, map, retryWhen, scan} from 'rxjs/operators';
import {Terminal} from 'xterm';
import {FitAddon} from 'xterm-addon-fit';

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
import {Spacer} from '../../../shared/components/spacer';
import {Filter} from './filter';
import {TimeRangeSelector} from './time-range-selector';
import {TailSelector} from './tail-selector';
import {Since} from '../../../shared/services/since';
import {PodNamesToggleButton} from './pod-names-toggle-button';
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
    containerStates: models.PodSpec[];
    onClickContainer?: (group: any, i: number, tab: string) => void;
}

// ansi colors, see https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
const gray = '\u001b[90m';
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
export const PodsLogsViewer = (props: PodLogsProps) => {
    const {containerName, onClickContainer, timestamp, containerGroups, applicationName, applicationNamespace, namespace, podName, group, kind, name} = props;
    if (!containerName || containerName === '') {
        return <div>Pod does not have container with name {containerName}</div>;
    }

    const queryParams = new URLSearchParams(location.search);
    const [viewPodNames, setViewPodNames] = useState(queryParams.get('viewPodNames') === 'true');
    const [follow, setFollow] = useState(queryParams.get('follow') === 'true');
    const [viewTimestamps, setViewTimestamps] = useState(queryParams.get('viewTimestamps') === 'true');
    const [previous, setPreviousLogs] = useState(queryParams.get('showPreviousLogs') === 'true');
    const [tail, setTail] = useState<number>(parseInt(queryParams.get('tail'), 10) || 100);
    const [since, setSince] = useState<Since>('1m');
    const [filter, setFilter] = useState(queryParams.get('filterText') || '');
    const [highlight, setHighlight] = useState('');

    const bottom = useRef<HTMLInputElement>(null);
    const loaderRef = useRef();

    const loader: LogLoader = loaderRef.current;

    const scrollToBottom = () => bottom.current?.scrollIntoView({behavior: 'auto'});

    const query = {
        applicationName,
        appNamespace: applicationNamespace,
        namespace,
        podName,
        resource: {group, kind, name},
        containerName,
        tail,
        follow,
        since,
        filter,
        previous
    };

    useEffect(() => {
        if (viewPodNames) {
            setViewTimestamps(false);
        }
    }, [viewPodNames]);

    useEffect(() => {
        const to = setTimeout(() => {
            loader?.reload();
            setHighlight(filter.replace(/[-\/\\^$*+?.()|[\]{}]/g, '\\$&')); // https://stackoverflow.com/questions/3561493/is-there-a-regexp-escape-function-in-javascript
        }, 250);
        return () => clearTimeout(to);
    }, [applicationName, applicationNamespace, namespace, podName, group, kind, name, containerName, tail, follow, since, filter, previous]);

    return (
        <DataLoader load={() => services.viewPreferences.getPreferences()}>
            {(prefs: ViewPreferences) => {
                return (
                    <React.Fragment>
                        <div className='pod-logs-viewer__settings'>
                            <span>
                                <ContainerSelector containerGroups={containerGroups} containerName={containerName} onClickContainer={onClickContainer} />
                                <Spacer />
                                <TailSelector tail={tail} setTail={setTail} />
                                <Spacer />
                                <TimeRangeSelector since={since} setSince={setSince} />
                                <Spacer />
                                <Filter filterText={filter} setFilterText={setFilter} />
                                <Spacer />
                                <ShowPreviousLogsToggleButton loader={loader} setPreviousLogs={setPreviousLogs} showPreviousLogs={previous} />
                                <FollowToggleButton follow={follow} setFollow={setFollow} />
                            </span>
                            <Spacer />
                            <span>
                                <PodNamesToggleButton viewPodNames={viewPodNames} setViewPodNames={setViewPodNames} />
                                <TimestampsToggleButton setViewTimestamps={setViewTimestamps} viewTimestamps={viewTimestamps} timestamp={timestamp} />
                                <WrapLinesToggleButton prefs={prefs} />
                                <DarkModeToggleButton prefs={prefs} />
                            </span>
                            <Spacer />
                            <span>
                                <CopyLogsButton loader={loader} />
                                <DownloadLogsButton {...props} />
                                <FullscreenButton {...props} />
                            </span>
                        </div>

                        <div
                            className={classNames('pod-logs-viewer', {
                                'pod-logs-viewer--inverted': prefs.appDetails.darkMode
                            })}>
                            <pre
                                style={{
                                    height: '100%',
                                    whiteSpace: prefs.appDetails.wrapLines ? 'normal' : 'pre'
                                }}>
                                <DataLoader
                                    ref={loaderRef}
                                    loadingRenderer={() => <>Loading...</>}
                                    input={containerName}
                                    load={() => {
                                        let logsSource = services.applications
                                            .getContainerLogs(query)
                                            // show only current page lines
                                            .pipe(
                                                scan((lines, logEntry) => {
                                                    // first equal true means retry attempt so we should clear accumulated log entries
                                                    if (logEntry.first) {
                                                        lines = [logEntry];
                                                    } else {
                                                        lines.push(logEntry);
                                                    }
                                                    if (lines.length > tail) {
                                                        lines.splice(0, lines.length - tail);
                                                    }
                                                    return lines;
                                                }, new Array<models.LogEntry>())
                                            )
                                            // accumulate log changes and render only once every 100ms to reduce CPU usage
                                            .pipe(bufferTime(100))
                                            .pipe(rxfilter(batch => batch.length > 0))
                                            .pipe(map(batch => batch[batch.length - 1]));
                                        if (follow) {
                                            logsSource = logsSource.pipe(retryWhen(errors => errors.pipe(delay(500))));
                                        }
                                        return logsSource;
                                    }}>
                                    {(logs: any[]) => {
                                        logs = logs || [];
                                        setTimeout(() => {
                                            scrollToBottom();
                                        });
                                        const lineNumberWidth = String(tail).length;
                                        const lineNumberLeftPadding = ' '.repeat(lineNumberWidth);
                                        return (
                                            <Ansi>
                                                {logs
                                                    .map(
                                                        (log, lineNum) =>
                                                            // show the pod name if there are multiple pods, pad with spaces to align
                                                            (viewPodNames
                                                                ? (lineNum === 0 || logs[lineNum - 1].podName !== log.podName
                                                                      ? podColor(podName) + log.podName + reset
                                                                      : ' '.repeat(log.podName.length)) + ' '
                                                                : '') +
                                                            // show the timestamp if requested, pad with spaces to align
                                                            (viewTimestamps
                                                                ? lineNum === 0 || logs[lineNum - 1].timeStamp !== log.timeStamp
                                                                    ? log.timeStampStr
                                                                    : ' '.repeat(log.timeStampStr)
                                                                : '') +
                                                            // show the line number, in gray
                                                            (gray + (lineNumberLeftPadding + lineNum).slice(-lineNumberWidth) + reset) +
                                                            ' ' +
                                                            // show the log content, highlight the filter text
                                                            log.content.replace(new RegExp(highlight, 'g'), (substring: string) => whiteOnYellow + substring + reset)
                                                    )
                                                    .join('\n')}
                                            </Ansi>
                                        );
                                    }}
                                </DataLoader>

                                <div ref={bottom} style={{height: '1px'}} />
                            </pre>
                        </div>
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
