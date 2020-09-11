import * as React from 'react';

import {ApplicationDestination, GroupKind, OrphanedResource, Project, ProjectSignatureKey, ProjectSpec} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {GetProp, SetProp} from '../../utils';
import {Card} from '../card/card';
import {FieldData, FieldSizes, FieldTypes} from '../card/field';
import {DocLinks} from '../doc-links';

require('./summary.scss');
require('../card/card.scss');

interface SummaryProps {
    proj: Project;
}

interface ProjectFields {
    sources: FieldData[];
    destinations: FieldData[];
    resources: FieldData[];
    signatureKeys: FieldData[];
    orphanedResources: FieldData[];
}

interface SummaryState extends ProjectSpec {
    name: string;
    description: string;
    proj: Project;
    fields: ProjectFields;
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

export class ProjectSummary extends React.Component<SummaryProps, SummaryState> {
    get descriptionChanged(): boolean {
        if (!this.state.proj.spec.description && this.state.description === '') {
            return false;
        }
        return this.state.description !== this.state.proj.spec.description;
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
        const fields: ProjectFields = {
            sources: [{name: 'url', type: FieldTypes.Url, size: FieldSizes.Grow}],
            destinations: [{name: 'namespace', type: FieldTypes.Text, size: FieldSizes.Normal}, {name: 'server', type: FieldTypes.Text, size: FieldSizes.Grow}],
            resources: [{name: 'group', type: FieldTypes.AutoComplete, size: FieldSizes.Normal}, {name: 'kind', type: FieldTypes.ResourceKindSelector, size: FieldSizes.Normal}],
            signatureKeys: [{name: 'keyID', type: FieldTypes.AutoComplete, size: FieldSizes.Normal}],
            orphanedResources: [
                {name: 'group', type: FieldTypes.Text, size: FieldSizes.Normal},
                {name: 'kind', type: FieldTypes.ResourceKindSelector, size: FieldSizes.Normal},
                {name: 'name', type: FieldTypes.Text, size: FieldSizes.Normal}
            ]
        };
        this.state = {
            name: props.proj.metadata.name,
            proj: props.proj,
            ...props.proj.spec,
            fields
        };
        this.save = this.save.bind(this);
        this.setOrphanedResourceWarning = this.setOrphanedResourceWarning.bind(this);
        this.setOrphanedResourceMonitoring = this.setOrphanedResourceMonitoring.bind(this);
    }
    public async componentDidMount() {
        const fields = {...this.state.fields};
        const keys = await this.getGpgKeyIDs();
        fields.signatureKeys[0].values = keys;
        this.setState({fields});
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
                                        className='card__button card__button-save'
                                        onClick={async () => {
                                            const update = {...this.state.proj};
                                            update.spec.description = this.state.description;
                                            const res = await services.projects.updateLean(this.state.name, update);
                                            this.setState({proj: res});
                                        }}>
                                        SAVE
                                    </button>
                                    <button
                                        className='card__button card__button-cancel'
                                        onClick={async () => {
                                            this.setState({description: this.props.proj.spec.description || ''});
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
                            fields={this.state.fields.sources}
                            data={this.state.sourceRepos}
                            add={() => this.addSpecItem(IterableSpecFieldNames.sourceRepos, '')}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.sourceRepos, i)}
                            save={(i, values) => this.save(IterableSpecFieldNames.sourceRepos, i, values as string[])}
                            fullWidth={true}
                        />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={this.state.fields.destinations}
                            data={this.state.destinations}
                            add={() => this.addSpecItem(IterableSpecFieldNames.destinations, {} as ApplicationDestination)}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.destinations, i)}
                            save={(i, values) => this.save(IterableSpecFieldNames.destinations, i, values as ApplicationDestination[])}
                            fullWidth={true}
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
                            fields={this.state.fields.resources}
                            data={this.state.clusterResourceWhitelist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.clusterResourceWhitelist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.clusterResourceWhitelist, idxs)}
                            save={(i, values) => this.save(IterableSpecFieldNames.clusterResourceWhitelist, i, values as string[])}
                            fullWidth={false}
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
                            fields={this.state.fields.resources}
                            data={this.state.clusterResourceBlacklist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.clusterResourceBlacklist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.clusterResourceBlacklist, idxs)}
                            save={(i, values) => this.save(IterableSpecFieldNames.clusterResourceBlacklist, i, values as string[])}
                            fullWidth={false}
                        />
                        <Card<GroupKind>
                            title='Denied Namespace Resources'
                            fields={this.state.fields.resources}
                            data={this.state.namespaceResourceBlacklist}
                            add={() => this.addSpecItem(IterableSpecFieldNames.namespaceResourceBlacklist, {} as GroupKind)}
                            remove={idxs => this.removeSpecItems(IterableSpecFieldNames.namespaceResourceBlacklist, idxs)}
                            save={(i, values) => this.save(IterableSpecFieldNames.namespaceResourceBlacklist, i, values as string[])}
                            fullWidth={false}
                        />
                    </div>
                </div>
                <div className='project-summary__section'>
                    <div className='project-summary__label'>
                        GPG SIGNATURE KEYS&nbsp;
                        <i className='fas fa-key' />
                    </div>
                    <div className='project-summary__section--row'>
                        <Card<ProjectSignatureKey>
                            title='Required Signature Keys'
                            fields={this.state.fields.signatureKeys}
                            data={this.state.signatureKeys}
                            add={() => this.addSpecItem(IterableSpecFieldNames.signatureKeys, {} as ProjectSignatureKey)}
                            remove={i => this.removeSpecItems(IterableSpecFieldNames.signatureKeys, i)}
                            save={(i, values) => this.save(IterableSpecFieldNames.signatureKeys, i, values as string[])}
                            fullWidth={false}
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
                        <div>
                            {this.toggleSwitch('WARN', this.orphanedResourceWarningEnabled, this.setOrphanedResourceWarning)}
                            <Card<OrphanedResource>
                                title='Orphaned Resource Ignore List'
                                fields={this.state.fields.orphanedResources}
                                data={this.state.orphanedResources ? this.state.orphanedResources.ignore : null}
                                add={async () => {
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
                                    return {} as OrphanedResource;
                                }}
                                disabled={!this.orphanedResourceMonitoringEnabled}
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
                                save={async (idxs, values) => {
                                    const update = {...this.state.proj};
                                    const obj = update.spec.orphanedResources;
                                    const arr = obj.ignore;
                                    for (const i of idxs) {
                                        arr[i] = values[i] as OrphanedResource;
                                    }
                                    obj.ignore = arr;
                                    update.spec.orphanedResources = obj;
                                    const res = await services.projects.updateLean(this.state.name, update);
                                    this.updateProject(res);
                                    return res;
                                }}
                                docs={DocLinks.OrphanedResources}
                                fullWidth={false}
                            />
                        </div>
                        {/*) : null}*/}
                    </div>
                </div>
            </div>
        );
    }
    private async getGpgKeyIDs(): Promise<string[]> {
        const keys = await services.gpgkeys.list();
        const ids = [];
        for (const key of keys) {
            ids.push(key.keyID);
        }
        return ids;
    }
    private toggleSwitch(label: string, status: boolean, change: (_: boolean) => void) {
        return (
            <div className='project-summary__monitoring-toggle'>
                <b>{label}</b>
                <div className='project__toggle'>
                    <button className={`card__button card__button--on${status ? '__selected' : '__deselected'}`} onClick={() => change(true)}>
                        ON
                    </button>
                    <button className={`card__button card__button--off${!status ? '__selected' : '__deselected'}`} onClick={() => change(false)}>
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
    private async addSpecItem(key: keyof ProjectSpec, empty: IterableSpecField): Promise<IterableSpecField> {
        const arr = (GetProp(this.state as ProjectSpec, key) as IterableSpecField[]) || [];
        arr.push(empty);
        const update = {...this.state};
        SetProp(update, key as keyof SummaryState, arr);
        this.setState(update);
        this.reconcileProject();
        return empty;
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
    private async save(key: keyof ProjectSpec, idxs: number[], values: IterableSpecField[]): Promise<any> {
        const update = {...this.state.proj};
        const arr = GetProp(this.state, key) as IterableSpecField[];
        values.forEach((value, i) => {
            arr[idxs[i]] = values[i] as IterableSpecField;
        });
        SetProp(update.spec, key as keyof ProjectSpec, arr);
        const res = await services.projects.updateLean(this.state.name, update);
        this.updateProject(res);
        return GetProp(res.spec as ProjectSpec, key as keyof ProjectSpec);
    }
}
