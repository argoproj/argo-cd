import * as React from 'react';
import * as models from '../../../shared/models';
import {Filter, FiltersGroup} from '../../../applications/components/filter/filter';
import {getFilterOptions} from '../list-filter-utils';

export interface CertsListPreferences {
    certTypeFilter: string[];
}

export interface FilterResult {
    certType: boolean;
}

export interface FilteredCert extends models.RepoCert {
    filterResult: FilterResult;
}

export class CertsListPreferencesHelper {
    public static clearFilters(pref: CertsListPreferences) {
        pref.certTypeFilter = [];
    }
}

export function getCertFilterResults(certs: models.RepoCert[], pref: CertsListPreferences): FilteredCert[] {
    return certs.map(cert => ({
        ...cert,
        filterResult: {
            certType: pref.certTypeFilter.length === 0 || pref.certTypeFilter.includes(cert.certType)
        }
    }));
}

export function filterCerts(certs: FilteredCert[]): models.RepoCert[] {
    return certs.filter(cert => Object.values(cert.filterResult).every(v => v));
}

interface CertsFilterProps {
    certs: FilteredCert[];
    pref: CertsListPreferences;
    onChange: (newPref: CertsListPreferences) => void;
    collapsed?: boolean;
}

const getCertTypeIcon = (certType: string) => {
    switch (certType) {
        case 'https':
            return <i className='fa fa-lock' />;
        case 'ssh':
            return <i className='fa fa-key' />;
        default:
            return null;
    }
};

const getCertTypeLabel = (certType: string) => {
    switch (certType) {
        case 'https':
            return 'TLS Certificate';
        case 'ssh':
            return 'SSH Known Host';
        default:
            return certType.charAt(0).toUpperCase() + certType.slice(1);
    }
};

const getCertTypeValue = (label: string) => {
    switch (label) {
        case 'TLS Certificate':
            return 'https';
        case 'SSH Known Host':
            return 'ssh';
        default:
            return label.toLowerCase();
    }
};

const CertTypeFilter = (props: CertsFilterProps) => (
    <Filter
        label='CERT TYPE'
        selected={props.pref.certTypeFilter.map(getCertTypeLabel)}
        setSelected={s => props.onChange({...props.pref, certTypeFilter: s.map(getCertTypeValue)})}
        options={getFilterOptions(props.certs, 'certType', cert => cert.certType, ['https', 'ssh'], getCertTypeIcon, getCertTypeLabel)}
    />
);

export const CertsFilter = (props: CertsFilterProps) => {
    const appliedFilter = [...(props.pref.certTypeFilter || [])];

    const onClearFilter = () => {
        const newPref = {...props.pref};
        CertsListPreferencesHelper.clearFilters(newPref);
        props.onChange(newPref);
    };

    return (
        <FiltersGroup title='Certificate filters' content={null} appliedFilter={appliedFilter} onClearFilter={onClearFilter} collapsed={props.collapsed}>
            <CertTypeFilter {...props} />
        </FiltersGroup>
    );
};
