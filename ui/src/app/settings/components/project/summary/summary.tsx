import * as React from 'react';

import {Project} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {Card} from '../card/card';
import {FieldData, FieldTypes} from '../card/row';
require('./summary.scss');

interface SummaryProps {
    proj: Project;
}

interface SummaryState {
    name: string;
    description: string;
    proj: Project;
}

const SourceFields: FieldData[] = [{name: 'url', type: FieldTypes.Text}];
interface SourceData {
    url: string;
}

const DestinationFields: FieldData[] = [{name: 'namespace', type: FieldTypes.Text}, {name: 'server', type: FieldTypes.Text}];
interface DestinationData {
    namespace: string;
    server: string;
}

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    constructor(props: SummaryProps) {
        super(props);
        this.state = {name: props.proj.metadata.name, description: props.proj.spec.description, proj: props.proj};
    }

    get descriptionChanged(): boolean {
        return this.state.description !== this.props.proj.spec.description;
    }

    public render() {
        return (
            <div className='project-summary'>
                <div>
                    <div className='project-summary__label'>PROJECT</div>
                    <div className='project-summary__title'>{this.state.name}</div>
                    <div className='project-summary__description'>
                        <div className='project-summary__description--row'>
                            <div className='project-summary__col'>
                                <i className='fa fa-pencil-alt' />
                            </div>
                            <input value={this.state.description} onChange={e => this.setState({description: e.target.value})} />
                        </div>
                        <div className='project-summary__description--row'>
                            {this.descriptionChanged ? (
                                <div className='project-summary__description--actions'>
                                    <button
                                        className='project__button project__button-save'
                                        onClick={async () => {
                                            const update = {...this.state.proj};
                                            update.spec.description = this.state.description;
                                            const res = await services.projects.updateLean(this.state.name, update);
                                            this.setState({proj: res});
                                        }}>
                                        SAVE
                                    </button>
                                    <button
                                        className='project__button project__button-cancel'
                                        onClick={async () => {
                                            this.setState({description: this.props.proj.spec.description});
                                        }}>
                                        REVERT
                                    </button>
                                </div>
                            ) : null}
                        </div>
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>DEPLOYMENT</div>
                    <div className='project-summary__section--row'>
                        <Card<SourceData> title='Sources' fields={SourceFields} data={[{url: 'helloworld.com'}]} />
                        <Card<DestinationData> title='Destinations' fields={DestinationFields} data={[{namespace: 'default', server: 'myserver'}]} />
                    </div>
                </div>
            </div>
        );
    }
}
