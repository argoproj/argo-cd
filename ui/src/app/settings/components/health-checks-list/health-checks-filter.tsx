import * as React from 'react';
import * as models from '../../../shared/models';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';

export interface HealthChecksListPreferences {
    originFilter: string[];
}

export interface FilterResult {
    origin: boolean;
}

export interface FilteredHealthCheck extends models.HealthCheckItem {
    filterResult: FilterResult;
}

export class HealthChecksListPreferencesHelper {
    public static clearFilters(pref: HealthChecksListPreferences) {
        pref.originFilter = [];
    }
}

export function getHealthCheckFilterResults(items: models.HealthCheckItem[], pref: HealthChecksListPreferences): FilteredHealthCheck[] {
    return (items || []).map(item => ({
        ...item,
        filterResult: {
            origin: (pref.originFilter || []).length === 0 || pref.originFilter.includes(item.origin)
        }
    }));
}

export function filterHealthChecks(items: FilteredHealthCheck[]): models.HealthCheckItem[] {
    return items.filter(item => Object.values(item.filterResult).every(v => v));
}

interface HealthChecksFilterProps {
    items: FilteredHealthCheck[];
    pref: HealthChecksListPreferences;
    onChange: (newPref: HealthChecksListPreferences) => void;
    collapsed?: boolean;
}

const ORIGINS = [
    {label: 'BuiltinGo', display: 'Built-in Go'},
    {label: 'BuiltinLua', display: 'Built-in Lua'},
    {label: 'CustomLua', display: 'Custom Lua'},
    {label: 'OverrideLua', display: 'Override Lua'}
];

const OriginFilter = React.memo((props: HealthChecksFilterProps) => {
    const originOptions = React.useMemo(() => {
        const counts = new Map<string, number>();
        (props.items || []).forEach(item => {
            counts.set(item.origin, (counts.get(item.origin) || 0) + 1);
        });

        return ORIGINS.map(o => ({
            label: o.label,
            count: counts.get(o.label) || 0
        }));
    }, [props.items]);

    return (
        <Filter label='ORIGIN' selected={props.pref.originFilter || []} setSelected={selected => props.onChange({...props.pref, originFilter: selected})} options={originOptions} />
    );
});

export const HealthChecksFilter = (props: HealthChecksFilterProps) => {
    const appliedFilter = [...(props.pref.originFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        HealthChecksListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Health check filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <OriginFilter {...props} />
        </FiltersGroup>
    );
};
