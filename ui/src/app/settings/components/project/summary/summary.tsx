import * as React from 'react';

import {ApplicationDestination, Project} from '../../../../shared/models';
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
    sources: string[];
    destinations: ApplicationDestination[];
    proj: Project;
}

const SourceFields: FieldData[] = [{name: 'url', type: FieldTypes.Text}];
const DestinationFields: FieldData[] = [{name: 'namespace', type: FieldTypes.Text}, {name: 'server', type: FieldTypes.Text}];

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    get descriptionChanged(): boolean {
        return this.state.description !== this.props.proj.spec.description;
    }

    get sources(): {url: string}[] {
        return this.state.sources
            ? this.state.sources.map(item => {
                  return {url: item};
              })
            : [];
    }

    constructor(props: SummaryProps) {
        super(props);
        this.state = {
            name: props.proj.metadata.name,
            description: props.proj.spec.description,
            sources: props.proj.spec.sourceRepos,
            destinations: props.proj.spec.destinations,
            proj: props.proj
        };
        this.addSource = this.addSource.bind(this);
        this.addDestination = this.addDestination.bind(this);
        this.removeSource = this.removeSource.bind(this);
        this.removeDestination = this.removeDestination.bind(this);
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
                        <Card<{url: string}> title='Sources' fields={SourceFields} data={this.sources} add={this.addSource} remove={this.removeSource} />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={DestinationFields}
                            data={this.state.destinations}
                            add={this.addDestination}
                            remove={this.removeDestination}
                        />
                    </div>
                </div>
            </div>
        );
    }

    private addSource() {
        const update = this.state.sources || [];
        update.push('');
        this.setState({sources: update});
    }

    private addDestination() {
        const update = this.state.destinations || [];
        update.push({} as ApplicationDestination);
        this.setState({destinations: update});
    }

    private removeSource(i: number) {
        if (!this.state.sources || this.state.sources.length < 1) {
            return;
        }
        const update = this.state.sources;
        update.splice(i, 1);
        this.setState({sources: update});
    }

    private removeDestination(i: number) {
        if (!this.state.destinations || this.state.destinations.length < 1) {
            return;
        }
        const update = this.state.destinations;
        update.splice(i, 1);
        this.setState({destinations: update});
    }
}
