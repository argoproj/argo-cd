import * as React from 'react';
import {Project} from '../../../../shared/models';
import {CardRow, FieldData, FieldValue} from './row';

require('../project.scss');
require('./card.scss');

interface CardProps<T> {
    title: string;
    data: T[];
    fields: FieldData[];
    add: () => void;
    remove: (i: number) => void;
    save: (i: number, value: T | FieldValue) => Promise<Project>;
}

export class Card<T> extends React.Component<CardProps<T>> {
    public render() {
        return (
            <div className='card'>
                <div className='card__row'>
                    <div className='card__title'>{this.props.title}</div>
                    <button className='project__button project__button-add project__button-round' onClick={this.props.add}>
                        <i className='fa fa-plus' />
                    </button>
                </div>
                {this.props.data && this.props.data.length > 0 ? (
                    <div>
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
                        {this.props.data.map((row, i) => {
                            return (
                                <div key={row.toString() + '.' + i}>
                                    <CardRow<T> fields={this.props.fields} data={row} remove={() => this.props.remove(i)} save={value => this.props.save(i, value)} />
                                </div>
                            );
                        })}
                    </div>
                ) : (
                    this.empty()
                )}
            </div>
        );
    }
    private empty() {
        return <div className={`card__col card__col-fill-${this.props.fields.length}`}>This is empty!</div>;
    }
}
