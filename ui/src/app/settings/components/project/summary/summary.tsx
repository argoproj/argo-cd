import * as React from 'react';

import {Card, FieldType} from '../card';
require('./summary.scss');

interface SummaryProps {
    name: string;
}

export class ProjectSummary extends React.Component<SummaryProps> {
    public render() {
        return (
            <div className='project-summary'>
                <div>
                    <div className='project-summary__label'>PROJECT</div>
                    <div className='project-summary__title'>{this.props.name}</div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>DEPLOYMENT</div>
                    <div className='project-summary__section--row'>
                        <Card title='Sources' fields={[{type: FieldType.Text, name: 'hello'}, {type: FieldType.ResourceKindSelector, name: 'kind'}]} />
                        <Card title='Destinations' fields={[{type: FieldType.Text, name: 'foo'}]} />
                    </div>
                </div>
            </div>
        );
    }
}
