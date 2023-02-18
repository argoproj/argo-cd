import {DataLoader, DropDownMenu, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import {useEffect, useRef, useState} from 'react';
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
import {Spacer} from '../../../shared/components/spacer';
import {Filter} from './filter';
import {TimeRangeSelector} from './time-range-selector';
import {TailSelector} from './tail-selector';
import {Since} from "../../../shared/services/since";

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

export const PodsLogsViewer = (props: PodLogsProps) => {
    const {containerName, onClickContainer, timestamp, containerGroups, applicationName, applicationNamespace, namespace, podName, group, kind, name} = props;
    if (!containerName || containerName === '') {
        return <div>Pod does not have container with name {containerName}</div>;
    }

    const queryParams = new URLSearchParams(location.search);
    const [selectedLine, setSelectedLine] = useState(parseInt(queryParams.get('selectedLine'), 10) || -1);
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
                                <TimestampsToggleButton
                                    setViewPodNames={setViewPodNames}
                                    viewPodNames={viewPodNames}
                                    setViewTimestamps={setViewTimestamps}
                                    viewTimestamps={viewTimestamps}
                                    timestamp={timestamp}
                                />
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
                        <DataLoader
                            ref={loaderRef}
                            loadingRenderer={() => (
                                <div className={`pod-logs-viewer ${prefs.appDetails.darkMode ? 'pod-logs-viewer--inverted' : ''}`}>
                                    <pre style={{height: '95%', textAlign: 'center'}}>Loading...</pre>
                                </div>
                            )}
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
                                const pods = Array.from(new Set(logs.map(log => log.podName)));
                                const podColors = pods.reduce((colors, pod, i) => colors.set(pod, BASE_COLORS[i % BASE_COLORS.length]), new Map<string, string>());
                                const lines = logs.map(item => item.content);
                                return (
                                    <div
                                        className={classNames('pod-logs-viewer', {
                                            'pod-logs-viewer--inverted': prefs.appDetails.darkMode,
                                            'pod-logs-viewer--pod-name-visible': viewPodNames,
                                            'pod-logs-viewer--pod-timestamp-visible': viewTimestamps
                                        })}>
                                        {!podName && (
                                            <Tooltip content={viewPodNames ? 'Hide pod names' : 'Show pod names'}>
                                                <i
                                                    className={classNames('fa pod-logs-viewer__pod-name-toggle', {
                                                        'fa-chevron-left': viewPodNames,
                                                        'fa-chevron-right': !viewPodNames
                                                    })}
                                                    onClick={() => {
                                                        setViewPodNames(!viewPodNames);
                                                        if (viewTimestamps) {
                                                            setViewTimestamps(false);
                                                        }
                                                    }}
                                                />
                                            </Tooltip>
                                        )}
                                        <pre
                                            style={{
                                                height: '95%',
                                                whiteSpace: prefs.appDetails.wrapLines ? 'normal' : 'pre'
                                            }}>
                                            {lines.map((l, i) => {
                                                const lineNum = i;
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
                                                                                <i className='fa fa-clipboard' /> Copy Line
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
                                                        {!podName && (
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
                                                            <Ansi>{l.replace(new RegExp(highlight, 'g'), (y: string) => '\u001b[1m\u001b[43;1m\u001b[37m' + y + '\u001b[0m')}</Ansi>
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
                );
            }}
        </DataLoader>
    );
};
