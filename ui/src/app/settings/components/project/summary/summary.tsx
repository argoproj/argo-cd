import * as React from 'react';

import {services} from '../../../../shared/services';
import {Card, FieldType} from '../card';
require('./summary.scss');

interface SummaryProps {
    name: string;
    description: string;
}

interface SummaryState {
    name: string;
    description: string;
}

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    constructor(props: SummaryProps) {
        super(props);
        this.state = {name: props.name, description: props.description};
    }

    public render() {
        return (
            <div className='project-summary'>
                <div>
                    <div className='project-summary__label'>PROJECT</div>
                    <div className='project-summary__title'>{this.props.name}</div>
                    <div className='project-summary__description'>
                        <input value={this.state.description} onChange={e => this.setState({description: e.target.value})} />
                        {this.state.description !== this.props.description ? (
                            <button
                                onClick={async () => {
                                    await services.projects.updateDescription(this.props.name, this.state.description);
                                }}>
                                Save
                            </button>
                        ) : (
                            <i className='fa fa-pencil-alt' />
                        )}
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>DEPLOYMENT</div>
                    <div className='project-summary__section--row'>
                        <Card title='Sources' fields={[{type: FieldType.Text, name: 'url'}]} />
                        <Card title='Destinations' fields={[{type: FieldType.Text, name: 'server'}, {type: FieldType.Text, name: 'namespace'}]} />
                    </div>
                </div>
            </div>
        );
    }
}
