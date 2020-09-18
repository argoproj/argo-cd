import * as React from 'react';
import {FieldData, FieldLabels, FieldValue} from '../../../settings/components/project/card/field';
import {CardRow} from '../../../settings/components/project/card/row';

require('../../../settings/components/project/project.scss');
require('../../../settings/components/project/card/card.scss');

interface MultiInputProps<T> {
    title: string;
    data: T[];
    empty: T;
    fields: FieldData[];
    save: (i: number[], values: FieldValue[] | T[]) => Promise<any>;
    docs?: string;
    disabled?: boolean;
}

interface MultiInputState<T> {
    selected: boolean[];
    isChanged: boolean[];
    data: Row<T>[];
    changeCount: number;
}

interface Row<T> {
    id: number;
    value: T;
}

export class MultiInput<T> extends React.Component<MultiInputProps<T>, MultiInputState<T>> {
    constructor(props: MultiInputProps<T>) {
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
            <div className='multi-input card__multi-data'>
                <div>
                    <div className='card__row'>
                        <div className='card__title'>
                            {this.props.docs ? (
                                <a href={this.props.docs} target='_blank'>
                                    <i className='fas fa-question-circle' />
                                </a>
                            ) : null}
                        </div>
                    </div>
                    {FieldLabels(this.props.fields)}
                    {this.props.data && this.props.data.length > 0 ? (
                        <div>
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
                    <div className='card__actions'>
                        {this.state.changeCount > 0 ? (
                            <button
                                className={'card__button card__button-save'}
                                onClick={() => {
                                    const [i, v] = this.changedVals;
                                    this.save(i, v);
                                }}>
                                SAVE ALL
                            </button>
                        ) : null}
                        {this.selectedIdxs.length > 1 ? (
                            <button className={'card__button card__button-error'} onClick={() => this.remove(this.selectedIdxs)}>
                                DELETE SELECTED
                            </button>
                        ) : null}
                        {this.props.disabled ? null : (
                            <button className='card__button card__button-add card__button-round' onClick={_ => this.add(this.props.empty)}>
                                <i className='fa fa-plus' />
                            </button>
                        )}
                    </div>
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
    }
    private empty() {
        return (
            <div className={'card__row'}>
                <div className={`card__col card__col-fill-${this.props.fields.length}`}>Project has no {this.props.title}</div>
            </div>
        );
    }
    private async add(empty: T) {
        const data = [...this.state.data];
        data.push({value: empty, id: Math.random()});
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
