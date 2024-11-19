import * as React from 'react';
import {useState} from 'react';
import {ViewType} from 'react-diff-view';
import 'react-diff-view/style/index.css';

import './application-resources-diff.scss';
import DiffView from '../diff-view/diff-view';
import {FileWithSource} from './application-resources-diff';

export interface IndividualDiffSectionProps {
    resourceName: string;
    file: FileWithSource;
    showPath: boolean;
    whiteBox: string;
    viewType: ViewType;
}

export const IndividualDiffSection = (props: IndividualDiffSectionProps) => {
    const {resourceName, file, showPath, whiteBox, viewType} = props;
    const [collapsed, setCollapsed] = useState(false);
    return (
        <div className={`${whiteBox} application-component-diff__diff`}>
            {showPath && (
                <p className='application-resources-diff__diff__title'>
                    {resourceName}
                    <i className={`fa fa-caret-${collapsed ? 'down' : 'up'} diff__collapse`} onClick={() => setCollapsed(!collapsed)} />
                </p>
            )}
            {!collapsed && <DiffView diffType={file.type} viewType={viewType} editsType={'block'} language={'yaml'} hunks={file.hunks} oldSource={file.oldSource} />}
        </div>
    );
};
