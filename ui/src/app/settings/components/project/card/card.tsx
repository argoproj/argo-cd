import {Tooltip} from 'argo-ui';
import * as React from 'react';
import {MultiData} from '../../../../shared/components/multi-data';
import {MultiInput} from '../../../../shared/components/multi-input';
import {FieldData} from './field';

interface CardProps<T> {
    title: string;
    values: T[];
    fields: FieldData[];
    save: (values: T[]) => Promise<any>;
    docs?: string;
    help?: string;
    fullWidth?: boolean;
    disabled?: boolean;
}

interface CardState<T> {
    selected: boolean[];
    isChanged: boolean[];
    changeCount: number;
    data: T[];
    edit: boolean;
}

function helpTip(text: string) {
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
        let isChanged: boolean[] = [];
        if (props.values) {
            selected = new Array(props.values.length);
            isChanged = new Array(props.values.length);
        }
        this.state = {selected, isChanged, data: props.values, changeCount: 0, edit: false};
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
                            {this.props.help && helpTip(this.props.help)}&nbsp;
                            {this.props.docs ? (
                                <a href={this.props.docs} target='_blank'>
                                    <i className='fas fa-question-circle' />
                                </a>
                            ) : null}
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
                                        <button className='argo-button argo-button--base' onClick={e => this.props.save(this.state.data)}>
                                            Save
                                        </button>
                                        <button
                                            onClick={() => {
                                                this.setState({edit: false});
                                            }}
                                            className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button>
                                    </React.Fragment>
                                )}
                            </div>
                        </div>
                    </div>
                    {this.state.edit ? (
                        <MultiInput<T> title={this.props.title} data={this.state.data} fields={this.props.fields} onChange={async data => await this.setState({data})} />
                    ) : (
                        MultiData(this.props.fields, this.props.values, this.props.title)
                    )}
                </div>
            </div>
        );
    }
}
