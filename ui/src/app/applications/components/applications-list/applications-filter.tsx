import {Checkbox, DataLoader, Tooltip} from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import {TagsInput} from '../../../shared/components';
import * as models from '../../../shared/models';
import {AppsListPreferences, services} from '../../../shared/services';

export interface ApplicationsFilterProps {
    clusters: models.Cluster[];
    applications: models.Application[];
    pref: AppsListPreferences;
    onChange: (pref: AppsListPreferences) => any;
}

const maxFilterItemsCount = 10;

class ItemsFilter extends React.Component<
    {
        items: {name: string; count: number}[];
        type: string;
        selected: string[];
        onChange: (selected: string[]) => any;
    },
    {
        expanded: boolean;
    }
> {
    constructor(props: any) {
        super(props);
        this.state = {expanded: false};
    }

    public render() {
        const unavailableSelected = this.props.selected.filter(selected => !this.props.items.some(item => item.name === selected));
        const items = this.props.items.sort((first, second) => (first.name > second.name ? 1 : -1)).concat(unavailableSelected.map(selected => ({name: selected, count: 0})));
        return (
            <React.Fragment>
                <ul className={classNames('applications-list__filter', {'applications-list__filter--expanded': this.state.expanded})}>
                    {items.map(item => (
                        <li key={item.name}>
                            <div className='applications-list__filter-label'>
                                <Checkbox
                                    checked={this.props.selected.indexOf(item.name) > -1}
                                    id={`filter-${this.props.type}-${item.name}`}
                                    onChange={() => {
                                        const newSelected = this.props.selected.slice();
                                        const index = newSelected.indexOf(item.name);
                                        if (index > -1) {
                                            newSelected.splice(index, 1);
                                        } else {
                                            newSelected.push(item.name);
                                        }
                                        this.props.onChange(newSelected);
                                    }}
                                />{' '}
                                <label title={item.name} htmlFor={`filter-${this.props.type}-${item.name}`}>
                                    {item.name}
                                </label>
                            </div>{' '}
                            <span>{item.count}</span>
                        </li>
                    ))}
                </ul>
                {items.length > maxFilterItemsCount && <a onClick={() => this.setState({expanded: !this.state.expanded})}>{this.state.expanded ? 'collapse' : 'expand'}</a>}
            </React.Fragment>
        );
    }
}

function getLabelsSuggestions(labels: Map<string, Set<string>>) {
    const suggestions = new Array<string>();
    Array.from(labels.entries()).forEach(([label, values]) => {
        suggestions.push(label);
        values.forEach(val => suggestions.push(`${label}=${val}`));
    });
    return suggestions;
}

export class ApplicationsFilter extends React.Component<ApplicationsFilterProps, {expanded: boolean}> {
    constructor(props: ApplicationsFilterProps) {
        super(props);
        this.state = {expanded: false};
    }

    public render() {
        const {applications, pref, onChange} = this.props;

        const sync = new Map<string, number>();
        Object.keys(models.SyncStatuses).forEach(key => sync.set(models.SyncStatuses[key], 0));
        applications.filter(app => app.status.sync.status).forEach(app => sync.set(app.status.sync.status, (sync.get(app.status.sync.status) || 0) + 1));
        const health = new Map<string, number>();
        Object.keys(models.HealthStatuses).forEach(key => health.set(models.HealthStatuses[key], 0));
        applications.filter(app => app.status.health.status).forEach(app => health.set(app.status.health.status, (health.get(app.status.health.status) || 0) + 1));
        const labels = new Map<string, Set<string>>();
        applications
            .filter(app => app.metadata && app.metadata.labels)
            .forEach(app =>
                Object.keys(app.metadata.labels).forEach(label => {
                    let values = labels.get(label);
                    if (!values) {
                        values = new Set<string>();
                        labels.set(label, values);
                    }
                    values.add(app.metadata.labels[label]);
                })
            );

        const filtersCount = AppsListPreferences.countEnabledFilters(pref);

        return (
            <div
                className={classNames('applications-list__filters-container', {'applications-list__filters-container--expanded': this.state.expanded})}
                onClick={() => {
                    if (window.innerWidth < 1440) {
                        this.setState({expanded: !this.state.expanded});
                    }
                }}>
                <div className={classNames('applications-list__filters-title', {'applications-list__filters-title--expanded': this.state.expanded})}>
                    <Tooltip content='Filters'>
                        <i className='fa fa-filter' />
                    </Tooltip>
                    <div style={{marginLeft: 'auto'}} />
                    <div className='applications-list__filters-title__counter tags-input__tag' style={filtersCount === 0 ? {display: 'none'} : {}}>
                        {filtersCount} Filter{filtersCount === 1 ? '' : 's'}
                        <Tooltip content='Clear Filters'>
                            <i
                                className='fa fa-times'
                                onClick={() => {
                                    AppsListPreferences.clearFilters(this.props.pref);
                                    this.props.onChange(this.props.pref);
                                }}
                            />
                        </Tooltip>
                    </div>
                    <Tooltip content={(this.state.expanded ? 'Collapse' : 'Expand') + ' Filters'}>
                        <i
                            className={classNames('fa applications-list__filters-title__expander', {'fa-chevron-up': this.state.expanded, 'fa-chevron-down': !this.state.expanded})}
                            onClick={() => this.setState({expanded: !this.state.expanded})}
                        />
                    </Tooltip>
                </div>
                <div className='applications-list__filters-container-contents row'>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <div className='applications-list__filter-title'>Sync</div>
                        <ItemsFilter
                            selected={pref.syncFilter}
                            onChange={selected => onChange({...pref, syncFilter: selected})}
                            items={Array.from(sync.keys()).map(status => ({name: status, count: sync.get(status) || 0}))}
                            type='sync'
                        />
                    </div>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <div className='applications-list__filter-title'>Health</div>
                        <ItemsFilter
                            selected={pref.healthFilter}
                            onChange={selected => onChange({...pref, healthFilter: selected})}
                            items={Array.from(health.keys()).map(status => ({name: status, count: health.get(status) || 0}))}
                            type='health'
                        />
                    </div>
                    <div className='columns small-12 medium-6 xxlarge-12'>
                        <div className='applications-list__filter'>
                            <div className='applications-list__filter-title'>Labels</div>
                            <ul>
                                <li>
                                    <TagsInput
                                        placeholder=''
                                        autocomplete={getLabelsSuggestions(labels)}
                                        tags={pref.labelsFilter}
                                        onChange={selected => onChange({...pref, labelsFilter: selected})}
                                    />
                                </li>
                            </ul>
                            <div className='applications-list__filter-title'>Projects</div>
                            <ul>
                                <li>
                                    <DataLoader load={() => services.projects.list('items.metadata.name')}>
                                        {projects => {
                                            const projAppCount = new Map<string, number>();
                                            projects.forEach(proj => projAppCount.set(proj.metadata.name, 0));
                                            applications.forEach(app => projAppCount.set(app.spec.project, (projAppCount.get(app.spec.project) || 0) + 1));
                                            return (
                                                <TagsInput
                                                    placeholder='default'
                                                    autocomplete={projects.map(proj => proj.metadata.name)}
                                                    tags={pref.projectsFilter}
                                                    onChange={selected => onChange({...pref, projectsFilter: selected})}
                                                />
                                            );
                                        }}
                                    </DataLoader>
                                </li>
                            </ul>
                            <div className='applications-list__filter-title'>Clusters</div>
                            <ul>
                                <li>
                                    <TagsInput
                                        placeholder='in-cluster (https://kubernetes.default.svc)'
                                        autocomplete={Array.from(new Set(applications.map(app => this.getClusterDetail(app.spec.destination)).filter(item => !!item))).filter(
                                            ns => pref.clustersFilter.indexOf(ns) === -1
                                        )}
                                        tags={pref.clustersFilter}
                                        onChange={selected => onChange({...pref, clustersFilter: selected})}
                                    />
                                </li>
                            </ul>
                            <div className='applications-list__filter-title'>Namespaces</div>
                            <ul>
                                <li>
                                    <TagsInput
                                        placeholder='*-us-west-*'
                                        autocomplete={Array.from(new Set(applications.map(app => app.spec.destination.namespace).filter(item => !!item))).filter(
                                            ns => pref.namespacesFilter.indexOf(ns) === -1
                                        )}
                                        tags={pref.namespacesFilter}
                                        onChange={selected => onChange({...pref, namespacesFilter: selected})}
                                    />
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
        );
    }

    private getClusterDetail(dest: models.ApplicationDestination): string {
        const cluster = this.props.clusters.find(target => target.name === dest.name || target.server === dest.server);
        if (!cluster) {
            return dest.server || dest.name;
        }
        if (cluster.name === cluster.server) {
            return cluster.name;
        }
        return `${cluster.name} (${cluster.server})`;
    }
}
