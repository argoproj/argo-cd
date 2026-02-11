import * as React from 'react';
import {resourceIcons} from './resources';
import {resourceIconGroups as resourceCustomizations} from './resource-customizations';
import * as minimatch from 'minimatch';

export const ResourceIcon = ({group, kind, customStyle}: {group: string; kind: string; customStyle?: React.CSSProperties}) => {
    if (kind === 'node') {
        return <img src={'assets/images/infrastructure_components/' + kind + '.svg'} alt={kind} style={{padding: '2px', width: '40px', height: '32px', ...customStyle}} />;
    }
    if (kind === 'Application') {
        return <i title={kind} className={`icon argo-icon-application`} style={customStyle} />;
    }
    // First, check for group-based custom icons
    if (group) {
        const matchedGroup = matchGroupToResource(group);
        if (matchedGroup) {
            return <img src={`assets/images/resources/${matchedGroup}/icon.svg`} alt={kind} style={{paddingBottom: '2px', width: '40px', height: '32px', ...customStyle}} />;
        }
    }
    // Fallback to kind-based icons (works for both empty group and non-matching groups)
    const i = resourceIcons.get(kind);
    if (i !== undefined) {
        return <img src={'assets/images/resources/' + i + '.svg'} alt={kind} style={{padding: '2px', width: '40px', height: '32px', ...customStyle}} />;
    }
    const initials = kind.replace(/[a-z]/g, '');
    const n = initials.length;
    const style: React.CSSProperties = {
        display: 'inline-block',
        verticalAlign: 'middle',
        padding: `${n <= 2 ? 2 : 0}px 4px`,
        width: '32px',
        height: '32px',
        borderRadius: '50%',
        backgroundColor: '#8FA4B1',
        textAlign: 'center',
        lineHeight: '30px',
        ...customStyle
    };
    return (
        <div style={style}>
            <span style={{color: 'white', fontSize: `${n <= 2 ? 1 : 0.6}em`}}>{initials}</span>
        </div>
    );
};

// Utility function to match group with possible wildcards in resourceCustomizations. If found, returns the matched key
// as a path component (with '*' replaced by '_' if necessary), otherwise returns an empty string.
function matchGroupToResource(group: string): string {
    // Check for an exact match
    if (group in resourceCustomizations) {
        return group;
    }

    // Loop over the map keys to find a match using minimatch
    for (const key in resourceCustomizations) {
        if (key.includes('*') && minimatch(group, key)) {
            return key.replace(/\*/g, '_');
        }
    }

    // Return an empty string if no match is found
    return '';
}
