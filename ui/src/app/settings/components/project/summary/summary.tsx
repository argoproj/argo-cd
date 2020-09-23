import * as React from 'react';

import {ApplicationDestination, GroupKind, OrphanedResource, Project, ProjectSignatureKey, ProjectSpec} from '../../../../shared/models';
import {services} from '../../../../shared/services';
import {GetProp, SetProp} from '../../utils';
import {Card} from '../card/card';
import {FieldData, FieldSizes, FieldTypes} from '../card/field';

require('./summary.scss');

enum DocLinks {
    OrphanedResources = 'https://github.com/argoproj/argo-cd/blob/master/docs/user-guide/orphaned-resources.md',
    Projects = 'https://github.com/argoproj/argo-cd/blob/master/docs/user-guide/projects.md'
}

enum HelpTips {
    Sources = 'Git repositories where application manifests are permitted to be retrieved from',
    Destinations = 'Cluster and namespaces where applications are permitted to be deployed to',
    ClusterResourceWhitelist = 'Cluster-scoped K8s API Groups and Kinds which are permitted to be deployed',
    ClusterResourceBlacklist = 'Cluster-scoped K8s API Groups and Kinds which are not permitted to be deployed',
    NamespaceResourceBlacklist = 'Namespace-scoped K8s API Groups and Kinds which are prohibited from being deployed',
    NamespaceResourceWhitelist = 'Namespace-scoped K8s API Groups and Kinds which are permitted to deploy',
    SignatureKeys = 'IDs of GnuPG keys that commits must be signed with in order to be allowed to sync to',
    OrphanedResources = 'Enables monitoring of top level resources in the application target namespace'
}

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
    namespaceResourceWhitelist = 'namespaceResourceWhitelist',
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
            <div className='project-summary argo-container'>
                <div>
                    <div className='project-summary__label'>
                        PROJECT&nbsp;
                        <a href={DocLinks.Projects} target='_blank'>
                            <i className='fas fa-question-circle' />
                        </a>
                    </div>
                    <div className='project-summary__title'>{this.state.name}</div>
                    <div className='project-summary__description'>
                        <div style={{marginBottom: '1em'}}>
                            <input
                                className='argo-field'
                                value={this.state.description}
                                onChange={e => this.setState({description: e.target.value})}
                                placeholder='Click to add a description'
                            />
                        </div>
                        <div>
                            {this.descriptionChanged ? (
                                <React.Fragment>
                                    <button
                                        className='argo-button argo-button--base'
                                        onClick={async () => {
                                            const update = {...this.state.proj};
                                            update.spec.description = this.state.description;
                                            const res = await services.projects.updateProj(update);
                                            this.setState({proj: res});
                                        }}>
                                        SAVE
                                    </button>
                                    <button
                                        className='argo-button argo-button--base-o'
                                        onClick={async () => {
                                            this.setState({description: this.props.proj.spec.description || ''});
                                        }}>
                                        REVERT
                                    </button>
                                </React.Fragment>
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
                            values={this.state.sourceRepos}
                            save={values => this.save(IterableSpecFieldNames.sourceRepos, values as string[])}
                            help={HelpTips.Sources}
                            fullWidth={true}
                        />
                        <Card<ApplicationDestination>
                            title='Destinations'
                            fields={this.state.fields.destinations}
                            values={this.state.destinations}
                            save={values => this.save(IterableSpecFieldNames.destinations, values as ApplicationDestination[])}
                            help={HelpTips.Destinations}
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
                            values={this.state.clusterResourceWhitelist}
                            save={values => this.save(IterableSpecFieldNames.clusterResourceWhitelist, values as GroupKind[])}
                            help={HelpTips.ClusterResourceWhitelist}
                            fullWidth={false}
                        />
                        <Card<GroupKind>
                            title='Allowed Namespace Resources'
                            fields={this.state.fields.resources}
                            values={this.state.namespaceResourceWhitelist}
                            save={values => this.save(IterableSpecFieldNames.namespaceResourceWhitelist, values as GroupKind[])}
                            help={HelpTips.NamespaceResourceWhitelist}
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
                            values={this.state.clusterResourceBlacklist}
                            save={values => this.save(IterableSpecFieldNames.clusterResourceBlacklist, values as GroupKind[])}
                            help={HelpTips.ClusterResourceBlacklist}
                            fullWidth={false}
                        />
                        <Card<GroupKind>
                            title='Denied Namespace Resources'
                            fields={this.state.fields.resources}
                            values={this.state.namespaceResourceBlacklist}
                            save={values => this.save(IterableSpecFieldNames.namespaceResourceBlacklist, values as GroupKind[])}
                            help={HelpTips.NamespaceResourceBlacklist}
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
                            values={this.state.signatureKeys}
                            save={values => this.save(IterableSpecFieldNames.signatureKeys, values as ProjectSignatureKey[])}
                            help={HelpTips.SignatureKeys}
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
                                values={this.state.orphanedResources ? this.state.orphanedResources.ignore : null}
                                disabled={!this.orphanedResourceMonitoringEnabled}
                                save={async values => {
                                    const update = {...this.state.proj};
                                    update.spec.orphanedResources.ignore = values;
                                    const res = await services.projects.updateProj(update);
                                    this.updateProject(res);
                                    return res;
                                }}
                                help={HelpTips.OrphanedResources}
                                docs={DocLinks.OrphanedResources}
                                fullWidth={false}
                            />
                        </div>
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
            <div className='card__row'>
                <div className='card__col-select-button card__col'>
                    <button className={`card__button card__button-select${status ? '--selected' : ''}`} onClick={() => change(!status)}>
                        <i className='fa fa-check' />
                    </button>
                </div>
                <div className={'card__col'}>{label}</div>
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
        const res = await services.projects.updateProj(update);
        this.updateProject(res);
        return;
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
    private async save<T>(key: keyof ProjectSpec, values: T[]): Promise<any> {
        const update = {...this.state.proj};
        SetProp(update.spec, key as keyof ProjectSpec, values);
        const res = await services.projects.updateProj(update);
        this.updateProject(res);
        return GetProp(res.spec as ProjectSpec, key as keyof ProjectSpec);
    }
}
