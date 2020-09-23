import * as React from 'react';
import {FieldData, IsFieldValue} from '../../../settings/components/project/card/field';
import {FieldLabels} from '../../../settings/components/project/card/row';
import {GetProp} from '../../../settings/components/utils';

require('../../../settings/components/project/card/card.scss');

export function MultiData<T>(fields: FieldData[], data: T[], title: string): React.ReactFragment {
    const rows =
        data && data.length > 0 ? (
            data.map((d: T, idx) => (
                <div className='card__row' key={idx}>
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
            ))
        ) : (
            <div className='card__row'>Project has no {title}</div>
        );
    return (
        <React.Fragment>
            {FieldLabels(fields, false)}
            {rows}
        </React.Fragment>
    );
}
