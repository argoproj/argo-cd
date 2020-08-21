import * as React from 'react';
import {Project} from '../../../../shared/models';
import {CardRow, FieldData, FieldTypes, FieldValue} from './row';

require('../project.scss');
require('./card.scss');

interface CardProps<T> {
    title: string;
    data: T[];
    fields: FieldData[];
    add: () => void;
    remove: (i: number[]) => void;
    save: (i: number, value: T | FieldValue) => Promise<Project>;
    docs: string;
}

interface CardState {
    selected: boolean[];
}

export class Card<T> extends React.Component<CardProps<T>, CardState> {
    constructor(props: CardProps<T>) {
        super(props);
        const selected: boolean[] = [];
        this.state = {selected};
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
            <div className={`card white-box ${this.props.data && this.props.data.length > 0 ? null : 'card__empty'}`}>
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
                            {this.selectedIdxs.length > 1 ? (
                                <button className={'project__button project__button-error'} onClick={() => this.remove(this.selectedIdxs)}>
                                    DELETE SELECTED
                                </button>
                            ) : null}
                            <button className='project__button project__button-add project__button-round' onClick={this.props.add}>
                                <i className='fa fa-plus' />
                            </button>
                        </div>
                    </div>
                    {this.props.data && this.props.data.length > 0 ? (
                        <div>
                            <div className='card__row card__input-labels card__input-container'>
                                <div className='card__col-round-button card__col' />
                                <div className='card__input-labels--label'>
                                    {this.props.fields.map(field => {
                                        return (
                                            <div className='card__col-input card__col' key={field.name + 'label'}>
                                                {field.name}
                                                {field.type === FieldTypes.ResourceKindSelector ? (
                                                    <a href='https://kubernetes.io/docs/reference/kubectl/overview/#resource-types' target='_blank' className='card__info-icon'>
                                                        <i className='fas fa-info-circle' />
                                                    </a>
                                                ) : null}
                                            </div>
                                        );
                                    })}
                                </div>
                                <div className='card__col-button card__col' />
                            </div>
                            {this.props.data.map((row, i) => {
                                return (
                                    <div key={row.toString() + '.' + i}>
                                        <CardRow<T>
                                            fields={this.props.fields}
                                            data={row}
                                            remove={() => this.remove([i])}
                                            save={value => this.props.save(i, value)}
                                            selected={this.state.selected[i]}
                                            toggleSelect={() => this.toggleSelect(i)}
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
        while (tmp.length) {
            selected.splice(tmp.pop(), 1);
        }
        this.setState({selected});
        this.props.remove(idxs);
    }
    private empty() {
        return (
            <div className={'card__row'}>
                <div className={`card__col card__col-fill-${this.props.fields.length}`}>Project has no {this.props.title}</div>
            </div>
        );
    }
}
