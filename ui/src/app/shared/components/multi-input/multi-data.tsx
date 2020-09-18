import * as React from 'react';
import {GetProp} from '../../../settings/components/utils';
import {FieldData, FieldLabels, IsFieldValue} from '../../../settings/components/project/card/field';

require('../../../settings/components/project/card/card.scss')

export function MultiData<T>(fields: FieldData[], data: T[]): React.ReactFragment {
    const rows = data.map((d: T, idx) => (
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
                    <div key={field.name} className={`card__col-input card__col card__col-${field.size}`}>
                        {curVal}
                    </div>
                );
            })}
        </div>
    ));
    return (
        <div className='card__multi-data'>
            {FieldLabels(fields)}
            {rows}
        </div>
    );
}
