import {fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';
import {TagsInput} from './tags-input';

describe('TagsInput', () => {
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
});
