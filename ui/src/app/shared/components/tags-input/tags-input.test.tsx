import {fireEvent, render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import {TagsInput} from './tags-input';

describe('TagsInput', () => {
    test('gives each values field its own accessible description while hints are closed', () => {
        render(
            <>
                <TagsInput tags={[]} ariaLabel='First values file' description='First source guidance' />
                <TagsInput tags={[]} ariaLabel='Second values file' description='Second source guidance' />
            </>
        );

        const first = screen.getByRole('combobox', {name: 'First values file'});
        const second = screen.getByRole('combobox', {name: 'Second values file'});
        expect(first).toHaveAccessibleDescription('First source guidance');
        expect(second).toHaveAccessibleDescription('Second source guidance');
        expect(first.getAttribute('aria-describedby')).not.toBe(second.getAttribute('aria-describedby'));
    });

    test('commits typed input as a tag on Enter', () => {
        const onChange = jest.fn();
        render(<TagsInput tags={[]} onChange={onChange} />);

        const input = screen.getByRole('combobox');
        fireEvent.change(input, {target: {value: 'values.yaml'}});
        fireEvent.keyUp(input, {key: 'Enter', keyCode: 13});

        expect(onChange).toHaveBeenCalledWith(['values.yaml']);
        expect(screen.getByText('values.yaml')).toBeInTheDocument();
    });

    test('commits typed input as a tag on blur', () => {
        const onChange = jest.fn();
        render(<TagsInput tags={[]} placeholder='Add value' onChange={onChange} />);

        const input = screen.getByRole('combobox');
        fireEvent.change(input, {target: {value: '$values/path/to/values.yaml'}});
        fireEvent.blur(input);

        expect(onChange).toHaveBeenCalledWith(['$values/path/to/values.yaml']);
        expect(screen.getByText('$values/path/to/values.yaml')).toBeInTheDocument();
    });

    test('does not add a duplicate tag on blur', () => {
        const onChange = jest.fn();
        render(<TagsInput tags={['values.yaml']} onChange={onChange} />);

        const input = screen.getByRole('combobox');
        fireEvent.change(input, {target: {value: 'values.yaml'}});
        fireEvent.blur(input);

        expect(onChange).not.toHaveBeenCalled();
        expect(screen.getAllByText('values.yaml')).toHaveLength(1);
    });

    test.each([
        {selection: 'default highlight', key: '', expected: 'values.yaml'},
        {selection: 'keyboard highlight', key: '{ArrowDown}', expected: 'values-dev.yaml'}
    ])('commits only the $selection on Tab and clears the input', async ({key, expected}) => {
        const onChange = jest.fn();
        const user = userEvent.setup();
        render(<TagsInput tags={[]} autocomplete={['values.yaml', 'values-dev.yaml']} onChange={onChange} />);

        const input = screen.getByRole('combobox');
        await user.click(input);
        await user.type(input, 'values');
        if (key) {
            await user.keyboard(key);
        }
        await user.tab();

        expect(onChange).toHaveBeenCalledTimes(1);
        expect(onChange).toHaveBeenLastCalledWith([expected]);
        expect(input).toHaveValue('');

        // Reopening starts with every suggestion, and selecting the same file is a no-op.
        await user.click(input);
        expect(screen.getAllByRole('option')).toHaveLength(2);
        await user.type(input, expected);
        await user.tab();

        expect(onChange).toHaveBeenCalledTimes(1);
        expect(screen.getAllByText(expected)).toHaveLength(1);
        expect(input).toHaveValue('');
    });

    test.each(['$values/path/to/values.yaml', 'values.yaml'])('saves %s when clicking Save directly without Enter', async value => {
        const onSave = jest.fn();
        const user = userEvent.setup();
        const ValuesFilesForm = () => {
            const [tags, setTags] = React.useState<string[]>([]);
            return (
                <>
                    <TagsInput tags={tags} autocomplete={['values.yaml']} onChange={setTags} />
                    <button type='button' onClick={() => onSave(tags)}>
                        Save
                    </button>
                </>
            );
        };
        render(<ValuesFilesForm />);

        const input = screen.getByRole('combobox');
        await user.click(input);
        await user.type(input, value);
        await user.click(screen.getByRole('button', {name: 'Save'}));

        expect(onSave).toHaveBeenCalledWith([value]);
        expect(input).toHaveValue('');
    });
});
