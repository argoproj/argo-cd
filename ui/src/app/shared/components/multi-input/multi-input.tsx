import * as React from 'react';
import {FieldData, FieldValue, IsFieldValue} from '../../../settings/components/project/card/field';
import {CardRow, FieldLabels} from '../../../settings/components/project/card/row';

interface MultiInputProps<T> {
    title: string;
    data: T[];
    fields: FieldData[];
    disabled?: boolean;
    onChange?: (values: T[]) => Promise<any>;
}

interface MultiInputState<T> {
    selected: boolean[];
    isChanged: boolean[];
    data: Row<T>[];
    changeCount: number;
}

export interface Row<T> {
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
    get emptyItem(): T | FieldValue {
        if (IsFieldValue(this.raw(this.state.data)[0])) {
            return '';
        } else {
            return {} as T;
        }
    }
    public render() {
        return (
            <React.Fragment>
                <div>
                    <div>{FieldLabels(this.props.fields, true)}</div>
                    {this.props.data && this.props.data.length > 0 ? (
                        <div>
                            {this.state.data.map((row, i) => {
                                return (
                                    <div key={i}>
                                        <CardRow<T>
                                            fields={this.props.fields}
                                            data={row.value}
                                            remove={() => this.remove([i])}
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
                        <div className='card__row'>Project has no {this.props.title}</div>
                    )}
                    <div className='card__row'>
                        {this.selectedIdxs.length > 1 ? (
                            <button className='argo-button argo-button--base-o' onClick={() => this.remove(this.selectedIdxs)}>
                                DELETE SELECTED
                            </button>
                        ) : null}
                        {this.props.disabled ? null : (
                            <button className='card__button card__button-add card__button-round' onClick={_ => this.add()}>
                                <i className='fa fa-plus' />
                            </button>
                        )}
                    </div>
                </div>
            </React.Fragment>
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
        const tmp = [...idxs];
        const selected = this.state.selected;
        const data = [...this.state.data];
        while (tmp.length) {
            const i = tmp.pop();
            selected.splice(i, 1);
            data.splice(i, 1);
        }
        this.setState({data, selected});
        this.props.onChange(this.raw(data));
    }
    private async add() {
        const data = [...this.state.data];
        data.push({value: this.emptyItem as T, id: Math.random()});
        this.setState({data});
        this.props.onChange(this.raw(data));
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
        if (this.props.onChange) {
            this.props.onChange(this.raw(data));
        }
    }
}
