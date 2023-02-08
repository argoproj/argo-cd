import {Checkbox, DropDown, Duration, NotificationType, Ticker} from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import {ErrorNotification, Revision, Timestamp} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as utils from '../utils';

import './application-operation-state.scss';

interface Props {
    application: models.Application;
}

const Filter = (props: {filters: string[]; setFilters: (f: string[]) => void; options: string[]; title: string; style?: React.CSSProperties}) => {
    const {filters, setFilters, options, title, style} = props;
    return (
        <DropDown
            isMenu={true}
            anchor={() => (
                <div title='Filter' style={style}>
                    <button className='argo-button argo-button--base'>
                        {title} <i className='argo-icon-filter' aria-hidden='true' />
                    </button>
                </div>
            )}>
            {options.map(f => (
                <div key={f} style={{minWidth: '150px', lineHeight: '2em', padding: '5px'}}>
                    <Checkbox
                        checked={filters.includes(f)}
                        onChange={checked => {
                            const selectedValues = [...filters];
                            const idx = selectedValues.indexOf(f);
                            if (idx > -1 && !checked) {
                                selectedValues.splice(idx, 1);
                            } else {
                                selectedValues.push(f);
                            }
                            setFilters(selectedValues);
                        }}
                    />
                    <label htmlFor={`filter__${f}`}>{f}</label>
                </div>
            ))}
        </DropDown>
    );
};

export const ApplicationOperationState: React.FC<Props> = ({application}) => {
    const {apis} = useContext(AppContext);
    const [operationState, setOperationState] = useState<models.OperationState | null>(null);

    useEffect(() => {
        const fetchOperationState = async () => {
            try {
                const state = await services.applications.getOperationState(application.metadata.name, application.metadata.namespace);
                setOperationState(state);
            } catch (e) {
                apis.notifications.show({
                    content: <Error
