import * as React from 'react';
import {ResourceKindSelector} from './resource-kind-selector';

require('./project.scss');
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
                    {this.props.fields.map(field => {
                        let format;
                        switch (field.type) {
                            case FieldType.ResourceKindSelector:
                                format = <ResourceKindSelector />;
                                break;
                            default:
                                format = <input type='text' placeholder={field.name} />;
                        }
                        return (
                            <div key={field.type + '.' + field.name} className='card__col-input card__col'>
                                {format}
                            </div>
                        );
                    })}
                    <div className='card__col-button card__col'>
                        <button className='project__button project__button-save'>SAVE</button>
                    </div>
                </div>
            </div>
        );
    }
}
