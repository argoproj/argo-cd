import * as React from 'react';

import './filter.scss';

interface FilterMap {
    [label: string]: boolean;
}

export const Filter = (props: {items: FilterMap; setItems: (items: FilterMap) => void}) => {
    return (
        <div className='filter'>
            {Object.keys(props.items).map(label => (
                <div className='filter__item'>{label}</div>
            ))}
        </div>
    );
};
