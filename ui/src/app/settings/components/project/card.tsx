import * as React from 'react';
import {ResourceKindSelector} from './resource-kind-selector';

require('./card.scss');

interface CardProps {
    title: string;
    fields: Field[];
}

interface Field {
    type: FieldType;
    name: string;
}

export enum FieldType {
    Text = 'text',
    ResourceKindSelector = 'resourceKindSelector'
}

export class Card extends React.Component<CardProps> {
    public render() {
        return (
            <div className='card'>
                <div className='card__row'>
                    <div className='card__title'>{this.props.title}</div>
                    <button className='card__button card__button-add card__button-round'>+</button>
                </div>
                <div className='card__row card__input-labels'>
                    {this.props.fields.map(field => {
                        return <div className='card__input-labels--label'>{field.name}</div>;
                    })}
                </div>
                <div className='card__input-container card__row'>
                    <button className='card__button card__button-remove card__button-round'>-</button>
                    {this.props.fields.map(field => {
                        switch (field.type) {
                            case FieldType.ResourceKindSelector:
                                return <ResourceKindSelector />;
                            default:
                                return <input key={field.type + '.' + field.name} type='text' placeholder={field.name} />;
                        }
                    })}
                    <button className='card__button card__button-save'>SAVE</button>
                </div>
            </div>
        );
    }
}
