import { Tooltip } from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';
import {SyncWindow} from '../../../shared/models';

interface ProjectSyncWindowProps {
    projName: string;
    window: SyncWindow;
    formApi: ReactForm.FormApi;
}

function helpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}> <i className='fa fa-question-circle'/></span>
        </Tooltip>
    );
}

export const ProjectSyncWindowApplicationsEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Applications</h6>
        <div>Manage applications assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.applications || []).map((a, i) => (
                <Attribute key={i} field={['window.applications', i]}
                        formApi={props.formApi}
                        projName={props.projName}
                        deleteApp={() => props.formApi.setValue('window.applications', removeEl(props.window.applications, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newA = '';
                        props.formApi.setValue('window.applications', (props.formApi.values.window.applications || []).concat(newA));
                    }}>Add Application</a>
                </div>
            </div>
        </div>
    </React.Fragment>
);

export const ProjectSyncWindowNamespaceEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Namespaces</h6>
        <div>Manage namespaces assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.namespaces || []).map((n, i) => (
                <Attribute key={i} field={['window.namespaces', i]}
                           formApi={props.formApi}
                           projName={props.projName}
                           deleteApp={() => props.formApi.setValue('window.namespaces', removeEl(props.window.namespaces, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newN = '';
                        props.formApi.setValue('window.namespaces', (props.formApi.values.window.namespaces || []).concat(newN));
                    }}>Add Namespace</a>
                </div>
            </div>
        </div>
    </React.Fragment>
);

export const ProjectSyncWindowClusterEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Clusters</h6>
        <div>Manage clusters assigned to this window ("*" for any)</div>
        <div className='argo-table-list__row'>
            {(props.window.clusters || []).map((c, i) => (
                <Attribute key={i} field={['window.clusters', i]}
                           formApi={props.formApi}
                           projName={props.projName}
                           deleteApp={() => props.formApi.setValue('window.clusters', removeEl(props.window.clusters, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newC = '';
                        props.formApi.setValue('window.clusters', (props.formApi.values.window.clusters || []).concat(newC));
                    }}>Add Cluster</a>
                </div>
            </div>
        </div>
    </React.Fragment>
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

class AttributeWrapper extends React.Component<AttributeProps, any> {

    public render() {
        return (
                <div className='row'>
                    <div className='columns small-6'>
                        <input className='argo-field' value={this.getApplication()} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                            this.setApplication(e.target.value);
                        }}/>
                    </div>
                    <div className='columns small-1'>
                        <i className='fa fa-times' onClick={() => this.props.deleteApp()} style={{cursor: 'pointer'}}/>
                    </div>
                </div>
        );
    }

    private getApplication(): string {
        return this.props.fieldApi.getValue();
    }

    private setApplication(application: string) {
        this.props.fieldApi.setValue(application);
    }
}

const Attribute = ReactForm.FormField(AttributeWrapper);

function generateSchedule(minute?: string, hour?: string, dom?: string, month?: string, dow?: string): string {
    return `${minute} ${hour} ${dom} ${month} ${dow}`;
}

export const ProjectSyncWindowScheduleEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Schedule</h6>
        <div className='argo-table-list__head'>
            <div className='row'>
                <div className='columns small-2'>Min{helpTip('The minute that the schedule will start on')}</div>
                <div className='columns small-2'>Hour{helpTip('The hour that the schedule will start on')}</div>
                <div className='columns small-2'>DOM{helpTip('The day of the month that the schedule will start on')}</div>
                <div className='columns small-2'>Mon{helpTip('The month that the schedule will start on')}</div>
                <div className='columns small-2'>DOW{helpTip('The day of the week that the schedule will start on')}</div>
            </div>
        </div>
        <div className='row project-sync-windows-panel__form-row'>
            <Schedule key='schedule' field={'window.schedule'}
                    formApi={props.formApi}
            />
        </div>
    </React.Fragment>
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

class ScheduleWrapper extends React.Component<ScheduleProps, any> {

    public render() {
        return (
            <React.Fragment>
                <div className='columns small-2'>
                    <select className='argo-field' name='month' value={this.getMinute()} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                            this.setMinute(e.target.value);
                    }}>
                        {generateRange(60, true).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                        <option key='wildcard' value='*'>Every</option>
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' name='hours' value={this.getHour()} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        this.setHour(e.target.value);
                    }}>
                        {generateRange(24, true).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                        <option key='wildcard' value='*'>Every</option>
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' name='dom' value={this.getDOM()} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        this.setDOM(e.target.value);
                    }}>
                        {generateRange(31, false).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                        <option key='wildcard' value='*'>Every</option>
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' name='month' value={this.getMonth()} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        this.setMonth(e.target.value);
                    }}>
                        <option key='1' value='1'>Jan</option>
                        <option key='2' value='2'>Feb</option>
                        <option key='3' value='3'>Mar</option>
                        <option key='4' value='4'>Apr</option>
                        <option key='5' value='5'>May</option>
                        <option key='6' value='6'>Jun</option>
                        <option key='7' value='7'>Jul</option>
                        <option key='8' value='8'>Aug</option>
                        <option key='9' value='9'>Sep</option>
                        <option key='10' value='10'>Oct</option>
                        <option key='11' value='11'>Nov</option>
                        <option key='12' value='12'>Dec</option>
                        <option key='wildcard' value='*'>Every</option>
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' name='dow' value={this.getDOW()} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        this.setDOW(e.target.value);
                    }}>
                       <option key='0' value='0'>Sun</option>
                        <option key='1' value='1'>Mon</option>
                        <option key='2' value='2'>Tue</option>
                        <option key='3' value='3'>Wed</option>
                        <option key='4' value='4'>Thu</option>
                        <option key='5' value='5'>Fri</option>
                        <option key='6' value='6'>Sat</option>
                        <option key='wildcard' value='*'>Every</option>
                    </select>
                </div>
            </React.Fragment>
        );
    }

    private getMinute(): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            return fields[0];
        }
        return '*';
    }

    private setMinute(minute: string) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[0] = `${minute}`;
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            this.props.fieldApi.setValue(generateSchedule(minute, '*', '*', '*', '*'));
        }
        return;
    }

    private getHour(): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            return fields[1];
        }
        return '*';
    }

    private setHour(hour: string) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[1] = `${hour}`;
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            this.props.fieldApi.setValue(generateSchedule('*', hour, '*', '*', '*'));
        }
        return;
    }

    private getDOM(): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            return fields[2];
        }
        return '*';
    }

    private setDOM(dom: string) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[2] = `${dom}`;
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            this.props.fieldApi.setValue(generateSchedule('*', '*', dom, '*', '*'));
        }
        return;
    }

    private getMonth(): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            return fields[3];
        }
        return '*';
    }

    private setMonth(month: string) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[3] = `${month}`;
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            this.props.fieldApi.setValue(generateSchedule('*', '*', '*', month, '*'));
        }
        return;
    }

    private getDOW(): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            return fields[4];
        }
        return '*';
    }

    private setDOW(dow: string) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[4] = `${dow}`;
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            this.props.fieldApi.setValue(generateSchedule('*', '*', '*', '*', dow));
        }
        return;
    }

}

const Schedule = ReactForm.FormField(ScheduleWrapper);
