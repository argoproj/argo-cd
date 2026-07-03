import * as React from 'react';
import * as models from '../../../shared/models';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';
import {capitalize, getFilterCounts, getFilterOptions} from '../list-filter-utils';

export interface AccountsListPreferences {
    statusFilter: string[];
    capabilitiesFilter: string[];
}

export interface FilterResult {
    status: boolean;
    capabilities: boolean;
}

export interface FilteredAccount extends models.Account {
    filterResult: FilterResult;
}

export class AccountsListPreferencesHelper {
    public static clearFilters(pref: AccountsListPreferences) {
        pref.statusFilter = [];
        pref.capabilitiesFilter = [];
    }
}

export function getAccountStatus(account: models.Account): string {
    return account.enabled ? 'enabled' : 'disabled';
}

export function getAccountFilterResults(accounts: models.Account[], pref: AccountsListPreferences): FilteredAccount[] {
    return accounts.map(account => ({
        ...account,
        filterResult: {
            status: pref.statusFilter.length === 0 || pref.statusFilter.includes(getAccountStatus(account)),
            capabilities: pref.capabilitiesFilter.length === 0 || (account.capabilities && pref.capabilitiesFilter.every(cap => account.capabilities.includes(cap)))
        }
    }));
}

export function filterAccounts(accounts: FilteredAccount[]): models.Account[] {
    return accounts.filter(account => Object.values(account.filterResult).every(v => v));
}

interface AccountsFilterProps {
    accounts: FilteredAccount[];
    pref: AccountsListPreferences;
    onChange: (newPref: AccountsListPreferences) => void;
    collapsed?: boolean;
}

const StatusFilter = (props: AccountsFilterProps) => (
    <Filter
        label='STATUS'
        selected={props.pref.statusFilter.map(s => s.charAt(0).toUpperCase() + s.slice(1))}
        setSelected={s => props.onChange({...props.pref, statusFilter: s.map(v => v.toLowerCase())})}
        options={getFilterOptions(props.accounts, 'status', getAccountStatus, ['enabled', 'disabled'], undefined, capitalize)}
        radio={true}
    />
);

const formatCapabilityLabel = (capability: string): string => {
    // Handle camelCase like "apiKey" -> "Api Key"
    return capability
        .replace(/([A-Z])/g, ' $1')
        .trim()
        .split(' ')
        .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
        .join(' ');
};

const normalizeCapability = (label: string): string => {
    // Convert "Api Key" back to "apiKey"
    const words = label.split(' ');
    if (words.length === 1) {
        return label.toLowerCase();
    }
    return (
        words[0].toLowerCase() +
        words
            .slice(1)
            .map(w => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
            .join('')
    );
};

const CapabilitiesFilter = React.memo((props: AccountsFilterProps) => {
    const capabilitiesOptions = React.useMemo(() => {
        const allCapabilities = Array.from(new Set(props.accounts.flatMap(account => account.capabilities || []).filter(cap => cap && cap.trim() !== ''))).sort();
        const counts = getFilterCounts(props.accounts, 'capabilities', account => account.capabilities || [], allCapabilities);
        return allCapabilities.map(cap => ({
            label: formatCapabilityLabel(cap),
            count: counts.get(cap)
        }));
    }, [props.accounts]);

    return (
        <Filter
            label='CAPABILITIES'
            selected={props.pref.capabilitiesFilter.map(formatCapabilityLabel)}
            setSelected={s => props.onChange({...props.pref, capabilitiesFilter: s.map(normalizeCapability)})}
            options={capabilitiesOptions}
        />
    );
});

export const AccountsFilter = (props: AccountsFilterProps) => {
    const appliedFilter = [...(props.pref.statusFilter || []), ...(props.pref.capabilitiesFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        AccountsListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Account filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <StatusFilter {...props} />
            <CapabilitiesFilter {...props} />
        </FiltersGroup>
    );
};
