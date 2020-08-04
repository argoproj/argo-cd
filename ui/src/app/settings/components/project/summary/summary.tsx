import * as React from 'react';

import {ApplicationDestination, Project, ProjectSpec} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {GetProp, SetProp} from '../../utils';
import {Card} from '../card/card';
import {FieldData, FieldTypes} from '../card/row';
require('./summary.scss');

interface SummaryProps {
    proj: Project;
}

interface SummaryState extends IterableSpecFields {
    name: string;
    description: string;
    sourceRepos: string[];
    destinations: ApplicationDestination[];
    proj: Project;
}

interface IterableSpecFields {
    destinations: ApplicationDestination[];
    sourceRepos: string[];
}

export type IterableSpecField = ApplicationDestination | string;

const SourceFields: FieldData[] = [{name: 'url', type: FieldTypes.Text}];
const DestinationFields: FieldData[] = [{name: 'namespace', type: FieldTypes.Text}, {name: 'server', type: FieldTypes.Text}];

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    get descriptionChanged(): boolean {
        return this.state.description !== this.props.proj.spec.description;
    }

    constructor(props: SummaryProps) {
        super(props);
        this.state = {
            name: props.proj.metadata.name,
            description: props.proj.spec.description,
            sourceRepos: props.proj.spec.sourceRepos,
            destinations: props.proj.spec.destinations,
            proj: props.proj
        };
        this.addDestination = this.addDestination.bind(this);
        this.removeSource = this.removeSource.bind(this);
        this.removeDestination = this.removeDestination.bind(this);
        this.save = this.save.bind(this);
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
                        <Card<string>
                            title='Sources'
                            fields={SourceFields}
                            data={this.state.sourceRepos}
                            add={() => this.addSpecItem('sourceRepos', '')}
                            remove={this.removeSource}
                            save={value => this.save('sourceRepos', value as string)}
                        />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={DestinationFields}
                            data={this.state.destinations}
                            add={() => this.addSpecItem('destinations', {} as ApplicationDestination)}
                            remove={this.removeDestination}
                            save={() => null}
                        />
                    </div>
                </div>
            </div>
        );
    }

    private addSpecItem(key: keyof IterableSpecFields, empty: IterableSpecField) {
        const arr = (GetProp(this.state as IterableSpecFields, key) as IterableSpecField[]) || [];
        arr.push(empty);
        const update = {...this.state};
        SetProp(update, key as keyof SummaryState, arr);
        this.setState(update);
    }

    private addDestination() {
        const update = this.state.destinations || [];
        update.push({} as ApplicationDestination);
        this.setState({destinations: update});
    }

    private removeSource(i: number) {
        if (!this.state.sourceRepos || this.state.sourceRepos.length < 1) {
            return;
        }
        const update = this.state.sourceRepos;
        update.splice(i, 1);
        this.setState({sourceRepos: update});
    }

    private removeDestination(i: number) {
        if (!this.state.destinations || this.state.destinations.length < 1) {
            return;
        }
        const update = this.state.destinations;
        update.splice(i, 1);
        this.setState({destinations: update});
    }

    private async save(key: keyof IterableSpecFields, value: IterableSpecField): Promise<Project> {
        const update = {...this.state.proj};
        const arr = GetProp(this.state, key) as IterableSpecField[];
        arr.push(value as IterableSpecField);
        SetProp(update.spec, key as keyof ProjectSpec, arr);
        console.log(update);
        const res = await services.projects.updateLean(this.state.name, update);
        this.setState({proj: res});
        return res;
    }
}
