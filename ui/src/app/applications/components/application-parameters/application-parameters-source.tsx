import * as classNames from 'classnames';
import * as React from 'react';
import {ReactNode, useContext, useState} from 'react';
import {FormApi} from 'react-form';
import {EditablePanelItem} from '../../../shared/components';
import {EditableSection} from '../../../shared/components/editable-panel/editable-section';
import {Context} from '../../../shared/context';
import '../../../shared/components/editable-panel/editable-panel.scss';

export interface ApplicationParametersPanelProps<T> {
    floatingTitle?: string | ReactNode;
    titleTop?: string | ReactNode;
    titleBottom?: string | ReactNode;
    index: number;
    valuesTop?: T;
    valuesBottom?: T;
    validateTop?: (values: T) => any;
    validateBottom?: (values: T) => any;
    saveTop?: (input: T, query: {validate?: boolean}) => Promise<any>;
    saveBottom?: (input: T, query: {validate?: boolean}) => Promise<any>;
    itemsTop?: EditablePanelItem[];
    itemsBottom?: EditablePanelItem[];
    onModeSwitch?: () => any;
    viewTop?: string | ReactNode;
    viewBottom?: string | ReactNode;
    editTop?: (formApi: FormApi) => ReactNode;
    editBottom?: (formApi: FormApi) => ReactNode;
    numberOfSources?: number;
    noReadonlyMode?: boolean;
    collapsible?: boolean;
    deleteSource: () => void;
}

// Currently two editable sections, but can be modified to support N panels in general.  This should be part of a white-box, editable-panel.
export function ApplicationParametersSource<T = {}>(props: ApplicationParametersPanelProps<T>) {
    const [editTop, setEditTop] = useState(!!props.noReadonlyMode);
    const [editBottom, setEditBottom] = useState(!!props.noReadonlyMode);
    const [savingTop] = useState(false);
    const ctx = useContext(Context);

    const onModeSwitch = () => {
        if (props.onModeSwitch) {
            props.onModeSwitch();
        }
    };

    return (
        <div className={classNames({'editable-panel--disabled': savingTop})}>
            {props.floatingTitle && <div className='white-box--additional-top-space editable-panel__sticky-title'>{props.floatingTitle}</div>}
            <EditableSection
                uniqueId={'top_' + props.index}
                title={props.titleTop}
                view={props.viewTop}
                values={props.valuesTop}
                items={props.itemsTop}
                validate={props.validateTop}
                save={props.saveTop}
                onModeSwitch={() => onModeSwitch()}
                noReadonlyMode={props.noReadonlyMode}
                edit={props.editTop}
                collapsible={props.collapsible}
                ctx={ctx}
                isTopSection={true}
                disabledState={editTop || editTop === null}
                disabledDelete={props.numberOfSources <= 1}
                updateButtons={editClicked => {
                    setEditBottom(editClicked);
                }}
                deleteSource={props.deleteSource}
            />
            {props.itemsTop && (
                <>
                    <div className='row white-box__details-row'>
                        <p>&nbsp;</p>
                    </div>
                    <div className='white-box--no-padding editable-panel__divider' />
                </>
            )}
            <EditableSection
                uniqueId={'bottom_' + props.index}
                title={props.titleBottom}
                view={props.viewBottom}
                values={props.valuesBottom}
                items={props.itemsBottom}
                validate={props.validateBottom}
                save={props.saveBottom}
                onModeSwitch={() => onModeSwitch()}
                noReadonlyMode={props.noReadonlyMode}
                edit={props.editBottom}
                collapsible={props.collapsible}
                ctx={ctx}
                isTopSection={false}
                disabledState={editBottom || editBottom === null}
                updateButtons={editClicked => {
                    setEditTop(editClicked);
                }}
            />
        </div>
    );
}
