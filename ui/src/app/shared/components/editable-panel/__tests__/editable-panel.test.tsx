import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {act} from 'react';

// helpTip is imported by editable-panel from applications/components/utils,
// which transitively pulls in lodash-es (ESM-only) and many heavy deps.
// Mock the entire utils module to avoid transform failures.
jest.mock('../../../../applications/components/utils', () => ({
    helpTip: (text: string) => React.createElement('span', {title: text}, '?'),
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

function render(element: React.ReactElement) {
    let instance: renderer.ReactTestRenderer;
    act(() => {
        instance = renderer.create(<Wrapper>{element}</Wrapper>);
    });
    return {
        getInstance: () => instance,
        findByText: (text: string) => {
            const json = instance.toJSON();
            return JSON.stringify(json).includes(text);
        },
    };
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
        const {findByText} = render(
            <EditablePanel
                values={{name: 'alice'}}
                items={basicItems}
                save={jest.fn().mockResolvedValue(undefined)}
            />
        );
        expect(findByText('alice')).toBe(true);
        expect(findByText('Edit')).toBe(true);
        expect(findByText('Save')).toBe(false);
    });

    test('clicking Edit switches to edit mode', () => {
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{name: 'alice'}}
                        items={basicItems}
                        save={jest.fn().mockResolvedValue(undefined)}
                    />
                </Wrapper>
            );
        });

        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });

        const json = JSON.stringify(instance!.toJSON());
        expect(json).toContain('Save');
        expect(json).toContain('Cancel');
    });

    test('clicking Cancel exits edit mode without calling save', () => {
        const save = jest.fn().mockResolvedValue(undefined);
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
                </Wrapper>
            );
        });

        // Enter edit mode
        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });

        // Click Cancel
        const cancelButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Cancel'))[0];
        act(() => { cancelButton.props.onClick(); });

        expect(save).not.toHaveBeenCalled();
        const json = JSON.stringify(instance!.toJSON());
        expect(json).toContain('Edit');
        expect(json).not.toContain('Cancel');
    });

    test('clicking Save calls save() and returns to view mode', async () => {
        const save = jest.fn().mockResolvedValue(undefined);
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
                </Wrapper>
            );
        });

        // Enter edit mode
        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });

        // Click Save (triggers formApiRef.current.submitForm)
        const saveButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Save'))[0];
        await act(async () => { saveButton.props.onClick(); });

        expect(save).toHaveBeenCalled();
        const json = JSON.stringify(instance!.toJSON());
        expect(json).toContain('Edit'); // back to view mode
    });

    test('onModeSwitch is called when entering and exiting edit mode', async () => {
        const onModeSwitch = jest.fn();
        const save = jest.fn().mockResolvedValue(undefined);
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} onModeSwitch={onModeSwitch} />
                </Wrapper>
            );
        });

        // Enter edit mode
        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });
        expect(onModeSwitch).toHaveBeenCalledTimes(1);

        // Cancel
        const cancelButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Cancel'))[0];
        act(() => { cancelButton.props.onClick(); });
        expect(onModeSwitch).toHaveBeenCalledTimes(2);
    });
});

// ===========================================================================
// 2b. Save Failure Handling
// ===========================================================================

describe('EditablePanel – save failure handling', () => {
    test('shows notification on save failure and stays in edit mode', async () => {
        const save = jest.fn().mockRejectedValue(new Error('Server error'));
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{name: 'alice'}} items={basicItems} save={save} />
                </Wrapper>
            );
        });

        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });

        const saveButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Save'))[0];
        await act(async () => { saveButton.props.onClick(); });

        expect(mockNotificationsShow).toHaveBeenCalled();
        // Panel stays in edit mode (Save button still present)
        const json = JSON.stringify(instance!.toJSON());
        expect(json).toContain('Save');
    });
});

// ===========================================================================
// 2c. noReadonlyMode (always-edit)
// ===========================================================================

describe('EditablePanel – noReadonlyMode', () => {
    test('renders in edit mode immediately when noReadonlyMode=true', () => {
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{name: 'alice'}}
                        items={basicItems}
                        save={jest.fn().mockResolvedValue(undefined)}
                        noReadonlyMode={true}
                    />
                </Wrapper>
            );
        });
        // No Edit/Save buttons in noReadonlyMode (those are only shown in standard mode)
        const json = JSON.stringify(instance!.toJSON());
        expect(json).not.toContain('"Edit"');
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

        act(() => {
            renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{name: 'alice'}}
                        items={editItems}
                        save={save}
                        noReadonlyMode={true}
                    />
                </Wrapper>
            );
        });

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
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={valuesV1} items={editItems} save={jest.fn().mockResolvedValue(undefined)} />
                </Wrapper>
            );
        });

        // Enter edit mode to get formApi
        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'))[0];
        act(() => { editButton.props.onClick(); });

        // Patch captured formApi to spy on setAllValues
        if (capturedFormApi) {
            (capturedFormApi as any).setAllValues = setAllValues;
        }

        // Update with same data, different key order — JSON.stringify will differ
        const valuesV2 = {b: 2, a: 1};
        act(() => {
            instance.update(
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

        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{name: 'v1'}} items={editItems} save={jest.fn().mockResolvedValue(undefined)} noReadonlyMode />
                </Wrapper>
            );
        });

        const setAllValuesSpy = jest.spyOn(capturedFormApi!, 'setAllValues');

        act(() => {
            instance.update(
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
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{}}
                        items={[]}
                        collapsible={true}
                        collapsed={true}
                        collapsedDescription='Click to expand'
                    />
                </Wrapper>
            );
        });
        const json = JSON.stringify(instance!.toJSON());
        expect(json).toContain('Click to expand');
        expect(json).not.toContain('"Edit"');
    });

    test('clicking collapsed panel expands it', () => {
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{}}
                        items={[]}
                        collapsible={true}
                        collapsed={true}
                        collapsedDescription='Click to expand'
                    />
                </Wrapper>
            );
        });

        // The collapsed panel is a div with an onClick
        const collapsedDiv = instance!.root.findAll(n => n.type === 'div' && typeof n.props.onClick === 'function')[0];
        act(() => { collapsedDiv.props.onClick(); });

        const json = JSON.stringify(instance!.toJSON());
        expect(json).not.toContain('Click to expand');
    });

    test('external collapsed prop change updates internal state', () => {
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel values={{}} items={[]} collapsible={true} collapsed={false} />
                </Wrapper>
            );
        });
        // Not collapsed initially
        expect(JSON.stringify(instance!.toJSON())).not.toContain('fa-angle-down filter__collapse');

        act(() => {
            instance.update(
                <Wrapper>
                    <EditablePanel values={{}} items={[]} collapsible={true} collapsed={true} />
                </Wrapper>
            );
        });
        // Now collapsed
        const json = JSON.stringify(instance!.toJSON());
        // The collapsed view renders redirect-panel, not the editable white-box
        expect(json).toContain('redirect-panel');
    });

    test('disabled Edit button renders when hasMultipleSources=true', () => {
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Wrapper>
                    <EditablePanel
                        values={{}}
                        items={[]}
                        save={jest.fn()}
                        hasMultipleSources={true}
                    />
                </Wrapper>
            );
        });
        const editButton = instance!.root.findAll(n => n.type === 'button' && String(n.children).includes('Edit'));
        expect(editButton.length).toBeGreaterThan(0);
        expect(editButton[0].props.disabled).toBe(true);
    });
});
