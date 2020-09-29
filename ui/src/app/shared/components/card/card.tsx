import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {GetProp} from '../../../settings/components/utils';
import {FieldData} from './field';
import {CardRow, FieldLabels} from './row';

require('./card.scss');

interface CardProps<T> {
    title: string;
    values: T[];
    fields: FieldData[];
    save: (values: T[]) => Promise<any>;
    docs?: string;
    help?: string;
    fullWidth?: boolean;
    disabled?: boolean;
    emptyItem?: T;
}

interface CardState<T> {
    selected: boolean[];
    data: T[];
    rows: Row<T>[];
    edit: boolean;
}

export interface Row<T> {
    id: number;
    value: T;
}

export function HelpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}>
                {' '}
                <i className='fa fa-question-circle' />
            </span>
        </Tooltip>
    );
}

export class Card<T> extends React.Component<CardProps<T>, CardState<T>> {
    constructor(props: CardProps<T>) {
        super(props);
        let selected: boolean[] = [];
        const rows: Row<T>[] = [];
        if (props.values) {
            selected = new Array(props.values.length);
            for (const r of props.values) {
                const id = Math.random();
                rows.push({id, value: r});
            }
        }
        this.state = {selected, data: props.values, rows, edit: false};
    }
    get selectedIdxs(): number[] {
        const arr: number[] = [];
        this.state.selected.forEach((s, idx) => {
            if (s) {
                arr.push(idx);
            }
        });
        return arr;
    }
    public render() {
        return (
            <div className='white-box'>
                <div className='white-box__details'>
                    <div className='row'>
                        <p>
                            {this.props.title.toUpperCase()}&nbsp;
                            {this.props.help && HelpTip(this.props.help)}&nbsp;
                            {this.props.docs && (
                                <a href={this.props.docs} target='_blank'>
                                    <i className='fa fa-file-alt' />
                                </a>
                            )}
                        </p>
                        <div className='row'>
                            <div className='editable-panel__buttons'>
                                {!this.state.edit && (
                                    <button
                                        onClick={() => {
                                            this.setState({edit: true});
                                        }}
                                        className='argo-button argo-button--base'>
                                        Edit
                                    </button>
                                )}
                                {this.state.edit && (
                                    <React.Fragment>
                                        <button
                                            className='argo-button argo-button--base'
                                            onClick={e => {
                                                this.props.save(this.state.data);
                                                this.setState({edit: false});
                                            }}>
                                            Save
                                        </button>
                                        <button onClick={() => this.setState({edit: false})} className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button>
                                    </React.Fragment>
                                )}
                            </div>
                        </div>
                    </div>
                    {this.state.data && this.state.data.length > 0 ? (
                        <React.Fragment>
                            {FieldLabels(this.props.fields, this.state.edit)}
                            {this.state.edit ? (
                                <React.Fragment>
                                    {(this.state.rows || []).map((row, i) => {
                                        return (
                                            <div key={i}>
                                                <CardRow<T>
                                                    fields={this.props.fields}
                                                    data={row.value}
                                                    remove={() => this.remove([i])}
                                                    selected={this.state.selected[i]}
                                                    toggleSelect={() => this.toggleSelect(i)}
                                                    onChange={r => {
                                                        const rows = this.state.rows;
                                                        rows[i] = {...rows[i], value: r as T};
                                                        this.syncState(rows);
                                                    }}
                                                />
                                            </div>
                                        );
                                    })}
                                </React.Fragment>
                            ) : (
                                <React.Fragment>
                                    {(this.state.data || []).map((d: T, idx) => (
                                        <div className='card__row' key={idx}>
                                            {this.props.fields.map((field, i) => {
                                                return (
                                                    <div key={field.name} className={`card__col-input card__col card__col-${field.size}`}>
                                                        {typeof d === 'string' ? d.toString() : (GetProp(d as T, field.name as keyof T) || '').toString()}
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    ))}
                                </React.Fragment>
                            )}
                        </React.Fragment>
                    ) : (
                        <div className='card__row'>Project has no {this.props.title.toLowerCase()}</div>
                    )}
                    {this.state.edit && (
                        <div className='card__row'>
                            {this.selectedIdxs.length > 1 ? (
                                <button className='argo-button argo-button--base-o' onClick={() => this.remove(this.selectedIdxs)}>
                                    DELETE SELECTED
                                </button>
                            ) : null}
                            {this.props.disabled ? null : (
                                <button
                                    className='argo-button argo-button--base argo-button--short'
                                    onClick={_ => this.syncState(this.state.rows.concat([{value: this.props.emptyItem, id: Math.random()}]))}>
                                    <i className='fa fa-plus' style={{cursor: 'pointer'}} />
                                </button>
                            )}
                        </div>
                    )}
                </div>
            </div>
        );
    }

    private toggleSelect(i: number) {
        const selected = this.state.selected;
        selected[i] = !selected[i];
        this.setState({selected});
    }
    private raw(data: Row<T>[]): T[] {
        return data && data.length > 0 ? data.map(d => d.value) : [];
    }
    private remove(idxs: number[]) {
        const selected = this.state.selected;
        const rows = this.state.rows;
        while (idxs.length) {
            const i = idxs.pop();
            selected.splice(i, 1);
            rows.splice(i, 1);
        }
        this.setState({selected});
        this.syncState(rows);
    }
    private syncState(rows: Row<T>[]) {
        this.setState({rows});
        this.setState({data: this.raw(rows)});
    }
}
