import {Tooltip} from 'argo-ui';
import React, {ChangeEvent} from 'react';
import * as ReactForm from 'react-form';
import {SyncWindow} from '../../../shared/models';

require('./project-sync-windows-edit.scss');
interface ProjectSyncWindowProps {
    projName: string;
    window: SyncWindow;
    formApi: ReactForm.FormApi;
}

function helpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}>
                {' '}
                <i className='fas fa-info-circle' />
            </span>
        </Tooltip>
    );
}

export const ProjectSyncWindowApplicationsEdit = (props: ProjectSyncWindowProps) => (
    <>
        <p>APPLICATIONS</p>
        <div>Manage applications assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.applications || []).map((a, i) => (
                <Attribute
                    key={i}
                    field={['window.applications', i]}
                    formApi={props.formApi}
                    projName={props.projName}
                    deleteApp={() => props.formApi.setValue('window.applications', removeEl(props.window.applications, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a
                        className='argo-button argo-button--base'
                        onClick={() => {
                            const newA = '';
                            props.formApi.setValue('window.applications', (props.formApi.values.window.applications || []).concat(newA));
                        }}>
                        Add Application
                    </a>
                </div>
            </div>
        </div>
    </>
);

export const ProjectSyncWindowNamespaceEdit = (props: ProjectSyncWindowProps) => (
    <>
        <p>NAMESPACES</p>
        <div>Manage namespaces assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.namespaces || []).map((n, i) => (
                <Attribute
                    key={i}
                    field={['window.namespaces', i]}
                    formApi={props.formApi}
                    projName={props.projName}
                    deleteApp={() => props.formApi.setValue('window.namespaces', removeEl(props.window.namespaces, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a
                        className='argo-button argo-button--base'
                        onClick={() => {
                            const newN = '';
                            props.formApi.setValue('window.namespaces', (props.formApi.values.window.namespaces || []).concat(newN));
                        }}>
                        Add Namespace
                    </a>
                </div>
            </div>
        </div>
    </>
);

export const ProjectSyncWindowClusterEdit = (props: ProjectSyncWindowProps) => (
    <>
        <p>CLUSTERS</p>
        <div>Manage clusters assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.clusters || []).map((c, i) => (
                <Attribute
                    key={i}
                    field={['window.clusters', i]}
                    formApi={props.formApi}
                    projName={props.projName}
                    deleteApp={() => props.formApi.setValue('window.clusters', removeEl(props.window.clusters, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a
                        className='argo-button argo-button--base'
                        onClick={() => {
                            const newC = '';
                            props.formApi.setValue('window.clusters', (props.formApi.values.window.clusters || []).concat(newC));
                        }}>
                        Add Cluster
                    </a>
                </div>
            </div>
        </div>
    </>
);

interface AttributeProps {
    projName: string;
    roleName: string;
    fieldApi: ReactForm.FieldApi;
    deleteApp: () => void;
}

function removeEl(items: any[], index: number) {
    items.splice(index, 1);
    return items;
}

function AttributeWrapper({fieldApi, deleteApp}: AttributeProps) {
    const getApplication = () => fieldApi.getValue();

    const setApplication = (application: string) => fieldApi.setValue(application);

    return (
        <div className='row'>
            <div className='columns small-6'>
                <input
                    className='argo-field'
                    value={getApplication()}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                        setApplication(e.target.value);
                    }}
                />
            </div>
            <div className='columns small-1'>
                <i className='fa fa-times' onClick={deleteApp} style={{cursor: 'pointer'}} />
            </div>
        </div>
    );
}

const Attribute = ReactForm.FormField(AttributeWrapper);

function generateSchedule(minute?: string, hour?: string, dom?: string, month?: string, dow?: string): string {
    return `${minute} ${hour} ${dom} ${month} ${dow}`;
}

export const ProjectSyncWindowScheduleEdit = (props: ProjectSyncWindowProps) => (
    <>
        <p>Schedule</p>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-2'>Minute{helpTip('The minute/minutes assigned to the schedule')}</div>
                <div className='columns small-2'>Hour{helpTip('The hour/hours assigned to the schedule')}</div>
                <div className='columns small-2'>Day Of The Month{helpTip('The day/days of the month assigned to the schedule')}</div>
                <div className='columns small-2'>Month{helpTip('The month/months assigned to the schedule.')}</div>
                <div className='columns small-2'>Day Of the Week{helpTip('The day/days of the week assigned to the schedule')}</div>
            </div>
        </div>
        <div className='row project-sync-windows-panel__form-row'>
            <Schedule key='schedule' field={'window.schedule'} formApi={props.formApi} />
        </div>
    </>
);

interface ScheduleProps {
    fieldApi: ReactForm.FieldApi;
}

function generateRange(limit: number, zeroStart: boolean): string[] {
    const range: string[] = new Array(limit);
    for (let i = 0; i < limit; i++) {
        if (zeroStart) {
            range[i] = i.toString();
        } else {
            range[i] = (i + 1).toString();
        }
    }
    return range;
}

function getRanges(config: string): string[] {
    const values = [];
    const fields = config.split(',');
    for (const f of fields) {
        if (f.search(/-/) !== -1) {
            const r = f.split('-');
            for (let i = parseInt(r[0], 10); i <= parseInt(r[1], 10); i++) {
                values.push(i.toString());
            }
        } else {
            values.push(f);
        }
    }
    return values;
}

function setRanges(config: string[]): string {
    const values = [];
    const ranges = [];

    config.sort((n1, n2) => parseInt(n1, 10) - parseInt(n2, 10));

    for (let i = 0; i < config.length; i++) {
        if (ranges.length === 0) {
            ranges[0] = [config[i]];
        } else {
            if (parseInt(config[i], 10) - 1 === parseInt(config[i - 1], 10)) {
                ranges[ranges.length - 1].push(config[i]);
            } else {
                ranges[ranges.length] = [config[i]];
            }
        }
    }

    if (ranges.length > 0) {
        for (const r of ranges) {
            if (r.length > 1) {
                values.push(r[0] + '-' + r[r.length - 1]);
            } else {
                values.push(r[0]);
            }
        }
    }
    return values.join(',');
}

function ScheduleWrapper({fieldApi}: ScheduleProps) {
    const getValues = (f: number): string[] => {
        if (fieldApi.getValue() !== undefined) {
            const fields = (fieldApi.getValue() as string).split(' ');
            const subFields = getRanges(fields[f]);
            return subFields;
        }
        return ['*'];
    };

    const setValues = (values: string[], f: number) => {
        if (fieldApi.getValue() !== undefined) {
            const fields = (fieldApi.getValue() as string).split(' ');
            fields[f] = setRanges(values);
            fieldApi.setValue(fields.join(' '));
        } else {
            switch (f) {
                case 0:
                    fieldApi.setValue(generateSchedule(values.join(','), '*', '*', '*', '*'));
                    break;
                case 1:
                    fieldApi.setValue(generateSchedule('*', values.join(','), '*', '*', '*'));
                    break;
                case 2:
                    fieldApi.setValue(generateSchedule('*', '*', values.join(','), '*', '*'));
                    break;
                case 3:
                    fieldApi.setValue(generateSchedule('*', '*', '*', values.join(','), '*'));
                    break;
                case 4:
                    fieldApi.setValue(generateSchedule('*', '*', '*', '*', values.join(',')));
                    break;
            }
        }
    };

    return (
        <>
            <div className='columns small-2'>
                <select
                    className='argo-field project-sync-windows-panel__options-wrapper'
                    size={8}
                    name='minute'
                    multiple={true}
                    value={getValues(0)}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => {
                        const minuteOptions = e.target.options;
                        const minuteValues = [];
                        for (let i = 0, l = minuteOptions.length; i < l; i++) {
                            if (minuteOptions[i].selected) {
                                minuteValues.push(minuteOptions[i].value);
                            }
                        }
                        setValues(minuteValues, 0);
                    }}>
                    <option key='wildcard' value='*' className='project-sync-windows-panel__text-wrapper'>
                        Every Minute
                    </option>
                    {generateRange(60, true).map(m => (
                        <option key={m}>{m}</option>
                    ))}
                </select>
            </div>
            <div className='columns small-2'>
                <select
                    className='argo-field project-sync-windows-panel__options-wrapper'
                    size={8}
                    name='hours'
                    multiple={true}
                    value={getValues(1)}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => {
                        const hourOptions = e.target.options;
                        const hourValues = [];
                        for (let i = 0, l = hourOptions.length; i < l; i++) {
                            if (hourOptions[i].selected) {
                                hourValues.push(hourOptions[i].value);
                            }
                        }
                        setValues(hourValues, 1);
                    }}>
                    <option key='wildcard' value='*' className='project-sync-windows-panel__text-wrapper'>
                        Every Hour
                    </option>
                    {generateRange(24, true).map(m => (
                        <option key={m}>{m}</option>
                    ))}
                </select>
            </div>
            <div className='columns small-2'>
                <select
                    className='argo-field project-sync-windows-panel__options-wrapper'
                    size={8}
                    name='dom'
                    multiple={true}
                    value={getValues(2)}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => {
                        const domOptions = e.target.options;
                        const domValues = [];
                        for (let i = 0, l = domOptions.length; i < l; i++) {
                            if (domOptions[i].selected) {
                                domValues.push(domOptions[i].value);
                            }
                        }
                        setValues(domValues, 2);
                    }}>
                    <option key='wildcard' value='*' className='project-sync-windows-panel__text-wrapper'>
                        Every Day
                    </option>
                    {generateRange(31, false).map(m => (
                        <option key={m}>{m}</option>
                    ))}
                </select>
            </div>
            <div className='columns small-2'>
                <select
                    className='argo-field project-sync-windows-panel__options-wrapper'
                    size={8}
                    name='month'
                    multiple={true}
                    value={getValues(3)}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => {
                        const monthOptions = e.target.options;
                        const monthValues = [];
                        for (let i = 0, l = monthOptions.length; i < l; i++) {
                            if (monthOptions[i].selected) {
                                monthValues.push(monthOptions[i].value);
                            }
                        }
                        setValues(monthValues, 3);
                    }}>
                    <option key='wildcard' value='*' className='project-sync-windows-panel__text-wrapper'>
                        Every Month
                    </option>
                    <option key='1' value='1'>
                        Jan
                    </option>
                    <option key='2' value='2'>
                        Feb
                    </option>
                    <option key='3' value='3'>
                        Mar
                    </option>
                    <option key='4' value='4'>
                        Apr
                    </option>
                    <option key='5' value='5'>
                        May
                    </option>
                    <option key='6' value='6'>
                        Jun
                    </option>
                    <option key='7' value='7'>
                        Jul
                    </option>
                    <option key='8' value='8'>
                        Aug
                    </option>
                    <option key='9' value='9'>
                        Sep
                    </option>
                    <option key='10' value='10'>
                        Oct
                    </option>
                    <option key='11' value='11'>
                        Nov
                    </option>
                    <option key='12' value='12'>
                        Dec
                    </option>
                </select>
            </div>
            <div className='columns small-2'>
                <select
                    className='argo-field project-sync-windows-panel__options-wrapper'
                    size={8}
                    name='dow'
                    multiple={true}
                    value={getValues(4)}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => {
                        const dowOptions = e.target.options;
                        const dowValues = [];
                        for (let i = 0, l = dowOptions.length; i < l; i++) {
                            if (dowOptions[i].selected) {
                                dowValues.push(dowOptions[i].value);
                            }
                        }
                        setValues(dowValues, 4);
                    }}>
                    <option key='wildcard' value='*' className='project-sync-windows-panel__text-wrapper'>
                        Every Day
                    </option>
                    <option key='0' value='0'>
                        Sun
                    </option>
                    <option key='1' value='1'>
                        Mon
                    </option>
                    <option key='2' value='2'>
                        Tue
                    </option>
                    <option key='3' value='3'>
                        Wed
                    </option>
                    <option key='4' value='4'>
                        Thu
                    </option>
                    <option key='5' value='5'>
                        Fri
                    </option>
                    <option key='6' value='6'>
                        Sat
                    </option>
                </select>
            </div>
        </>
    );
}

const Schedule = ReactForm.FormField(ScheduleWrapper);
