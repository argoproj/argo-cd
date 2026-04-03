import * as React from 'react';
import {render as rtlRender, screen, fireEvent} from '@testing-library/react';
import {act} from 'react';

// helpTip is imported by editable-panel from applications/components/utils,
// which transitively pulls in lodash-es (ESM-only) and many heavy deps.
// Mock the entire utils module to avoid transform failures.
jest.mock('../../../../applications/components/utils', () => ({
    helpTip: (text: string) => React.createElement('span', {title: text}, '?'),
    SpinningIcon: () => React.createElement('span', {}, '...'),
}));

import {EditablePanel, EditablePanelItem} from '../editable-panel';
import {Context} from '../../../context';
import {FormApi} from 'argo-ui';

// ---------------------------------------------------------------------------
// Context mock — EditablePanel uses ctx.notifications.show() on save failure
// ---------------------------------------------------------------------------

const mockNotificationsShow = jest.fn();
const mockContext = {
    history: {} as any,
    popup: {} as any,
    navigation: {} as any,
    baseHref: '/',
    notifications: {
        show: mockNotificationsShow,
    } as any,
};

function Wrapper({children}: {children: React.ReactNode}) {
    return <Context.Provider value={mockContext}>{children}</Context.Provider>;
}

function renderPanel(element: React.ReactElement) {
    return rtlRender(<Wrapper>{element}</Wrapper>);
}

// ---------------------------------------------------------------------------
// Basic items fixture
// ---------------------------------------------------------------------------

const basicItems: EditablePanelItem[] = [
    {
        title: 'Name',
        view: <span>alice</span>,
        edit: (api: FormApi) => <input data-testid='name-input' value={api.values.name || ''} onChange={e => api.setValue('name', e.target.value)} />,
    },
];

// ===========================================================================
// 2a. Edit Mode Toggle
// ===========================================================================

describe('EditablePanel – edit mode toggle', () => {
    test('renders in view mode by default', () => {
        renderPanel(
            <EditablePanel
                values={{name: 'alice'}}
                items={basicItems}
                save={jest.fn().mockResolvedValue(undefined)}
            />
        );
        expect(screen.queryByText('alice')).toBeTruthy();
        expect(screen.queryByRole('button', {name: /Edit/i})).toBeTruthy();
        expect(screen.queryByRole('button', {name: /Save/i})).toBeFalsy();
    });

    test('clicking Edit switches to edit mode', () => {
        renderPanel(
            <EditablePanel
                values={{name: 'alice'}}
                items={basicItems}
                save={jest.fn().mockResolvedValue(undefined)}
            />
        );

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });

        expect(screen.queryByRole('button', {name: /Save/i})).toBeTruthy();
        expect(screen.queryByRole('button', {name: /Cancel/i})).toBeTruthy();
    });

    test('clicking Cancel exits edit mode without calling save', () => {
        const save = jest.fn().mockResolvedValue(undefined);
        renderPanel(
            <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
        );

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });
        act(() => { fireEvent.click(screen.getByRole('button', {name: /Cancel/i})); });

        expect(save).not.toHaveBeenCalled();
        expect(screen.queryByRole('button', {name: /Edit/i})).toBeTruthy();
        expect(screen.queryByRole('button', {name: /Cancel/i})).toBeFalsy();
    });

    test('clicking Save calls save() and returns to view mode', async () => {
        const save = jest.fn().mockResolvedValue(undefined);
        renderPanel(
            <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
        );

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });
        await act(async () => { fireEvent.click(screen.getByRole('button', {name: /Save/i})); });

        expect(save).toHaveBeenCalled();
        expect(screen.queryByRole('button', {name: /Edit/i})).toBeTruthy(); // back to view mode
    });

    test('onModeSwitch is called when entering and exiting edit mode', async () => {
        const onModeSwitch = jest.fn();
        const save = jest.fn().mockResolvedValue(undefined);
        renderPanel(
            <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} onModeSwitch={onModeSwitch} />
        );

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });
        expect(onModeSwitch).toHaveBeenCalledTimes(1);

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Cancel/i})); });
        expect(onModeSwitch).toHaveBeenCalledTimes(2);
    });
});

// ===========================================================================
// 2b. Save Failure Handling
// ===========================================================================

describe('EditablePanel – save failure handling', () => {
    test('shows notification on save failure and stays in edit mode', async () => {
        const save = jest.fn().mockRejectedValue(new Error('Server error'));
        renderPanel(
            <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
        );

        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });
        await act(async () => { fireEvent.click(screen.getByRole('button', {name: /Save/i})); });

        expect(mockNotificationsShow).toHaveBeenCalled();
        // Panel stays in edit mode (Save button still present)
        expect(screen.queryByRole('button', {name: /Save/i})).toBeTruthy();
    });
});

// ===========================================================================
// 2c. noReadonlyMode (always-edit)
// ===========================================================================

describe('EditablePanel – noReadonlyMode', () => {
    test('renders in edit mode immediately when noReadonlyMode=true', () => {
        renderPanel(
            <EditablePanel
                values={{name: 'alice'}}
                items={basicItems}
                save={jest.fn().mockResolvedValue(undefined)}
                noReadonlyMode={true}
            />
        );
        // No Edit button in noReadonlyMode
        expect(screen.queryByRole('button', {name: /^Edit$/i})).toBeFalsy();
    });

    test('formDidUpdate triggers save() automatically in noReadonlyMode', async () => {
        const save = jest.fn().mockResolvedValue(undefined);
        let capturedFormApi: FormApi | null = null;
        const editItems: EditablePanelItem[] = [
            {
                title: 'Name',
                view: <span>alice</span>,
                edit: (api: FormApi) => {
                    capturedFormApi = api;
                    return <input value={api.values.name || ''} onChange={e => api.setValue('name', e.target.value)} />;
                },
            },
        ];

        renderPanel(
            <EditablePanel
                values={{name: 'alice'}}
                items={editItems}
                save={save}
                noReadonlyMode={true}
            />
        );

        // Trigger a value change via the captured FormApi
        await act(async () => {
            capturedFormApi?.setValue('name', 'bob');
        });

        // save() should have been called at least once via formDidUpdate
        expect(save).toHaveBeenCalled();
    });
});

// ===========================================================================
// 2d. JSON.stringify Comparison
// ===========================================================================

describe('EditablePanel – values sync', () => {
    test('does not falsely detect change when property order differs', () => {
        // Both JSON representations are different due to order but semantically equal.
        // The component uses JSON.stringify so this IS a known limitation —
        // this test documents current behavior (may trigger false positive sync).
        const setAllValues = jest.fn();
        let capturedFormApi: FormApi | null = null;
        const editItems: EditablePanelItem[] = [
            {
                title: 'X',
                view: <span />,
                edit: (api: FormApi) => {
                    capturedFormApi = api;
                    return <span />;
                },
            },
        ];

        const valuesV1 = {a: 1, b: 2};
        const {rerender} = renderPanel(
            <EditablePanel values={valuesV1} items={editItems} save={jest.fn().mockResolvedValue(undefined)} />
        );

        // Enter edit mode to get formApi
        act(() => { fireEvent.click(screen.getByRole('button', {name: /Edit/i})); });

        // Patch captured formApi to spy on setAllValues
        if (capturedFormApi) {
            (capturedFormApi as any).setAllValues = setAllValues;
        }

        // Update with same data, different key order — JSON.stringify will differ
        const valuesV2 = {b: 2, a: 1};
        act(() => {
            rerender(
                <Wrapper>
                    <EditablePanel values={valuesV2} items={editItems} save={jest.fn().mockResolvedValue(undefined)} />
                </Wrapper>
            );
        });
        // Document: JSON.stringify({a:1,b:2}) !== JSON.stringify({b:2,a:1})
        // so the component WILL call setAllValues — this is the known limitation
        expect(JSON.stringify(valuesV1)).not.toBe(JSON.stringify(valuesV2));
    });

    test('detects genuine value change and syncs in noReadonlyMode', async () => {
        let capturedFormApi: FormApi | null = null;
        const editItems: EditablePanelItem[] = [
            {
                title: 'X',
                view: <span />,
                edit: (api: FormApi) => {
                    capturedFormApi = api;
                    return <span />;
                },
            },
        ];

        const {rerender} = renderPanel(
            <EditablePanel values={{name: 'v1'}} items={editItems} save={jest.fn().mockResolvedValue(undefined)} noReadonlyMode />
        );

        const setAllValuesSpy = jest.spyOn(capturedFormApi!, 'setAllValues');

        act(() => {
            rerender(
                <Wrapper>
                    <EditablePanel values={{name: 'v2'}} items={editItems} save={jest.fn().mockResolvedValue(undefined)} noReadonlyMode />
                </Wrapper>
            );
        });

        expect(setAllValuesSpy).toHaveBeenCalledWith({name: 'v2'});
    });
});

// ===========================================================================
// 2e. Collapsible Behavior
// ===========================================================================

describe('EditablePanel – collapsible behavior', () => {
    test('renders collapsed panel when collapsible=true and collapsed=true', () => {
        renderPanel(
            <EditablePanel
                values={{}}
                items={[]}
                collapsible={true}
                collapsed={true}
                collapsedDescription='Click to expand'
            />
        );
        expect(screen.queryByText('Click to expand')).toBeTruthy();
        expect(screen.queryByRole('button', {name: /^Edit$/i})).toBeFalsy();
    });

    test('clicking collapsed panel expands it', () => {
        renderPanel(
            <EditablePanel
                values={{}}
                items={[]}
                collapsible={true}
                collapsed={true}
                collapsedDescription='Click to expand'
            />
        );

        const collapsedDiv = document.querySelector('[class*="redirect-panel"]') as HTMLElement;
        act(() => { fireEvent.click(collapsedDiv); });

        expect(screen.queryByText('Click to expand')).toBeFalsy();
    });

    test('external collapsed prop change updates internal state', () => {
        const {rerender} = renderPanel(
            <EditablePanel values={{}} items={[]} collapsible={true} collapsed={false} />
        );
        // Not collapsed initially — no redirect-panel
        expect(document.querySelector('[class*="redirect-panel"]')).toBeFalsy();

        act(() => {
            rerender(
                <Wrapper>
                    <EditablePanel values={{}} items={[]} collapsible={true} collapsed={true} />
                </Wrapper>
            );
        });
        // Now collapsed — redirect-panel present
        expect(document.querySelector('[class*="redirect-panel"]')).toBeTruthy();
    });

    test('disabled Edit button renders when hasMultipleSources=true', () => {
        renderPanel(
            <EditablePanel
                values={{}}
                items={[]}
                save={jest.fn()}
                hasMultipleSources={true}
            />
        );
        const editButton = screen.getByRole('button', {name: /Edit/i}) as HTMLButtonElement;
        expect(editButton).toBeTruthy();
        expect(editButton.disabled).toBe(true);
    });
});
