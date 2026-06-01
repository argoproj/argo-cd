import * as React from 'react';
const PieChart = require('react-svg-piechart').default;

import {COLORS} from '../../../shared/components';
import {
    ARGO_FAILED_COLOR,
    ARGO_GRAY4_COLOR,
    ARGO_RUNNING_COLOR,
    ARGO_SUCCESS_COLOR,
    ARGO_SUSPENDED_COLOR,
    ARGO_TERMINATING_COLOR,
    ARGO_WARNING_COLOR
} from '../../../shared/components/colors';
import * as models from '../../../shared/models';
import {HealthStatusCode, SyncStatusCode} from '../../../shared/models';
import {ComparisonStatusIcon, HealthStatusIcon} from '../../../applications/components/utils';

const healthColors = new Map<models.HealthStatusCode, string>();
healthColors.set('Unknown', COLORS.health.unknown);
healthColors.set('Progressing', COLORS.health.progressing);
healthColors.set('Suspended', COLORS.health.suspended);
healthColors.set('Healthy', COLORS.health.healthy);
healthColors.set('Degraded', COLORS.health.degraded);
healthColors.set('Missing', COLORS.health.missing);

const syncColors = new Map<models.SyncStatusCode, string>();
syncColors.set('Unknown', COLORS.sync.unknown);
syncColors.set('Synced', COLORS.sync.synced);
syncColors.set('OutOfSync', COLORS.sync.out_of_sync);

// Kind/Cluster pie slices only — no grey; grey is reserved for the "Others" bucket.
// Must have at least MAX_PIE_SLICES - 1 entries so displayed slices never share a color.
const CATEGORY_PIE_COLORS = [
    ARGO_RUNNING_COLOR,
    ARGO_SUCCESS_COLOR,
    ARGO_WARNING_COLOR,
    ARGO_FAILED_COLOR,
    ARGO_SUSPENDED_COLOR,
    ARGO_TERMINATING_COLOR,
    '#DE7EAE',
    '#FF9500',
    '#4B0082'
];

const MAX_PIE_SLICES = 10;
const OTHERS_LABEL = 'Others';
const OTHERS_COLOR = ARGO_GRAY4_COLOR;

type PieSlice = {title: string; value: number; color: string};

type SummaryChart = {
    title: string;
    type: 'health' | 'sync' | 'category';
    data: PieSlice[];
    legend: Map<string, string>;
};

function limitPieSlices(slices: PieSlice[]): PieSlice[] {
    const sorted = [...slices].filter(slice => slice.value > 0).sort((a, b) => b.value - a.value || a.title.localeCompare(b.title));
    if (sorted.length <= MAX_PIE_SLICES) {
        return sorted;
    }
    const top = sorted.slice(0, MAX_PIE_SLICES - 1);
    const othersValue = sorted.slice(MAX_PIE_SLICES - 1).reduce((sum, slice) => sum + slice.value, 0);
    return [...top, {title: OTHERS_LABEL, value: othersValue, color: OTHERS_COLOR}];
}

function assignCategoryColors(slices: PieSlice[]): PieSlice[] {
    let colorIndex = 0;
    return slices.map(slice => {
        if (slice.title === OTHERS_LABEL) {
            return {...slice, color: OTHERS_COLOR};
        }
        return {...slice, color: CATEGORY_PIE_COLORS[colorIndex++]};
    });
}

function legendFromSlices(slices: PieSlice[], baseLegend?: Map<string, string>): Map<string, string> {
    const legend = new Map(baseLegend);
    slices.forEach(slice => legend.set(slice.title, slice.color));
    return legend;
}

function countBy(resources: models.Resource[], getKey: (resource: models.Resource) => string): Map<string, number> {
    const counts = new Map<string, number>();
    resources.forEach(resource => {
        const key = getKey(resource) || 'Unknown';
        counts.set(key, (counts.get(key) || 0) + 1);
    });
    return counts;
}

function buildCategoryChart(title: string, counts: Map<string, number>): SummaryChart {
    const keys = Array.from(counts.keys()).sort((a, b) => {
        const byCount = (counts.get(b) || 0) - (counts.get(a) || 0);
        return byCount !== 0 ? byCount : a.localeCompare(b);
    });
    const slices = keys.map(key => ({
        title: key,
        value: counts.get(key) || 0,
        color: ''
    }));
    const data = assignCategoryColors(limitPieSlices(slices));
    return {title, type: 'category', data, legend: legendFromSlices(data)};
}

function buildStatusChart(title: string, type: 'sync' | 'health', counts: Map<string, number>, statusColors: Map<string, string>): SummaryChart {
    const slices = Array.from(counts.keys()).map(key => ({
        title: key,
        value: counts.get(key) || 0,
        color: statusColors.get(key) || statusColors.get('Unknown') || OTHERS_COLOR
    }));
    const data = limitPieSlices(slices);
    return {title, type, data, legend: legendFromSlices(data, statusColors)};
}

function clusterLabel(resource: models.Resource): string {
    return resource.clusterName || resource.clusterServer || 'Unknown';
}

export const ResourcesSummary = ({resources}: {resources: models.Resource[]}) => {
    const sync = countBy(resources, resource => (resource.status as SyncStatusCode) || 'Unknown');
    const health = countBy(resources, resource => resource.health?.status || 'Unknown');
    const kind = countBy(resources, resource => resource.kind || 'Unknown');
    const cluster = countBy(resources, clusterLabel);

    const attributes = [
        {title: 'RESOURCES', value: resources.length},
        {title: 'SYNCED', value: resources.filter(resource => resource.status === 'Synced').length},
        {title: 'HEALTHY', value: resources.filter(resource => resource.health?.status === 'Healthy').length},
        {
            title: 'APPLICATIONS',
            value: new Set(resources.map(resource => `${resource.appNamespace}/${resource.appName}`)).size
        },
        {title: 'CLUSTERS', value: new Set(resources.map(clusterLabel).filter(label => label !== 'Unknown')).size},
        {title: 'KINDS', value: kind.size},
        {title: 'NAMESPACES', value: new Set(resources.map(resource => resource.namespace).filter(Boolean)).size}
    ];

    const charts: SummaryChart[] = [
        buildStatusChart('Sync', 'sync', sync, syncColors as Map<string, string>),
        buildStatusChart('Health', 'health', health, healthColors as Map<string, string>),
        buildCategoryChart('Kind', kind),
        buildCategoryChart('Cluster', cluster)
    ];

    return (
        <div className='white-box resources-list__summary'>
            <div className='row'>
                <div className='columns large-3 small-12'>
                    <div className='white-box__details'>
                        <p className='row'>SUMMARY</p>
                        {attributes.map(attr => (
                            <div className='row white-box__details-row' key={attr.title}>
                                <div className='columns small-8'>{attr.title}</div>
                                <div style={{textAlign: 'right'}} className='columns small-4'>
                                    {attr.value}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
                <div className='columns large-9 small-12'>
                    <div className='row chart-group'>
                        {charts.map(chart => (
                            <div className='columns large-6 small-12' key={chart.title}>
                                <div className='row chart'>
                                    <div className='large-8 small-6'>
                                        <h4 style={{textAlign: 'center'}}>{chart.title}</h4>
                                        <PieChart data={chart.data} />
                                    </div>
                                    <div className='large-3 small-1'>
                                        <ul>
                                            {chart.data.map(slice => (
                                                <li style={{listStyle: 'none', whiteSpace: 'nowrap'}} key={slice.title}>
                                                    {chart.type === 'health' && slice.title !== OTHERS_LABEL && (
                                                        <HealthStatusIcon state={{status: slice.title as HealthStatusCode, message: ''}} noSpin={true} />
                                                    )}
                                                    {chart.type === 'sync' && slice.title !== OTHERS_LABEL && (
                                                        <ComparisonStatusIcon status={slice.title as SyncStatusCode} noSpin={true} />
                                                    )}
                                                    {(chart.type === 'category' || slice.title === OTHERS_LABEL) && (
                                                        <span style={{color: slice.color}}>
                                                            <i className='fa fa-circle' style={{fontSize: '0.65em', verticalAlign: 'middle'}} />{' '}
                                                        </span>
                                                    )}
                                                    {`${slice.title} (${slice.value})`}
                                                </li>
                                            ))}
                                        </ul>
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
};
