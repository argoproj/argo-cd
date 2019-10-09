import * as React from 'react';

/*
    This provide a way to may a form field to an array of items. It allows you to

    * Add a new (maybe duplicate) item.
    * Replace an item.
    * Remove an item.

    E.g.
    env:
    - name: FOO
      value: bar
    - name: BAZ
      value: qux
    # You can have dup items
    - name: FOO
      value: bar

    It does not allow re-ordering of elements (maybe in a v2).
 */

class Props<I> {
    public items: I[];
    public readonly onChange: (items: I[]) => void;
    // render a component to edit an item
    public readonly itemEditor: (i: I, replaceItem: (k: I) => void, removeItem: () => void) => React.ReactFragment;
    // render a component to create a new item
    public readonly itemCreator: (addItem: (i: I) => void) => React.ReactFragment;
}

class State<I> {
    public items: I[];
}

export abstract class ArrayInput<I> extends React.Component<Props<I>, State<I>> {
    protected constructor(props: Readonly<Props<I>>) {
        super(props);
        this.state = {items: props.items};
    }

    public render() {
        const addItem = (i: I) => {
            this.setState((s) => {
                s.items.push(i);
                this.props.onChange(s.items);
                return {...s};
            });
        };
        const replaceItem = (i: I, j: number) => {
            this.setState((s) => {
                s.items[j] = i;
                this.props.onChange(s.items);
                return s;
            });
        };
        const removeItem = (j: number) => {
            this.setState((s) => {
                s.items.splice(j, 1);
                this.props.onChange(s.items);
                return s;
            });
        };
        return (
            <div className='argo-field' style={{border: 0}}>
                <div>
                    {this.state.items.map((i, j) => (
                        <div key={`item-${j}`}>
                            {this.props.itemEditor(i, (k: I) => replaceItem(k, j), () => removeItem(j))}
                        </div>
                    ))}
                    <div>
                        {this.props.itemCreator(addItem)}
                    </div>
                </div>
            </div>
        );
    }
}
