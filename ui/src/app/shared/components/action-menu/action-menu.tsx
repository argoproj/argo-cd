import {DropDownMenu, MenuItem} from 'argo-ui';
import * as React from 'react';

export interface ActionMenuProps {
    items: MenuItem[];
    qeId?: string;
}

// Standard "..." action menu used in the settings table lists. The onMouseDown handler closes any
// other open dropdown before this one opens: argo-ui's DropDown never broadcasts on open and its
// anchor stops click propagation, so clicking another anchor would otherwise leave the previous
// menu open. Triggering a body click first dispatches the outside-click that closes them.
export const ActionMenu = (props: ActionMenuProps) => (
    <DropDownMenu
        qeId={props.qeId}
        anchor={() => (
            <button className='argo-button argo-button--light argo-button--lg argo-button--short' onMouseDown={() => document.body.click()}>
                <i className='fa fa-ellipsis-v' />
            </button>
        )}
        items={props.items}
    />
);
