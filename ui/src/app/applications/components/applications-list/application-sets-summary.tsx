import * as React from 'react';
const PieChart = require('react-svg-piechart').default;

import {COLORS} from '../../../shared/components';
import * as models from '../../../shared/models';
import {HealthStatusCode} from '../../../shared/models';
import {HealthStatusIcon, getAppSetHealthStatus} from '../utils';

const appSetHealthColors = new Map<models.HealthStatusCode, string>();
appSetHealthColors.set('Unknown', COLORS.health.unknown);
appSetHealthColors.set('Healthy', COLORS.health.healthy);
appSetHealthColors.set('Degraded', COLORS.health.degraded);
appSetHealthColors.set('Progressing', COLORS.health.progressing);

const handleKeyDown = (e: React.KeyboardEvent, action: () => void) => {
    if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        action();
    }
};

export const ApplicationSetsSummary = ({appSets, onFilterClick}: {appSets: models.ApplicationSet[]; onFilterClick?: (type: 'Health', value: string) => void}) => {
    const [hoveredSector, setHoveredSector] = React.useState<{chartTitle: string; title: string} | null>(null);

    const health = new Map<string, number>();

    appSets.forEach(appSet => {
        const status = getAppSetHealthStatus(appSet);
        health.set(status, (health.get(status) || 0) + 1);
    });

    const attributes = [
        {title: 'APPLICATIONSETS', value: appSets.length},
        {title: 'HEALTHY', value: appSets.filter(appSet => getAppSetHealthStatus(appSet) === 'Healthy').length}
    ];

    const charts = [
        {
            title: 'Health',
            data: Array.from(health.keys()).map(key => ({title: key, value: health.get(key), color: appSetHealthColors.get(key as models.HealthStatusCode)})),
            legend: appSetHealthColors as Map<string, string>
        }
    ];

    return (
        <div className='white-box applications-list__summary'>
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
                        {charts.map(chart => {
                            const getLegendValue = (key: string) => {
                                const index = chart.data.findIndex((data: {title: string}) => data.title === key);
                                return index > -1 ? chart.data[index].value : 0;
                            };
                            return (
                                <React.Fragment key={chart.title}>
                                    <div className='columns large-6 small-12'>
                                        <div className='row chart'>
                                            <div className='large-8 small-6'>
                                                <h4 style={{textAlign: 'center'}}>{chart.title}</h4>
                                                <div
                                                    onClick={() => {
                                                        if (onFilterClick && hoveredSector && hoveredSector.chartTitle === chart.title) {
                                                            onFilterClick(chart.title as 'Health', hoveredSector.title);
                                                        }
                                                    }}
                                                    onKeyDown={e => {
                                                        if (onFilterClick && hoveredSector && hoveredSector.chartTitle === chart.title) {
                                                            handleKeyDown(e, () => onFilterClick(chart.title as 'Health', hoveredSector.title));
                                                        }
                                                    }}
                                                    style={{cursor: 'pointer'}}
                                                    title='Click to filter application sets'
                                                    role='button'
                                                    tabIndex={0}>
                                                    <PieChart
                                                        data={chart.data}
                                                        onSectorHover={(d: any) => setHoveredSector(d ? {chartTitle: chart.title, title: d.title} : null)}
                                                        expandOnHover={true}
                                                    />
                                                </div>
                                            </div>
                                            <div className='large-3 small-1'>
                                                <ul>
                                                    {Array.from(chart.legend.keys()).map(key => (
                                                        <li style={{listStyle: 'none', whiteSpace: 'nowrap'}} key={key}>
                                                            <button
                                                                type='button'
                                                                onClick={() => {
                                                                    if (onFilterClick) {
                                                                        onFilterClick(chart.title as 'Health', key);
                                                                    }
                                                                }}
                                                                title={`Filter by ${key}`}
                                                                style={{
                                                                    background: 'none',
                                                                    border: 'none',
                                                                    padding: 0,
                                                                    cursor: 'pointer',
                                                                    textAlign: 'left',
                                                                    font: 'inherit',
                                                                    color: 'inherit'
                                                                }}>
                                                                <HealthStatusIcon state={{status: key as HealthStatusCode, message: ''}} noSpin={true} />
                                                                {` ${key} (${getLegendValue(key)})`}
                                                            </button>
                                                        </li>
                                                    ))}
                                                </ul>
                                            </div>
                                        </div>
                                    </div>
                                </React.Fragment>
                            );
                        })}
                    </div>
                </div>
            </div>
        </div>
    );
};
