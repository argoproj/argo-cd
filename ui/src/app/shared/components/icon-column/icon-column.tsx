import * as React from 'react';

export interface IconColumnProps {
    // Full icon class, e.g. 'argo-icon-git' or 'fa fa-object-group'. Omit for the (empty) header cell.
    icon?: string;
}

// Leading fixed-width table column that holds a single list-item icon. The shared
// `.argo-table-list__icon-column` styles size and center the glyph consistently across lists.
export const IconColumn = (props: IconColumnProps) => (
    <div className='columns small-1 argo-table-list__icon-column'>{props.icon && <i className={`icon ${props.icon}`} />}</div>
);
