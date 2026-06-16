import {DropDownMenu, MenuItem} from 'argo-ui';
import * as React from 'react';

export interface ActionMenuProps {
    items: MenuItem[];
    qeId?: string;
}

// Shared "..." anchor for table action menus. The onMouseDown handler closes any other open dropdown
// before this one opens: argo-ui's DropDown never broadcasts on open and its anchor stops click
// propagation, so clicking another anchor would otherwise leave the previous menu open. Triggering a
// body click first dispatches the outside-click that closes them.
//
// Use it as the `anchor` of a DropDownMenu (static items, see ActionMenu) or a DropDown (arbitrary
// render-prop content, e.g. the application resource list node menu).
export const ActionMenuButton = () => (
    <button className='argo-button argo-button--light argo-button--lg argo-button--short' onMouseDown={() => document.body.click()}>
        <i className='fa fa-ellipsis-v' />
    </button>
);

// Standard "..." action menu used in the settings table lists, backed by a static list of items.
export const ActionMenu = (props: ActionMenuProps) => <DropDownMenu qeId={props.qeId} anchor={ActionMenuButton} items={props.items} />;
