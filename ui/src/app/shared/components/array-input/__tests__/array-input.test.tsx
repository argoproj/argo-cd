import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {act} from 'react';
import {
    ArrayInput,
    NameValueEditor,
    ValueEditor,
    ArrayInputField,
    MapInputField,
    ArrayValueField,
    StringValueField,
    MapValueField,
    ResetOrDeleteButton,
    NameValue,
} from '../array-input';
import {Form, FormApi} from 'argo-ui';

// ---------------------------------------------------------------------------
// Helper: render an ArrayInput field inside a Form and return the FormApi
// ---------------------------------------------------------------------------

function renderWithForm<T>(
    FieldComponent: React.ComponentType<any>,
    fieldProps: Record<string, any>,
    defaultValues: Record<string, any>
): {api: FormApi; getInstance: () => renderer.ReactTestRenderer} {
    let capturedApi: FormApi | null = null;
    let instance: renderer.ReactTestRenderer;
    act(() => {
        instance = renderer.create(
            <Form defaultValues={defaultValues} getApi={api => { capturedApi = api; }}>
                {() => <FieldComponent field='items' {...fieldProps} />}
            </Form>
        );
    });
    return {api: capturedApi as unknown as FormApi, getInstance: () => instance};
}

// ===========================================================================
// 3a. Basic Array Operations — ArrayInput<T>
// ===========================================================================

describe('ArrayInput – basic operations', () => {
    test('renders all items', () => {
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={jest.fn()} editor={NameValueEditor} />
            );
        });
        const json = JSON.stringify(tree!.toJSON());
        expect(json).toContain('"A"');
        expect(json).toContain('"B"');
    });

    test('renders "No items" label when empty', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={[]} onChange={jest.fn()} editor={NameValueEditor} />
            );
        });
        const json = JSON.stringify(tree!.toJSON());
        expect(json).toContain('No items');
    });

    test('clicking the + button calls onChange with a new empty item appended', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'X', value: '1'}];
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />
            );
        });

        const addButton = tree!.root.findAll(n => n.type === 'button' && n.children.some((c: any) => c?.props?.className?.includes('fa-plus')))[0];
        act(() => { addButton.props.onClick(); });

        expect(onChange).toHaveBeenCalledTimes(1);
        const newItems = onChange.mock.calls[0][0];
        expect(newItems).toHaveLength(2);
        expect(newItems[0]).toEqual({name: 'X', value: '1'}); // original preserved
        expect(newItems[1]).toEqual({});                        // new empty item
    });

    test('clicking × on an item removes it and preserves the rest', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}, {name: 'C', value: '3'}];
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />
            );
        });

        // Find all × buttons (fa-times icons)
        const removeIcons = tree!.root.findAll(n => n.type === 'i' && n.props.className?.includes('fa-times'));
        // Remove the second item (index 1)
        act(() => { removeIcons[1].props.onClick(); });

        expect(onChange).toHaveBeenCalledTimes(1);
        const newItems = onChange.mock.calls[0][0];
        expect(newItems).toHaveLength(2);
        expect(newItems[0]).toEqual({name: 'A', value: '1'});
        expect(newItems[1]).toEqual({name: 'C', value: '3'});
    });

    test('editing an item calls onChange with the updated item in place', () => {
        const onChange = jest.fn();
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />
            );
        });

        // The first name input
        const inputs = tree!.root.findAll(n => n.type === 'input' && n.props.title === 'Name');
        act(() => { inputs[0].props.onChange({target: {value: 'Z'}}); });

        const newItems = onChange.mock.calls[0][0];
        expect(newItems[0].name).toBe('Z');
        expect(newItems[1]).toEqual({name: 'B', value: '2'}); // untouched
    });
});

// ===========================================================================
// 3b. Key Stability
// ===========================================================================

describe('ArrayInput – key stability', () => {
    test('uses index-based keys (documents known limitation)', () => {
        // React uses key={`item-${i}`} — this test documents that two items render
        // two item rows. The key strategy (index-based) is confirmed by the source.
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={jest.fn()} editor={NameValueEditor} />
            );
        });
        // Each item renders as a div with an × button and editor inputs.
        // We verify via the remove icons (one per item).
        const removeIcons = tree!.root.findAll(n => n.type === 'i' && n.props.className?.includes('fa-times'));
        expect(removeIcons).toHaveLength(2);
    });

    test('removing the first item does not lose data from the second item', () => {
        const items: NameValue[] = [{name: 'A', value: '1'}, {name: 'B', value: '2'}];
        const captured: NameValue[][] = [];
        const onChange = (newItems: NameValue[]) => captured.push(newItems);
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <ArrayInput items={items} onChange={onChange} editor={NameValueEditor} />
            );
        });

        const removeIcons = tree!.root.findAll(n => n.type === 'i' && n.props.className?.includes('fa-times'));
        act(() => { removeIcons[0].props.onClick(); });

        const remaining = captured[0];
        expect(remaining).toHaveLength(1);
        expect(remaining[0]).toEqual({name: 'B', value: '2'});
    });
});

// ===========================================================================
// 3c. ArrayInputField (form-connected)
// ===========================================================================

describe('ArrayInputField – form connected', () => {
    test('renders items from form values', () => {
        const {getInstance} = renderWithForm(ArrayInputField, {}, {
            items: [{name: 'FOO', value: 'bar'}],
        });
        const json = JSON.stringify(getInstance().toJSON());
        expect(json).toContain('"FOO"');
        expect(json).toContain('"bar"');
    });

    test('adding an item updates form value', () => {
        const {api, getInstance} = renderWithForm(ArrayInputField, {}, {
            items: [{name: 'A', value: '1'}],
        });

        const addButton = getInstance().root.findAll(n => n.type === 'button' && n.children.some((c: any) => c?.props?.className?.includes('fa-plus')))[0];
        act(() => { addButton.props.onClick(); });

        expect(api.values.items).toHaveLength(2);
    });

    test('removing an item updates form value', () => {
        const {api, getInstance} = renderWithForm(ArrayInputField, {}, {
            items: [{name: 'A', value: '1'}, {name: 'B', value: '2'}],
        });

        const removeIcons = getInstance().root.findAll(n => n.type === 'i' && n.props.className?.includes('fa-times'));
        act(() => { removeIcons[0].props.onClick(); });

        expect(api.values.items).toHaveLength(1);
        expect(api.values.items[0].name).toBe('B');
    });
});

// ===========================================================================
// 3d. MapInputField – object ↔ name-value-pair conversion
// ===========================================================================

describe('MapInputField – object conversion', () => {
    test('renders object as name-value pairs', () => {
        const {getInstance} = renderWithForm(MapInputField, {}, {
            items: {foo: 'bar', baz: 'qux'},
        });
        const json = JSON.stringify(getInstance().toJSON());
        expect(json).toContain('"foo"');
        expect(json).toContain('"bar"');
    });

    test('editing a pair updates the object key/value', () => {
        const {api, getInstance} = renderWithForm(MapInputField, {}, {
            items: {foo: 'bar'},
        });

        // Edit the value input
        const valueInputs = getInstance().root.findAll(n => n.type === 'input' && n.props.title === 'Value');
        act(() => { valueInputs[0].props.onChange({target: {value: 'newval'}}); });

        expect(api.values.items.foo).toBe('newval');
    });

    test('adding a pair appends to the object', () => {
        const {api, getInstance} = renderWithForm(MapInputField, {}, {
            items: {existing: 'value'},
        });

        const addButton = getInstance().root.findAll(n => n.type === 'button' && n.children.some((c: any) => c?.props?.className?.includes('fa-plus')))[0];
        act(() => { addButton.props.onClick(); });

        // New empty key '' should be present
        expect(Object.keys(api.values.items)).toHaveLength(2);
    });

    test('removing a pair removes the key from the object', () => {
        const {api, getInstance} = renderWithForm(MapInputField, {}, {
            items: {a: '1', b: '2'},
        });

        const removeIcons = getInstance().root.findAll(n => n.type === 'i' && n.props.className?.includes('fa-times'));
        act(() => { removeIcons[0].props.onClick(); });

        expect(Object.keys(api.values.items)).toHaveLength(1);
    });
});

// ===========================================================================
// 3e. ResetOrDeleteButton
// ===========================================================================

describe('ResetOrDeleteButton', () => {
    function makeProps(overrides: Partial<React.ComponentProps<typeof ResetOrDeleteButton>> = {}) {
        return {
            isPluginPar: false,
            getValue: jest.fn().mockReturnValue([{name: 'myParam', string: 'val'}]),
            name: 'myParam',
            index: 0,
            setValue: jest.fn(),
            setAppParamsDeletedState: jest.fn(),
            ...overrides,
        };
    }

    test('renders Delete button for non-plugin parameter', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...makeProps()} />);
        });
        expect(JSON.stringify(tree!.toJSON())).toContain('Delete');
    });

    test('renders Reset button for plugin parameter', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...makeProps({isPluginPar: true})} />);
        });
        expect(JSON.stringify(tree!.toJSON())).toContain('Reset');
    });

    test('Delete button calls setAppParamsDeletedState with parameter name', () => {
        const props = makeProps();
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...props} />);
        });
        const btn = tree!.root.findByType('button');
        act(() => { btn.props.onClick(); });
        expect(props.setAppParamsDeletedState).toHaveBeenCalledTimes(1);
    });

    test('Reset button removes item from array via setValue', () => {
        const items = [{name: 'p1', string: 'v1'}, {name: 'p2', string: 'v2'}];
        const setValue = jest.fn();
        const props = makeProps({
            isPluginPar: true,
            getValue: jest.fn().mockReturnValue(items),
            name: 'p1',
            index: 0,
            setValue,
        });
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...props} />);
        });
        const btn = tree!.root.findByType('button');
        act(() => { btn.props.onClick(); });
        expect(setValue).toHaveBeenCalledTimes(1);
        const newVal = setValue.mock.calls[0][0];
        expect(newVal).toHaveLength(1);
        expect(newVal[0].name).toBe('p2');
    });

    test('button is disabled when index is -1', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...makeProps({index: -1})} />);
        });
        const btn = tree!.root.findByType('button');
        expect(btn.props.disabled).toBe(true);
    });

    test('does not call handlers when index is -1', () => {
        const props = makeProps({index: -1});
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(<ResetOrDeleteButton {...props} />);
        });
        const btn = tree!.root.findByType('button');
        act(() => { btn.props.onClick(); });
        // Neither handler should be called for non-plugin par when index === -1
        // (button is disabled so click is a no-op in a real browser, but we test the handlers)
        expect(props.setAppParamsDeletedState).not.toHaveBeenCalled();
    });
});

// ===========================================================================
// 3f. ReadOnly Mode — NameValueEditor / ValueEditor
// ===========================================================================

describe('NameValueEditor – readonly mode', () => {
    test('inputs are readOnly when onChange is not provided', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <>{NameValueEditor({name: 'foo', value: 'bar'})}</>
            );
        });
        const inputs = tree!.root.findAll(n => n.type === 'input');
        inputs.forEach(input => {
            expect(input.props.readOnly).toBe(true);
        });
    });

    test('inputs are editable when onChange is provided', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <>{NameValueEditor({name: 'foo', value: 'bar'}, jest.fn())}</>
            );
        });
        const inputs = tree!.root.findAll(n => n.type === 'input');
        inputs.forEach(input => {
            expect(input.props.readOnly).toBe(false);
        });
    });
});

describe('ValueEditor – readonly mode', () => {
    test('input is readOnly when onChange is falsy', () => {
        let tree: renderer.ReactTestRenderer;
        act(() => {
            tree = renderer.create(
                <>{ValueEditor('hello', null as any)}</>
            );
        });
        const input = tree!.root.findByType('input');
        expect(input.props.readOnly).toBe(true);
    });
});

// ===========================================================================
// 3g. ArrayValueField — direct mutation risk
// ===========================================================================

describe('ArrayValueField – rapid updates', () => {
    function renderArrayValueField(defaultItems: any[]) {
        let capturedApi: FormApi | null = null;
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Form defaultValues={{items: defaultItems}} getApi={api => { capturedApi = api; }}>
                    {() => (
                        <ArrayValueField
                            field='items'
                            name='myParam'
                            defaultVal={['default']}
                            isPluginPar={true}
                            setAppParamsDeletedState={jest.fn()}
                        />
                    )}
                </Form>
            );
        });
        return {api: capturedApi as unknown as FormApi, getInstance: () => instance};
    }

    test('renders array values for a named parameter', () => {
        const {getInstance} = renderArrayValueField([{name: 'myParam', array: ['val1', 'val2']}]);
        const json = JSON.stringify(getInstance().toJSON());
        expect(json).toContain('"val1"');
        expect(json).toContain('"val2"');
    });

    test('shows default values when parameter not yet overridden', () => {
        const {getInstance} = renderArrayValueField([]);
        const json = JSON.stringify(getInstance().toJSON());
        expect(json).toContain('"default"');
    });

    test('adding a value appends to the parameter array', () => {
        const {api, getInstance} = renderArrayValueField([{name: 'myParam', array: ['a']}]);
        const addButton = getInstance().root.findAll(n => n.type === 'button' && n.children.some((c: any) => c?.props?.className?.includes('fa-plus')))[0];
        act(() => { addButton.props.onClick(); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.array).toHaveLength(2);
    });
});

// ===========================================================================
// 3h. StringValueField
// ===========================================================================

describe('StringValueField', () => {
    function renderStringValueField(defaultItems: any[]) {
        let capturedApi: FormApi | null = null;
        let instance: renderer.ReactTestRenderer;
        act(() => {
            instance = renderer.create(
                <Form defaultValues={{items: defaultItems}} getApi={api => { capturedApi = api; }}>
                    {() => (
                        <StringValueField
                            field='items'
                            name='myParam'
                            defaultVal='default_string'
                            isPluginPar={false}
                            setAppParamsDeletedState={jest.fn()}
                        />
                    )}
                </Form>
            );
        });
        return {api: capturedApi as unknown as FormApi, getInstance: () => instance};
    }

    test('renders default value when parameter not overridden', () => {
        const {getInstance} = renderStringValueField([]);
        const json = JSON.stringify(getInstance().toJSON());
        expect(json).toContain('"default_string"');
    });

    test('typing a value creates/updates the parameter', () => {
        const {api, getInstance} = renderStringValueField([]);
        const input = getInstance().root.findAll(n => n.type === 'input' && n.props.placeholder === 'Value')[0];
        act(() => { input.props.onChange({target: {value: 'typed_value'}}); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.string).toBe('typed_value');
    });

    test('updating an existing parameter string value', () => {
        const {api, getInstance} = renderStringValueField([{name: 'myParam', string: 'initial'}]);
        const input = getInstance().root.findAll(n => n.type === 'input' && n.props.placeholder === 'Value')[0];
        act(() => { input.props.onChange({target: {value: 'updated'}}); });
        const param = api.values.items.find((p: any) => p.name === 'myParam');
        expect(param.string).toBe('updated');
    });
});
