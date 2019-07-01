import { Checkbox, DataLoader } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';

import {TagsInput} from '../../../shared/components';
import * as models from '../../../shared/models';
import { AppsListPreferences, services } from '../../../shared/services';

export interface ApplicationsFilterProps {
    applications: models.Application[];
    pref: AppsListPreferences;
    onChange: (pref: AppsListPreferences) => any;
}

const maxFilterItemsCount = 10;

class ItemsFilter extends React.Component<{
    items: { name: string, count: number }[];
    type: string;
    selected: string[],
    onChange: (selected: string[]) => any,
}, {
    expanded: boolean;
}> {
    constructor(props: any) {
        super(props);
        this.state = { expanded: false };
    }

    public render() {
        const unavailableSelected = this.props.selected.filter((selected) => !this.props.items.some((item) => item.name === selected));
        const items = this.props.items.sort((first, second) => second.count - first.count).concat(unavailableSelected.map((selected) => ({ name: selected, count: 0 })));
        return (
            <React.Fragment>
                <ul className={classNames('applications-list__filter', { 'applications-list__filter--expanded': this.state.expanded })}>
                    {items.map((item) => (
                        <li key={item.name}>
                            <div className='applications-list__filter-label'>
                                <Checkbox checked={this.props.selected.indexOf(item.name) > -1} id={`filter-${this.props.type}-${item.name}`} onChange={() => {
                                    const newSelected = this.props.selected.slice();
                                    const index = newSelected.indexOf(item.name);
                                    if (index > -1) {
                                        newSelected.splice(index, 1);
                                    } else {
                                        newSelected.push(item.name);
                                    }
                                    this.props.onChange(newSelected);
                                }}/> <label title={item.name} htmlFor={`filter-${this.props.type}-${item.name}`}>{item.name}</label>
                            </div> <span>{item.count}</span>
                        </li>
                    ))}
                </ul>
                {items.length > maxFilterItemsCount && (
                    <a onClick={() => this.setState({ expanded: !this.state.expanded })}>{this.state.expanded ? 'collapse' : 'expand'}</a>
                )}
            </React.Fragment>
        );
    }
}

export class ApplicationsFilter extends React.Component<ApplicationsFilterProps, { expanded: boolean }> {

    constructor(props: ApplicationsFilterProps) {
        super(props);
        this.state = { expanded: false };
    }

    public render() {
        const {applications, pref, onChange} = this.props;

        const sync = new Map<string, number>();
        Object.keys(models.SyncStatuses).forEach((key) => sync.set(models.SyncStatuses[key], 0));
        applications.filter((app) => app.status.sync.status).forEach((app) => sync.set(app.status.sync.status, (sync.get(app.status.sync.status) || 0) + 1));
        const health = new Map<string, number>();
        Object.keys(models.HealthStatuses).forEach((key) => health.set(models.HealthStatuses[key], 0));
        applications.filter((app) => app.status.health.status).forEach((app) => health.set(app.status.health.status, (health.get(app.status.health.status) || 0) + 1));
        return (
            <div className={classNames('applications-list__filters-container', { 'applications-list__filters-container--expanded': this.state.expanded })}>
            <i onClick={() => this.setState({ expanded: !this.state.expanded })}
                        className={classNames('fa applications-list__filters-expander', { 'fa-chevron-up': !this.state.expanded, 'fa-chevron-down': this.state.expanded })}/>
                <p className='applications-list__filters-container-title'>Filter By:</p>
                <div className='row'>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <p>Sync</p>
                        <ItemsFilter
                            selected={pref.syncFilter}
                            onChange={(selected) => onChange({...pref, syncFilter: selected})}
                            items={Array.from(sync.keys()).map((status) => ({name: status, count: sync.get(status) || 0 }))}
                            type='sync' />
                    </div>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <p>Health</p>
                        <ItemsFilter
                            selected={pref.healthFilter}
                            onChange={(selected) => onChange({...pref, healthFilter: selected})}
                            items={Array.from(health.keys()).map((status) => ({name: status, count: health.get(status) || 0 }))}
                            type='health' />
                    </div>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <p>Projects</p>
                        <DataLoader load={() => services.projects.list()}>
                        {(projects) => {
                            const projAppCount = new Map<string, number>();
                            projects.forEach((proj) => projAppCount.set(proj.metadata.name, 0));
                            applications.forEach((app) => projAppCount.set(app.spec.project, (projAppCount.get(app.spec.project) || 0) + 1));
                            return <ItemsFilter
                                selected={pref.projectsFilter}
                                onChange={(selected) => onChange({...pref, projectsFilter: selected})}
                                items={projects.map((proj) => ({name: proj.metadata.name, count: projAppCount.get(proj.metadata.name) || 0 }))}
                                type='projects' />;
                        }}
                        </DataLoader>
                    </div>
                    <div className='columns small-12 medium-3 xxlarge-12'>
                        <div className='applications-list__filter'>
                            <p>Clusters</p>
                            <ul>
                                <li>
                                    <TagsInput placeholder='https://kubernetes.default.svc'
                                        autocomplete={Array.from(new Set(applications.map((app) => app.spec.destination.server).filter((item) => !!item)))
                                            .filter((ns) => pref.clustersFilter.indexOf(ns) === -1)}
                                        tags={pref.clustersFilter}
                                        onChange={(selected) => onChange({...pref, clustersFilter: selected})}/>
                                </li>
                            </ul>
                            <p>Namespaces</p>
                            <ul>
                                <li>
                                    <TagsInput placeholder='*-us-west-*'
                                        autocomplete={Array.from(new Set(applications.map((app) => app.spec.destination.namespace).filter((item) => !!item)))
                                            .filter((ns) => pref.namespacesFilter.indexOf(ns) === -1)}
                                        tags={pref.namespacesFilter}
                                        onChange={(selected) => onChange({...pref, namespacesFilter: selected})}/>
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
        );
    }
}
