import * as React from 'react';

import {Project} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {Card, FieldType} from '../card';
require('./summary.scss');

interface SummaryProps {
    proj: Project;
}

interface SummaryState {
    name: string;
    description: string;
    proj: Project;
}

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    constructor(props: SummaryProps) {
        super(props);
        this.state = {name: props.proj.metadata.name, description: props.proj.spec.description, proj: props.proj};
    }

    public render() {
        return (
            <div className='project-summary'>
                <div>
                    <div className='project-summary__label'>PROJECT</div>
                    <div className='project-summary__title'>{this.state.name}</div>
                    <div className='project-summary__description'>
                        <div className='project-summary__col'>
                            {this.state.description !== this.props.proj.spec.description ? (
                                <button
                                    className='project__button project__button-save'
                                    onClick={async () => {
                                        const update = {...this.state.proj};
                                        update.spec.description = this.state.description;
                                        await services.projects.updateLean(this.state.name, update);
                                    }}>
                                    SAVE
                                </button>
                            ) : (
                                <i className='fa fa-pencil-alt' />
                            )}
                        </div>
                        <input value={this.state.description} onChange={e => this.setState({description: e.target.value})} />
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
