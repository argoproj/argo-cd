import * as React from 'react';

import {ApplicationDestination, GroupKind, OrphanedResource, Project, ProjectSignatureKey, ProjectSpec} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {GetProp, SetProp} from '../../utils';
import {Card} from '../card/card';
import {FieldData, FieldTypes} from '../card/row';
import {DocLinks} from '../doc-links';
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
    clusterResourceWhitelist = 'clusterResourceWhitelist',
    clusterResourceBlacklist = 'clusterResourceBlacklist',
    namespaceResourceBlacklist = 'namespaceResourceBlacklist',
    signatureKeys = 'signatureKeys'
}

export type IterableSpecField = ApplicationDestination | GroupKind | ProjectSignatureKey | string;

const SourceFields: FieldData[] = [{name: 'url', type: FieldTypes.Url}];
const DestinationFields: FieldData[] = [{name: 'namespace', type: FieldTypes.Text}, {name: 'server', type: FieldTypes.Text}];
const ResourceFields: FieldData[] = [{name: 'group', type: FieldTypes.Text}, {name: 'kind', type: FieldTypes.ResourceKindSelector}];
const SignatureKeyFields: FieldData[] = [{name: 'keyID', type: FieldTypes.Text}];
const OrphanedResourceFields: FieldData[] = [{name: 'group', type: FieldTypes.Text}, {name: 'kind', type: FieldTypes.ResourceKindSelector}, {name: 'name', type: FieldTypes.Text}];

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    get descriptionChanged(): boolean {
        return this.state.description !== this.props.proj.spec.description;
    }
    get orphanedResourceMonitoringEnabled(): boolean {
        return this.state.proj.spec.orphanedResources !== null && !!this.state.proj.spec.orphanedResources;
    }
    get orphanedResourceWarningEnabled(): boolean {
        if (this.state.proj.spec.orphanedResources) {
            return !!this.state.proj.spec.orphanedResources.warn;
        }
        return false;
    }

    constructor(props: SummaryProps) {
        super(props);
        this.state = {
            name: props.proj.metadata.name,
            proj: props.proj,
            ...props.proj.spec
        };
        this.save = this.save.bind(this);
        this.setOrphanedResourceWarning = this.setOrphanedResourceWarning.bind(this);
        this.setOrphanedResourceMonitoring = this.setOrphanedResourceMonitoring.bind(this);
    }

    public render() {
        return (
            <div className='project-summary'>
                <div>
                    <div className='project-summary__label'>
                        PROJECT&nbsp;
                        <a href={DocLinks.Projects} target='_blank'>
                            <i className='fas fa-question-circle' />
                        </a>
                    </div>
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
                    <div className='project-summary__label'>
                        DEPLOYMENT&nbsp;
                        <i className='fas fa-paper-plane' />
                    </div>
                    <div className='project-summary__section--row'>
                        <Card<string>
                            title='Sources'
                            fields={SourceFields}
                            data={this.state.sourceRepos}
                            add={() => this.addSpecItem(IterableSpecFieldNames.sourceRepos, '')}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.sourceRepos, i)}
                            save={(i, value) => this.save(IterableSpecFieldNames.sourceRepos, i, value as string)}
                            docs={null}
                        />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={DestinationFields}
                            data={this.state.destinations}
                            add={() => this.addSpecItem(IterableSpecFieldNames.destinations, {} as ApplicationDestination)}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.destinations, i)}
                            save={(i, value) => this.save(IterableSpecFieldNames.destinations, i, value as ApplicationDestination)}
                            docs={null}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>
                        ALLOW LIST&nbsp;
                        <i className='fas fa-tasks' />
                    </div>
                    <div className='project-summary__section--row'>
                        <Card<GroupKind>
                            title='Allowed Cluster Resources'
                            fields={ResourceFields}
                            data={this.state.clusterResourceWhitelist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.clusterResourceWhitelist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.clusterResourceWhitelist, idxs)}
                            save={(i, value) => this.save(IterableSpecFieldNames.clusterResourceWhitelist, i, value as string)}
                            docs={null}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>
                        DENY LIST&nbsp;
                        <i className='fas fa-ban' />
                    </div>
                    <div className='project-summary__section--row'>
                        <Card<GroupKind>
                            title='Denied Cluster Resources'
                            fields={ResourceFields}
                            data={this.state.clusterResourceBlacklist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.clusterResourceBlacklist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.clusterResourceBlacklist, idxs)}
                            save={(i, value) => this.save(IterableSpecFieldNames.clusterResourceBlacklist, i, value as string)}
                            docs={null}
                        />
                        <Card<GroupKind>
                            title='Denied Namespace Resources'
                            fields={ResourceFields}
                            data={this.state.namespaceResourceBlacklist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.namespaceResourceBlacklist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.namespaceResourceBlacklist, idxs)}
                            save={(i, value) => this.save(IterableSpecFieldNames.namespaceResourceBlacklist, i, value as string)}
                            docs={null}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>
                        SIGNATURE KEYS&nbsp;
                        <i className='fas fa-key' />
                    </div>
                    <div className='project-summary__section--row'>
                        <Card<ProjectSignatureKey>
                            title='Required Signature Keys'
                            fields={SignatureKeyFields}
                            data={this.state.signatureKeys}
                            add={() => this.addSpecItem(IterableSpecFieldNames.signatureKeys, {} as ProjectSignatureKey)}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.signatureKeys, i)}
                            save={(i, value) => this.save(IterableSpecFieldNames.signatureKeys, i, value as string)}
                            docs={null}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>
                        ORPHANED RESOURCES&nbsp;
                        <i className='fas fa-file' />
                    </div>
                    {this.toggleSwitch('MONITORING', this.orphanedResourceMonitoringEnabled, this.setOrphanedResourceMonitoring)}
                    <div className='project-summary__section--row'>
                        {this.orphanedResourceMonitoringEnabled ? (
                            <div>
                                {this.toggleSwitch('WARN', this.orphanedResourceWarningEnabled, this.setOrphanedResourceWarning)}
                                <Card<OrphanedResource>
                                    title='Orphaned Resource Ignore List'
                                    fields={OrphanedResourceFields}
                                    data={this.state.orphanedResources.ignore}
                                    add={() => {
                                        const obj = GetProp(this.state as ProjectSpec, 'orphanedResources');
                                        if (!obj || Object.keys(obj).length < 1) {
                                            return;
                                        }
                                        if (!obj.ignore) {
                                            obj.ignore = [];
                                        }
                                        obj.ignore.push({} as OrphanedResource);
                                        const update = {...this.state};
                                        SetProp(update, 'orphanedResources', obj);
                                        this.setState(update);
                                    }}
                                    remove={idxs => {
                                        const obj = GetProp(this.state as ProjectSpec, 'orphanedResources');
                                        if (!obj || Object.keys(obj).length < 1 || !obj.ignore) {
                                            return;
                                        }
                                        const arr = obj.ignore;
                                        if (arr.length < 1) {
                                            return;
                                        }
                                        while (idxs.length) {
                                            arr.splice(idxs.pop(), 1);
                                        }
                                        obj.ignore = arr;
                                        const update = {...this.state};
                                        SetProp(update, 'orphanedResources', obj);
                                        this.setState(update);
                                    }}
                                    save={async (i, value) => {
                                        const update = {...this.state.proj};
                                        const obj = update.spec.orphanedResources;
                                        const arr = obj.ignore;
                                        arr[i] = value as OrphanedResource;
                                        obj.ignore = arr;
                                        update.spec.orphanedResources = obj;
                                        const res = await services.projects.updateLean(this.state.name, update);
                                        this.updateProject(res);
                                        return res;
                                    }}
                                    docs={DocLinks.OrphanedResources}
                                />
                            </div>
                        ) : null}
                    </div>
                </div>
            </div>
        );
    }
    private toggleSwitch(label: string, status: boolean, change: (_: boolean) => void) {
        return (
            <div className='project-summary__monitoring-toggle'>
                <b>{label}</b>
                <div className='project__toggle'>
                    <button className={`project__button project__button--on${status ? '__selected' : '__deselected'}`} onClick={() => change(true)}>
                        ON
                    </button>
                    <button className={`project__button project__button--off${!status ? '__selected' : '__deselected'}`} onClick={() => change(false)}>
                        OFF
                    </button>
                </div>
            </div>
        );
    }
    private setOrphanedResourceWarning(on: boolean) {
        this.updateOrphanedResources(true, on);
    }
    private setOrphanedResourceMonitoring(on: boolean) {
        this.updateOrphanedResources(on, false);
    }
    private async updateOrphanedResources(on: boolean, warn: boolean) {
        const update = {...this.state.proj};
        if (on) {
            const cur = update.spec.orphanedResources || ({} as {warn: boolean; ignore: OrphanedResource[]});
            cur.warn = warn;
            SetProp(update.spec, 'orphanedResources', cur);
        } else {
            if (update.spec.orphanedResources) {
                delete update.spec.orphanedResources;
            }
        }
        const res = await services.projects.updateLean(this.state.name, update);
        this.updateProject(res);
        return;
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
        for (const key of Object.keys(proj.spec)) {
            const cur = GetProp(this.state, key as keyof ProjectSpec);
            SetProp(proj.spec, key as keyof ProjectSpec, cur);
        }
        this.setState({proj});
    }
    private updateProject(proj: Project) {
        const update = {...this.state};
        for (const key of Object.keys(proj.spec)) {
            const cur = GetProp(proj.spec, key as keyof ProjectSpec);
            SetProp(update, key as keyof ProjectSpec, cur);
        }
        this.setState(update);
        this.setState({name: proj.metadata.name, proj});
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
