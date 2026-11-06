import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {ProjectGroupHeading} from './project-group-heading';

describe('ProjectGroupHeading', () => {
    it('shows an open folder, the project name and the count when expanded', () => {
        const {container} = render(<ProjectGroupHeading project='infra' count={4} collapsed={false} onToggle={jest.fn()} />);
        const button = screen.getByRole('button');
        expect(button.getAttribute('aria-expanded')).toBe('true');
        expect(container.querySelector('i').className).toContain('fa-folder-open');
        expect(container.querySelector('.project-group-heading__name').textContent).toBe('infra');
        expect(container.querySelector('.project-group-heading__count').textContent).toBe('4');
    });

    it('shows a closed folder when collapsed and toggles on click', () => {
        const onToggle = jest.fn();
        const {container} = render(<ProjectGroupHeading project='infra' count={4} collapsed={true} onToggle={onToggle} />);
        expect(screen.getByRole('button').getAttribute('aria-expanded')).toBe('false');
        expect(container.querySelector('i').className).not.toContain('fa-folder-open');
        fireEvent.click(screen.getByRole('button'));
        expect(onToggle).toHaveBeenCalledTimes(1);
    });
});
