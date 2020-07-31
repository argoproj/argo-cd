import * as React from 'react';
import {CardRow, FieldData} from './row';

require('../project.scss');
require('./card.scss');

interface CardProps<T> {
    title: string;
    data: T[];
    fields: FieldData[];
}

// Field type describes structure of the card but not the state
// Row type will store the state

// this.state.rows[x][field].(type | value)

// PROBLEM: state data must match form structure
// so we need the structure of the data to define the structure of the form

// data must have knowledge of form structure to ensure type safety
// but there is a lot of redundancy and extra processing to store form structure in each piece of data

export class Card<T> extends React.Component<CardProps<T>> {
    public render() {
        return (
            <div className='card'>
                <div className='card__row'>
                    <div className='card__title'>{this.props.title}</div>
                    <button className='project__button project__button-add project__button-round'>+</button>
                </div>
                <div className='card__row card__input-labels'>
                    <div className='card__col-round-button card__col' />
                    {this.props.fields.map(field => {
                        return (
                            <div className='card__input-labels--label card__col-input card__col' key={field.name + 'label'}>
                                {field.name}
                            </div>
                        );
                    })}
                    <div className='card__col-button card__col' />
                </div>
                <div className='card__input-container card__row'>
                    <div className='card__col-round-button card__col'>
                        <button className='project__button project__button-remove project__button-round'>-</button>
                    </div>
                    {this.props.data.map(row => {
                        return <CardRow<T> key={row.toString()} fields={this.props.fields} data={row} />
                    })}
                    <div className='card__col-button card__col'>
                        <button className='project__button project__button-save'>SAVE</button>
                    </div>
                </div>
            </div>
        );
    }
}
