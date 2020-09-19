import * as React from 'react';
import {FieldData, FieldLabels, IsFieldValue} from '../../../settings/components/project/card/field';
import {GetProp} from '../../../settings/components/utils';

require('../../../settings/components/project/card/card.scss');

export function MultiData<T>(fields: FieldData[], data: T[]): React.ReactFragment {
    const rows =
        data && data.length > 0 ? (
            data.map((d: T, idx) => (
                <div className='card__input-container card__row card__row--data' key={idx}>
                    {fields.map((field, i) => {
                        let curVal = '';
                        if (d) {
                            if (IsFieldValue(d)) {
                                curVal = d.toString();
                            } else {
                                const tmp = GetProp(d as T, field.name as keyof T);
                                curVal = tmp ? tmp.toString() : '';
                            }
                        }
                        return (
                            <div key={field.name}>
                                <div className={`card__col-input card__col card__col-select-button`} />
                                <div className={`card__col-input card__col card__col-${field.size}`}>{curVal}</div>
                            </div>
                        );
                    })}
                </div>
            ))
        ) : (
            <div>Section is empty</div>
        );
    return (
        <div className='card__multi-data'>
            {FieldLabels(fields)}
            {rows}
        </div>
    );
}
