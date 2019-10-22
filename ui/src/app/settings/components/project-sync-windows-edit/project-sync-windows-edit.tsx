import { Tooltip } from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';
import {RuleCondition, SyncWindow, WindowRule} from '../../../shared/models';

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

function generateSchedule(minute?: string, hour?: string, dom?: string, month?: string, dow?: string): string {
    return `${minute} ${hour} ${dom} ${month} ${dow}`;
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
            if ((parseInt(config[i], 10) - 1) === parseInt(config[i - 1], 10)) {
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

export const ProjectSyncWindowScheduleEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Schedule</h6>
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
            <Schedule key='schedule' field={'window.schedule'}
                      formApi={props.formApi}
            />
        </div>
    </React.Fragment>
);

interface ScheduleProps {
    fieldApi: ReactForm.FieldApi;
}

class ScheduleWrapper extends React.Component<ScheduleProps, any> {

    public render() {
        return (
            <React.Fragment>
                <div className='columns small-2'>
                    <select className='argo-field' size={8} name='minute' multiple={true}  value={this.getValues(0)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const minuteOptions = e.target.options;
                        const minuteValues = [];
                        for (let i = 0, l = minuteOptions.length; i < l; i++) {
                            if (minuteOptions[i].selected) {
                                minuteValues.push(minuteOptions[i].value);
                            }
                        }
                        this.setValues(minuteValues, 0);
                    }}>
                        <option key='wildcard' value='*'>Every Minute</option>
                        {generateRange(60, true).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' size={8} name='hours' multiple={true} value={this.getValues(1)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const hourOptions = e.target.options;
                        const hourValues = [];
                        for (let i = 0, l = hourOptions.length; i < l; i++) {
                            if (hourOptions[i].selected) {
                                hourValues.push(hourOptions[i].value);
                            }
                        }
                        this.setValues(hourValues, 1);
                    }}>
                        <option key='wildcard' value='*'>Every Hour</option>
                        {generateRange(24, true).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' size={8} name='dom' multiple={true} value={this.getValues(2)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const domOptions = e.target.options;
                        const domValues = [];
                        for (let i = 0, l = domOptions.length; i < l; i++) {
                            if (domOptions[i].selected) {
                                domValues.push(domOptions[i].value);
                            }
                        }
                        this.setValues(domValues, 2);
                    }}>
                        <option key='wildcard' value='*'>Every Day</option>
                        {generateRange(31, false).map((m) => (
                            <option key={m}>{m}</option>
                        ))}
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' size={8} name='month' multiple={true} value={this.getValues(3)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const monthOptions = e.target.options;
                        const monthValues = [];
                        for (let i = 0, l = monthOptions.length; i < l; i++) {
                            if (monthOptions[i].selected) {
                                monthValues.push(monthOptions[i].value);
                            }
                        }
                        this.setValues(monthValues, 3);
                    }}>
                        <option key='wildcard' value='*'>Every Month</option>
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
                    </select>
                </div>
                <div className='columns small-2'>
                    <select className='argo-field' size={8} name='dow' multiple={true} value={this.getValues(4)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const dowOptions = e.target.options;
                        const dowValues = [];
                        for (let i = 0, l = dowOptions.length; i < l; i++) {
                            if (dowOptions[i].selected) {
                                dowValues.push(dowOptions[i].value);
                            }
                        }
                        this.setValues(dowValues, 4);
                    }}>
                        <option key='wildcard' value='*'>Sunday-Saturday</option>
                        <option key='0' value='0'>Sun</option>
                        <option key='1' value='1'>Mon</option>
                        <option key='2' value='2'>Tue</option>
                        <option key='3' value='3'>Wed</option>
                        <option key='4' value='4'>Thu</option>
                        <option key='5' value='5'>Fri</option>
                        <option key='6' value='6'>Sat</option>
                    </select>
                </div>
            </React.Fragment>
        );
    }

    private getValues(f: number): string[] {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            const subFields = getRanges(fields[f]);
            return subFields;
        }
        return ['*'];
    }

    private setValues(values: string[], f: number) {
        if (this.props.fieldApi.getValue() !== undefined) {
            const fields = (this.props.fieldApi.getValue() as string).split(' ');
            fields[f] = setRanges(values);
            this.props.fieldApi.setValue(fields.join(' '));
        } else {
            switch (f) {
                case 0:
                    this.props.fieldApi.setValue(generateSchedule(values.join(','), '*', '*', '*', '*'));
                    break;
                case 1:
                    this.props.fieldApi.setValue(generateSchedule('*', values.join(','), '*', '*', '*'));
                    break;
                case 2:
                    this.props.fieldApi.setValue(generateSchedule('*', '*', values.join(','), '*', '*'));
                    break;
                case 3:
                    this.props.fieldApi.setValue(generateSchedule('*', '*', '*', values.join(','), '*'));
                    break;
                case 4:
                    this.props.fieldApi.setValue(generateSchedule('*', '*', '*', '*', values.join(',')));
                    break;
            }
        }
        return;
    }

}

const Schedule = ReactForm.FormField(ScheduleWrapper);

function newRule(): WindowRule {
    const r = {
        conditions: [],
    } as WindowRule;
    const c = {
        kind: 'application',
        operator: 'in',
        values: [],
    } as RuleCondition;
    r.conditions.push(c);

    return r;
}

export const ProjectSyncWindowRulesEdit = (props: ProjectSyncWindowProps) => (
    <React.Fragment>
        <h6>Rules</h6>
        <div className='argo-table-list argo-table-list--clickable'>
            <div className='argo-table-list__head'>
                <div className='row'>
                    <div className='columns small-1'>
                        ID
                    </div>
                    <div className='columns small-11'>
                        RULE
                    </div>
                </div>
            </div>
            {(props.window.rules || []).map((rule, i) => (
                <div className='argo-table-list__row' key={`${i}`} >
                    <div className='row'>
                        <div className='columns small-1' >{i}</div>
                        <div className='columns small-11'>
                            <Rule key={i} field={['window.rules', i]}
                                  formApi={props.formApi}
                            />
                        </div>
                    </div>
                </div>
            ))}
            <div className='columns small-3'>
                <a onClick={() => {
                    props.formApi.setValue('window.rules', (props.formApi.values.window.rules || []).concat(newRule()));
                }}>Add Rule</a>
            </div>
        </div>
    </React.Fragment>
);

interface RuleProps {
    projName: string;
    roleName: string;
    fieldApi: ReactForm.FieldApi;
}

class RuleWrapper extends React.Component<RuleProps, any> {

    public render() {
        return (
            <div>
                {(this.props.fieldApi.getValue().conditions.map((comp: RuleCondition, i: number) => (
                    <div className='row'>
                        <div className='columns small-2'>
                            <select className='argo-field' value={this.getKind(comp)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                                this.setKind(i, e.target.value);
                            }}>
                                <option key='application'>application</option>
                                <option key='namespace'>namespace</option>
                                <option key='cluster'>cluster</option>
                                <option key='label'>label</option>
                            </select>
                        </div>
                        {comp.kind === 'label' && (
                            <div className='columns small-2'>
                                <input className='argo-field' value={this.getKey(comp)} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                    this.setKey(i, e.target.value);
                                }}/>
                            </div>
                        )}
                        <div className='columns small-2'>
                            <select className='argo-field' value={this.getOperator(comp)} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                                this.setOperator(i, e.target.value);
                            }}>
                                <option key='in'>in</option>
                                <option key='notIn'>notIn</option>
                                <option key='exists'>exists</option>
                            </select>
                        </div>
                        {comp.operator !== 'exists' && (
                            <div className='columns small-5'>
                                <input className='argo-field' value={this.getValues(comp)} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                    this.setValues(i, e.target.value);
                                }}/>
                            </div>
                        )}
                        <div className='columns small-1'>
                            <i className='fa fa-times' onClick={() => this.deleteCondition(i)} style={{cursor: 'pointer'}}/>
                        </div>
                    </div>
                )))}
                <div className='columns small-3'>
                    <a onClick={() => {
                        this.newCondition();
                    }}>Add Condition</a>
                </div>
            </div>
        );
    }

    private getKey(c: RuleCondition): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            if (c.key !== undefined) {
                return c.key;
            }
        }
        return '';
    }

    private setKey(i: number, v: string): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            rule.conditions[i].key = v;
            this.props.fieldApi.setValue(rule);
        }
        return;
    }

    private getKind(c: RuleCondition): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            if (c.kind !== undefined) {
                return c.kind;
            }
        }
        return '';
    }

    private setKind(i: number, v: string): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            rule.conditions[i].kind = v;
            if (v !== 'label') {
                rule.conditions[i].key = undefined;
            }
            this.props.fieldApi.setValue(rule);
        }
        return;
    }

    private getOperator(c: RuleCondition): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            if (c.operator !== undefined) {
                return c.operator;
            }
        }
        return '';
    }

    private setOperator(i: number, v: string): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            rule.conditions[i].operator = v;
            if (v === 'exists') {
                rule.conditions[i].values = undefined;
            }
            this.props.fieldApi.setValue(rule);
        }
        return;
    }

    private getValues(c: RuleCondition): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            if (c.values !== undefined) {
                return c.values.join(',');
            }
        }
        return '';
    }

    private setValues(i: number, v: string): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            rule.conditions[i].values = v.split(',');
            this.props.fieldApi.setValue(rule);
        }
        return;
    }

    private newCondition() {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            const c = {
                kind: 'application',
                operator: 'in',
                values: [],
            } as RuleCondition;
            rule.conditions.push(c);
            this.props.fieldApi.setValue(rule);
        }
        return;
    }

    private deleteCondition(i: number): string {
        if (this.props.fieldApi.getValue() !== undefined) {
            const rule = this.props.fieldApi.getValue();
            rule.conditions.splice(i, 1);
            if (rule.conditions.length === 0) {
                rule.conditions = [];
            }
            this.props.fieldApi.setValue(rule);
        }
        return;
    }
}

const Rule = ReactForm.FormField(RuleWrapper);
