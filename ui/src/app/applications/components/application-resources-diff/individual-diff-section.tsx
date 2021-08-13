import * as React from 'react';
import {useState} from 'react';
import {Diff, Hunk} from 'react-diff-view';
import 'react-diff-view/style/index.css';

require('./application-resources-diff.scss');

export interface IndividualDiffSectionProps {
    file: any;
    showPath: boolean;
    whiteBox: string;
    viewType: string;
}

export const IndividualDiffSection = (props: IndividualDiffSectionProps) => {
    const {file, showPath, whiteBox, viewType} = props;
    const [collapsed, setCollapsed] = useState(false);
    return (
        <div className={`${whiteBox} application-component-diff__diff`}>
            {showPath && (
                <p className='application-resources-diff__diff__title'>
                    {file.newPath}
                    <i className={`fa fa-caret-${collapsed ? 'down' : 'up'} diff__collapse`} onClick={() => setCollapsed(!collapsed)} />
                </p>
            )}
            {!collapsed && (
                <Diff viewType={viewType} diffType={file.type} hunks={file.hunks}>
                    {(hunks: any) => hunks.map((hunk: any) => <Hunk key={hunk.content} hunk={hunk} />)}
                </Diff>
            )}
        </div>
    );
};
