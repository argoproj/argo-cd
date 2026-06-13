import * as React from 'react';
import * as models from '../../../shared/models';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';

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

const getCounts = (accounts: FilteredAccount[], filterType: keyof FilterResult, filter: (account: models.Account) => string | string[], init?: string[]) => {
    const map = new Map<string, number>();
    if (init) {
        init.forEach(key => map.set(key, 0));
    }
    accounts.forEach(account => {
        if (Object.keys(account.filterResult).every((key: keyof FilterResult) => key === filterType || account.filterResult[key])) {
            const val = filter(account);
            if (Array.isArray(val)) {
                val.forEach(v => map.set(v, (map.get(v) || 0) + 1));
            } else if (val !== undefined) {
                map.set(val, (map.get(val) || 0) + 1);
            }
        }
    });
    return map;
};

const getOptions = (
    accounts: FilteredAccount[],
    filterType: keyof FilterResult,
    filter: (account: models.Account) => string | string[],
    keys: string[],
    getIcon?: (k: string) => React.ReactNode
) => {
    const counts = getCounts(accounts, filterType, filter, keys);
    return keys.map(k => ({
        label: k.charAt(0).toUpperCase() + k.slice(1),
        icon: getIcon && getIcon(k),
        count: counts.get(k)
    }));
};

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
        options={getOptions(props.accounts, 'status', getAccountStatus, ['enabled', 'disabled'])}
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
        const counts = getCounts(props.accounts, 'capabilities', account => account.capabilities || [], allCapabilities);
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
