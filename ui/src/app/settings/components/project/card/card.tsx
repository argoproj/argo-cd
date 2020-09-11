import * as React from 'react';
import {FieldData, FieldTypes, FieldValue} from './field';
import {CardRow} from './row';

require('../project.scss');
require('./card.scss');

interface CardProps<T> {
    title: string;
    data: T[];
    fields: FieldData[];
    add: () => Promise<any>;
    remove: (i: number[]) => void;
    save: (i: number[], values: FieldValue[] | T[]) => Promise<any>;
    docs: string;
    fullWidth: boolean;
}

interface CardState<T> {
    selected: boolean[];
    isChanged: boolean[];
    data: Row<T>[];
    changeCount: number;
}

interface Row<T> {
    id: number;
    value: T;
}

export class Card<T> extends React.Component<CardProps<T>, CardState<T>> {
    constructor(props: CardProps<T>) {
        super(props);
        let selected: boolean[] = [];
        let isChanged: boolean[] = [];
        const data: Row<T>[] = [];
        if (props.data) {
            selected = new Array(props.data.length);
            isChanged = new Array(props.data.length);
            for (const r of props.data) {
                const id = Math.random();
                data.push({id, value: r});
            }
        }
        this.add = this.add.bind(this);
        this.state = {selected, isChanged, data, changeCount: 0};
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
    get changedVals(): [number[], T[]] {
        const idxs: number[] = [];
        const vals: T[] = [];
        this.state.isChanged.forEach((s, idx) => {
            if (s) {
                idxs.push(idx);
                vals.push(this.state.data[idx].value);
            }
        });
        return [idxs, vals];
    }
    public render() {
        return (
            <div className={`card white-box ${this.props.data && this.props.data.length > 0 ? '' : 'card__empty'} ${this.props.fullWidth ? 'card__full-width' : ''}`}>
                <div className='white-box__details'>
                    <div className='card__row'>
                        <div className='card__title'>
                            {this.props.title}&nbsp;
                            {this.props.docs ? (
                                <a href={this.props.docs} target='_blank'>
                                    <i className='fas fa-question-circle' />
                                </a>
                            ) : null}
                        </div>
                        <div className='card__actions'>
                            {this.state.changeCount > 0 ? (
                                <button
                                    className={'project__button project__button-save'}
                                    onClick={() => {
                                        const [i, v] = this.changedVals;
                                        this.save(i, v);
                                    }}>
                                    SAVE ALL
                                </button>
                            ) : null}
                            {this.selectedIdxs.length > 1 ? (
                                <button className={'project__button project__button-error'} onClick={() => this.remove(this.selectedIdxs)}>
                                    DELETE SELECTED
                                </button>
                            ) : null}
                            <button className='project__button project__button-add project__button-round' onClick={this.add}>
                                <i className='fa fa-plus' />
                            </button>
                        </div>
                    </div>
                    {this.props.data && this.props.data.length > 0 ? (
                        <div>
                            <div className='card__row card__input-labels card__input-container'>
                                <div className='card__col-round-button card__col' />
                                {this.props.fields.map(field => {
                                    return (
                                        <div className={`card__input-labels--label card__col-input card__col card__col-${field.size}`} key={field.name + 'label'}>
                                            {field.name}
                                            {field.type === FieldTypes.ResourceKindSelector ? (
                                                <a href='https://kubernetes.io/docs/reference/kubectl/overview/#resource-types' target='_blank' className='card__info-icon'>
                                                    <i className='fas fa-info-circle' />
                                                </a>
                                            ) : null}
                                        </div>
                                    );
                                })}
                                <div className='card__col-button card__col' />
                            </div>
                            {this.state.data.map((row, i) => {
                                const val = row.value;
                                return (
                                    <div key={i}>
                                        <CardRow<T>
                                            fields={this.props.fields}
                                            data={val}
                                            remove={() => this.remove([i])}
                                            save={value => this.save([i], [value as T])}
                                            selected={this.state.selected[i]}
                                            toggleSelect={() => this.toggleSelect(i)}
                                            onChange={r => this.updateRow(i, r as T)}
                                            changed={this.state.isChanged[i]}
                                        />
                                    </div>
                                );
                            })}
                        </div>
                    ) : (
                        this.empty()
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
    private remove(idxs: number[]) {
        const tmp = [...idxs];
        const selected = this.state.selected;
        const data = [...this.state.data];
        while (tmp.length) {
            const i = tmp.pop();
            selected.splice(i, 1);
            data.splice(i, 1);
        }
        this.setState({data, selected});
        this.props.remove(idxs);
    }
    private empty() {
        return (
            <div className={'card__row'}>
                <div className={`card__col card__col-fill-${this.props.fields.length}`}>Project has no {this.props.title}</div>
            </div>
        );
    }
    private async add() {
        const data = [...this.state.data];
        const value = await this.props.add();
        data.push({value, id: Math.random()});
        this.setState({data});
    }
    private async save(idxs: number[], values: T[]) {
        await this.props.save(idxs, values);
        const isChanged = this.state.isChanged;
        const update = this.state.data;
        values.forEach((value, i) => {
            update[idxs[i]] = {value, id: Math.random()};
            isChanged[idxs[i]] = false;
        });
        this.setState({isChanged, data: update, changeCount: 0});
    }
    private updateRow(i: number, r: T) {
        const data = [...this.state.data];
        const isChanged = this.state.isChanged;
        const cur = {...data[i], value: r};
        data[i] = cur;
        if (data[i].value !== this.props.data[i]) {
            if (!isChanged[i]) {
                this.setState({changeCount: this.state.changeCount + 1});
            }
            isChanged[i] = true;
        } else {
            if (isChanged[i]) {
                this.setState({changeCount: this.state.changeCount - 1});
            }
            isChanged[i] = false;
        }
        this.setState({data});
    }
}
