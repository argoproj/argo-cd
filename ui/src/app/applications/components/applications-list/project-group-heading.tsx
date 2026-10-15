import classNames from 'classnames';
import * as React from 'react';

import './project-group-heading.scss';

export interface ProjectGroupHeadingProps {
    project: string;
    count: number;
    collapsed: boolean;
    onToggle: () => void;
}

export const ProjectGroupHeading = ({project, count, collapsed, onToggle}: ProjectGroupHeadingProps) => (
    <button type='button' className='project-group-heading' aria-expanded={!collapsed} title={`Project: ${project}`} onClick={onToggle}>
        <i className={classNames('fa', collapsed ? 'fa-folder' : 'fa-folder-open')} />
        <span className='project-group-heading__name'>{project}</span>
        <span className='project-group-heading__count'>{count}</span>
    </button>
);
