import * as React from 'react';

import {ApplicationDestination, GroupKind, Project, ProjectSpec} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {GetProp, SetProp} from '../../utils';
import {Card} from '../card/card';
import {FieldData, FieldTypes} from '../card/row';
require('./summary.scss');

interface SummaryProps {
    proj: Project;
}

interface SummaryState extends ProjectSpec {
    name: string;
    description: string;
    proj: Project;
}

enum IterableSpecFieldNames {
    destinations = 'destinations',
    sourceRepos = 'sourceRepos',
    clusterResourceWhitelist = 'clusterResourceWhitelist'
}

export type IterableSpecField = ApplicationDestination | GroupKind | string;

const SourceFields: FieldData[] = [{name: 'url', type: FieldTypes.Text}];
const DestinationFields: FieldData[] = [{name: 'namespace', type: FieldTypes.Text}, {name: 'server', type: FieldTypes.Text}];
const AllowedClusterResourceFields: FieldData[] = [{name: 'group', type: FieldTypes.Text}, {name: 'kind', type: FieldTypes.ResourceKindSelector}];

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    get descriptionChanged(): boolean {
        return this.state.description !== this.props.proj.spec.description;
    }

    constructor(props: SummaryProps) {
        super(props);
        this.state = {
            name: props.proj.metadata.name,
            proj: props.proj,
            ...props.proj.spec
        };
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
                            <input value={this.state.description} onChange={e => this.setState({description: e.target.value})} placeholder='Click to add a description' />
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
                            add={() => this.addSpecItem(IterableSpecFieldNames.sourceRepos, '')}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.sourceRepos, i)}
                            save={(i, value) => this.save(IterableSpecFieldNames.sourceRepos, i, value as string)}
                        />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={DestinationFields}
                            data={this.state.destinations}
                            add={() => this.addSpecItem(IterableSpecFieldNames.destinations, {} as ApplicationDestination)}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.destinations, i)}
                            save={(i, value) => this.save(IterableSpecFieldNames.destinations, i, value as ApplicationDestination)}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>ALLOW LIST</div>
                    <div className='project-summary__section--row'>
                        <Card<GroupKind>
                            title='Allowed Cluster Resources'
                            fields={AllowedClusterResourceFields}
                            data={this.state.clusterResourceWhitelist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.clusterResourceWhitelist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.clusterResourceWhitelist, idxs)}
                            save={(i, value) => this.save(IterableSpecFieldNames.clusterResourceWhitelist, i, value as string)}
                        />
                    </div>
                </div>
            </div>
        );
    }

    private async addSpecItem(key: keyof ProjectSpec, empty: IterableSpecField) {
        const arr = (GetProp(this.state as ProjectSpec, key) as IterableSpecField[]) || [];
        arr.push(empty);
        const update = {...this.state};
        SetProp(update, key as keyof SummaryState, arr);
        this.setState(update);
        this.reconcileProject();
    }
    private async removeSpecItems(key: keyof ProjectSpec, idxs: number[]) {
        const arr = GetProp(this.state as ProjectSpec, key) as IterableSpecField[];
        if (arr.length < 1 || !arr) {
            return;
        }
        while (idxs.length) {
            arr.splice(idxs.pop(), 1);
        }
        const update = {...this.state};
        SetProp(update, key as keyof SummaryState, arr);
        this.setState(update);
        const res = await services.projects.updateLean(this.state.name, update.proj);
        this.updateProject(res);
    }
    private reconcileProject() {
        const proj = this.state.proj;
        proj.spec.sourceRepos = this.state.sourceRepos;
        proj.spec.destinations = this.state.destinations;
        this.setState({proj});
    }
    private updateProject(proj: Project) {
        this.setState({
            name: proj.metadata.name,
            description: proj.spec.description,
            sourceRepos: proj.spec.sourceRepos,
            destinations: proj.spec.destinations,
            proj
        });
    }

    private async save(key: keyof ProjectSpec, idx: number, value: IterableSpecField): Promise<Project> {
        const update = {...this.state.proj};
        const arr = GetProp(this.state, key) as IterableSpecField[];
        arr[idx] = value as IterableSpecField;
        SetProp(update.spec, key as keyof ProjectSpec, arr);
        const res = await services.projects.updateLean(this.state.name, update);
        this.updateProject(res);
        return res;
    }
}
