import * as React from 'react';
import * as models from '../../../shared/models';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';

export interface GpgKeysListPreferences {
    keyTypeFilter: string[];
}

export interface FilterResult {
    keyType: boolean;
}

export interface FilteredGpgKey extends models.GnuPGPublicKey {
    filterResult: FilterResult;
}

export class GpgKeysListPreferencesHelper {
    public static clearFilters(pref: GpgKeysListPreferences) {
        pref.keyTypeFilter = [];
    }
}

export function getGpgKeyFilterResults(gpgkeys: models.GnuPGPublicKey[], pref: GpgKeysListPreferences): FilteredGpgKey[] {
    return gpgkeys.map(gpgkey => ({
        ...gpgkey,
        filterResult: {
            keyType: pref.keyTypeFilter.length === 0 || (gpgkey.subType && pref.keyTypeFilter.map(f => f.toLowerCase()).includes(gpgkey.subType.toLowerCase()))
        }
    }));
}

export function filterGpgKeys(gpgkeys: FilteredGpgKey[]): models.GnuPGPublicKey[] {
    return gpgkeys.filter(gpgkey => Object.values(gpgkey.filterResult).every(v => v));
}

const getCounts = (gpgkeys: FilteredGpgKey[], filterType: keyof FilterResult, filter: (gpgkey: models.GnuPGPublicKey) => string | undefined, init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    gpgkeys.forEach(gpgkey => {
        if (Object.keys(gpgkey.filterResult).every((key: keyof FilterResult) => key === filterType || gpgkey.filterResult[key])) {
            const val = filter(gpgkey);
            if (val !== undefined) {
                map.set(val, (map.get(val) || 0) + 1);
            }
        }
    });
    return map;
};

interface GpgKeysFilterProps {
    gpgkeys: FilteredGpgKey[];
    pref: GpgKeysListPreferences;
    onChange: (newPref: GpgKeysListPreferences) => void;
    collapsed?: boolean;
}

const getKeyTypeIcon = () => <i className='fa fa-key' />;

const KeyTypeFilter = React.memo((props: GpgKeysFilterProps) => {
    const keyTypeOptions = React.useMemo(() => {
        const keyTypes = Array.from(new Set(props.gpgkeys.map(gpgkey => gpgkey.subType?.toLowerCase()).filter((type): type is string => !!type && type.trim() !== ''))).sort();
        const counts = getCounts(props.gpgkeys, 'keyType', gpgkey => gpgkey.subType?.toLowerCase(), keyTypes);
        return keyTypes.map(type => ({
            label: type.toUpperCase(),
            icon: getKeyTypeIcon(),
            count: counts.get(type)
        }));
    }, [props.gpgkeys]);

    return (
        <Filter
            label='KEY TYPE'
            selected={props.pref.keyTypeFilter.map(s => s.toUpperCase())}
            setSelected={s => props.onChange({...props.pref, keyTypeFilter: s.map(v => v.toLowerCase())})}
            options={keyTypeOptions}
        />
    );
});

export const GpgKeysFilter = (props: GpgKeysFilterProps) => {
    const appliedFilter = [...(props.pref.keyTypeFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        GpgKeysListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='GPG key filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <KeyTypeFilter {...props} />
        </FiltersGroup>
    );
};
